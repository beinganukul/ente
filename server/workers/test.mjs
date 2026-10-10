import assert from 'node:assert/strict';
import {randomBytes,createHash} from 'node:crypto';
import {createServer as createHTTPServer} from 'node:http';
import {createServer} from 'node:tls';
import {execFileSync} from 'node:child_process';
import {once} from 'node:events';
import {readFileSync,writeFileSync,mkdirSync} from 'node:fs';
import {unstable_startWorker} from 'wrangler';

const port = Number(process.env.ENTE_WORKERS_TEST_PG_PORT);
const tursoURL = process.env.ENTE_WORKERS_TEST_TURSO_URL;
if (tursoURL) {
  const url = new URL(tursoURL);
  assert.ok(['localhost','127.0.0.1','[::1]'].includes(url.hostname), 'Use a disposable local Turso database for Worker tests');
} else {
  assert.ok(Number.isInteger(port) && port > 0, 'Set ENTE_WORKERS_TEST_PG_PORT or ENTE_WORKERS_TEST_TURSO_URL to a migrated disposable database');
}
mkdirSync('.wrangler',{recursive:true});
execFileSync('openssl',['req','-x509','-newkey','rsa:2048','-nodes','-keyout','.wrangler/smtp-test.key','-out','.wrangler/smtp-test.crt','-days','1','-subj','/CN=Museum Worker SMTP test','-addext','subjectAltName=IP:127.0.0.1'],{stdio:'ignore'});
const certificate = readFileSync('.wrangler/smtp-test.crt','utf8');
const messages = [];
const smtp = createServer({key:readFileSync('.wrangler/smtp-test.key'),cert:certificate},socket => {
  socket.setEncoding('utf8'); socket.write('220 Museum test SMTP\r\n');
  let buffered = '', data = false, message = '';
  socket.on('data',chunk => {
    buffered += chunk;
    while (buffered.includes('\r\n')) {
      const at = buffered.indexOf('\r\n'), line = buffered.slice(0,at);
      buffered = buffered.slice(at+2);
      if (data) {
        // The message is queued, then the peer closes before QUIT. Signup must
        // succeed even though the connection's cleanup can now fail.
        if (line === '.') { messages.push(message); message=''; data=false; socket.end('250 accepted\r\n'); }
        else message += line+'\r\n';
      } else if (line.startsWith('EHLO')) socket.write('250 test SMTP\r\n');
      else if (line === 'DATA') { data = true; socket.write('354 send message\r\n'); }
      else if (line === 'QUIT') { socket.end('221 bye\r\n'); }
      else socket.write('250 OK\r\n');
    }
  });
});
smtp.listen(0,'127.0.0.1'); await once(smtp,'listening');
// Exercise Worker -> S3 HTTP requests separately from clients' presigned PUTs.
const objects = new Map(), storageRequests = [];
const storage = createHTTPServer(async (req,res) => {
  const url = new URL(req.url,'http://localhost');
  storageRequests.push({method:req.method,path:url.pathname,authorization:req.headers.authorization});
  if (req.method==='PUT') {
    assert.ok(url.searchParams.get('X-Amz-Signature'),'Client upload must use a signed URL');
    const chunks=[]; for await (const chunk of req) chunks.push(chunk);
    const body=Buffer.concat(chunks);
    assert.equal(req.headers['content-md5'],createHash('md5').update(body).digest('base64'));
    objects.set(url.pathname,body); res.writeHead(200,{ETag:'"test-etag"'}).end();
  } else if (objects.has(url.pathname)) {
    const body=objects.get(url.pathname);
    res.writeHead(200,{'Content-Length':body.length,ETag:'"test-etag"'});
    res.end(req.method==='HEAD' ? undefined : body);
  } else res.writeHead(404).end();
});
storage.listen(0,'127.0.0.1'); await once(storage,'listening');
const config = JSON.parse(readFileSync('wrangler.jsonc','utf8'));
delete config.build;
config.name = 'ente-museum-test';
config.main = 'harness.mjs';
config.durable_objects.bindings[0].class_name = 'TestMuseum';
config.migrations[0].new_sqlite_classes = ['TestMuseum'];
config.vars = {
  ...(tursoURL ? {ENTE_DB_PROVIDER:'turso', ENTE_DB_TURSO_URL:tursoURL} : {
    ENTE_DB_PROVIDER:'postgres', ENTE_DB_HOST:'127.0.0.1', ENTE_DB_PORT:String(port), ENTE_DB_USER:'postgres', ENTE_DB_NAME:'ente_test_workers',
  }),
  ENTE_KEY_ENCRYPTION:randomBytes(32).toString('base64'),
  ENTE_KEY_HASH:randomBytes(64).toString('base64'),
  ENTE_JWT_SECRET:randomBytes(32).toString('base64url')+'=',
  ENTE_JOBS_CRON_SKIP:'true', ENTE_INTERNAL_SILENT:'true',
  ENTE_SMTP_HOST:'127.0.0.1', ENTE_SMTP_PORT:String(smtp.address().port),
  ENTE_SMTP_ENCRYPTION:'tls', CA_CERTIFICATES:certificate,
  MUSEUM_CONFIG:JSON.stringify({s3:{use_path_style_urls:true,are_local_buckets:true,'b2-eu-cen':{endpoint:`http://127.0.0.1:${storage.address().port}`,region:'auto',bucket:'museum-test',key:'test-access-key',secret:'test-secret-key'}},replication:{enabled:false}}),
};
mkdirSync('.wrangler',{recursive:true});
writeFileSync('.wrangler/test-config.json',JSON.stringify(config));
writeFileSync('.wrangler/harness.mjs',`
import worker, {Museum} from '../worker.mjs';
export class TestMuseum extends Museum {
 constructor(state,env) {
  const storage = new Proxy(state.storage, {get(target,key) {
   if (key==='get') return async (...args) => {
    if (args[0]==='museum_cron' && !await target.get('startup_failure_injected')) {
     await target.put('startup_failure_injected',true);
     await target.sync();
     throw new Error('Test: transient alarm storage read failure');
    }
    return target.get(...args);
   };
   const value=Reflect.get(target,key,target);
   return typeof value==='function' ? value.bind(target) : value;
  }});
  const startupState=new Proxy(state,{get(target,key) {
   if (key==='storage') return storage;
   const value=Reflect.get(target,key,target);
   return typeof value==='function' ? value.bind(target) : value;
  }});
  super(startupState,env);
 }
 async fetch(request) {
  const path = new URL(request.url).pathname;
  if (path==='/__test__/storage') return Response.json({startupFailure:await this.state.storage.get('startup_failure_injected'), rates:[...this.state.storage.sql.exec('SELECT * FROM museum_rate_limits')], jobs:await this.state.storage.get('museum_cron'), alarm:await this.state.storage.getAlarm()});
  if (path==='/__test__/alarm') {
   const jobs=await this.state.storage.get('museum_cron');
   for (const key in jobs) jobs[key]=new Date(0).toISOString();
   await this.state.storage.put('museum_cron',jobs);
   await super.alarm({isRetry:false,retryCount:0});
   const first=await this.state.storage.get('museum_cron');
   for (let i=1;i<Object.keys(jobs).length;i++) await super.alarm({isRetry:false,retryCount:0});
   return Response.json({first,jobs:await this.state.storage.get('museum_cron')});
  }
  return super.fetch(request);
 }
}
export default worker;
`);
let worker = await unstable_startWorker({config:'.wrangler/test-config.json', dev:{persist:`.wrangler/test-state-${process.pid}`}});
try {
  const request = (path,init) => worker.fetch(`http://localhost${path}`,init);
  // A failed constructor must reset Go memory without clearing durable data.
  try { await request('/ping'); } catch {}
  const ping = await request('/ping');
  assert.equal(ping.status,200,await ping.text());
  assert.equal((await (await request('/__test__/storage')).json()).startupFailure,true,'Startup failure was not injected or durable state was lost');
  const second = await request('/ping');
  assert.equal(second.status,200);
  assert.equal((await second.json()).message,'pong');
  const compressed = await request('/ping',{headers:{'Accept-Encoding':'gzip'}});
  assert.equal(compressed.status,200);
  assert.equal((await compressed.json()).message,'pong','Compressed API response must decode exactly once');
  assert.equal((await request('/users/details/v2')).status,401);
  assert.equal((await request('/unknown-route')).status,404);
  const options = await request('/users/details/v2',{method:'OPTIONS',headers:{Origin:'https://example.com'}});
  assert.equal(options.status,200);
  assert.equal(options.headers.get('Access-Control-Allow-Origin'),'https://example.com');
  const email = `worker-${randomBytes(6).toString('hex')}@example.com`;
  const jsonRequest = body => ({method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
  const sent = await request('/users/ott',jsonRequest({email,purpose:'signup'}));
  assert.equal(sent.status,200,await sent.text());
  assert.ok(messages.length>0,'No SMTP message received');
  const otp = messages[0].match(/\b\d{6}\b/)?.[0];
  assert.ok(otp,'Email template did not contain the verification code');
  const verified = await request('/users/verify-email',jsonRequest({email,ott:otp}));
  assert.equal(verified.status,200,await verified.clone().text());
  const account = await verified.json();
  assert.ok(account.id>0 && account.token,'Signup did not return a session');
  const authenticated = await request('/users/sessions',{headers:{'X-Auth-Token':account.token}});
  assert.equal(authenticated.status,200,await authenticated.clone().text());
  const sessions = await authenticated.json();
  assert.equal(sessions.sessions.length,1,'Signup should create one session');
  const authHeaders = {'X-Auth-Token':account.token};
  const details = await request('/users/details/v2',{headers:authHeaders});
  assert.equal(details.status,200,await details.text());
  const encoded = length => randomBytes(length).toString('base64');
  const keySetup = await request('/users/attributes',{
    ...jsonRequest({keyAttributes:{kekSalt:encoded(16),encryptedKey:encoded(48),keyDecryptionNonce:encoded(24),publicKey:encoded(32),encryptedSecretKey:encoded(48),secretKeyDecryptionNonce:encoded(24),memLimit:1073741824,opsLimit:4}}),
    method:'PUT',headers:{...authHeaders,'Content-Type':'application/json'},
  });
  assert.equal(keySetup.status,200,await keySetup.text());
  const created = await request('/collections',{
    ...jsonRequest({encryptedKey:encoded(48),keyDecryptionNonce:encoded(24),type:'album',attributes:{version:1},encryptedName:encoded(32),nameDecryptionNonce:encoded(24)}),
    headers:{...authHeaders,'Content-Type':'application/json'},
  });
  assert.equal(created.status,200,await created.clone().text());
  const collection = (await created.json()).collection;
  assert.ok(collection.id>0,'Collection ID missing');
  const listed = await request('/collections/v2',{headers:authHeaders});
  assert.equal(listed.status,200,await listed.clone().text());
  assert.ok((await listed.json()).collections.some(value=>value.id===collection.id),'Created album not listed');
  const upload = async body => {
    const md5=createHash('md5').update(body).digest('base64');
    const signed=await request('/files/upload-url',{...jsonRequest({contentLength:body.length,contentMD5:md5}),headers:{...authHeaders,'Content-Type':'application/json'}});
    assert.equal(signed.status,200,await signed.clone().text());
    const value=await signed.json(), url=new URL(value.url);
    assert.equal(url.pathname,`/museum-test/${value.objectKey}`);
    assert.ok(value.objectKey.startsWith(`${account.id}/`));
    assert.ok(url.searchParams.get('X-Amz-Signature'));
    const result=await fetch(url,{method:'PUT',headers:{'Content-MD5':md5},body});
    assert.equal(result.status,200,await result.text());
    return value.objectKey;
  };
  const fileBytes=randomBytes(4096), thumbnailBytes=randomBytes(256);
  const fileKey=await upload(fileBytes), thumbnailKey=await upload(thumbnailBytes);
  const registered=await request('/files',{
    ...jsonRequest({collectionID:collection.id,encryptedKey:encoded(48),keyDecryptionNonce:encoded(24),file:{objectKey:fileKey,decryptionHeader:encoded(24)},thumbnail:{objectKey:thumbnailKey,decryptionHeader:encoded(24)},metadata:{encryptedData:encoded(32),decryptionHeader:encoded(24)}}),
    headers:{...authHeaders,'Content-Type':'application/json'},
  });
  assert.equal(registered.status,200,await registered.clone().text());
  const photo=await registered.json();
  assert.ok(photo.id>0,'Registered file ID missing');
  assert.equal(photo.file.size,fileBytes.length);
  assert.equal(photo.thumbnail.size,thumbnailBytes.length);
  assert.equal(storageRequests.filter(value=>value.method==='HEAD' && value.authorization?.startsWith('AWS4-HMAC-SHA256')).length,2,'Museum must authenticate both outbound S3 HEAD requests');
  const download=await request(`/files/download/v2/${photo.id}`,{headers:authHeaders});
  assert.equal(download.status,200,await download.clone().text());
  const downloaded=await fetch((await download.json()).url);
  assert.equal(downloaded.status,200);
  assert.deepEqual(Buffer.from(await downloaded.arrayBuffer()),fileBytes);
  const deletedSession = await request('/users/logout',{method:'POST',headers:authHeaders});
  assert.equal(deletedSession.status,200,await deletedSession.text());
  assert.equal((await request('/users/sessions',{headers:authHeaders})).status,401,'Revoked cached session still authenticates');
  const before = await (await request('/__test__/storage')).json();
  assert.ok(before.alarm && Object.keys(before.jobs).length>=2,'Background jobs not scheduled');
  const after = await (await request('/__test__/alarm')).json();
  assert.equal(Object.values(after.first).filter(next => Date.parse(next)>Date.now()).length,1,'An alarm must run exactly one due job');
  assert.ok(Object.values(after.jobs).every(next => Date.parse(next)>Date.now()),'Alarms did not advance completed jobs');
  // Invalid requests still exercise the route limiter without sending email.
  for (let i=0;i<12;i++) await request('/users/ott',jsonRequest({}));
  assert.equal((await request('/users/ott',jsonRequest({}))).status,429);
  await worker.dispose();
  worker = await unstable_startWorker({config:'.wrangler/test-config.json',dev:{persist:`.wrangler/test-state-${process.pid}`}});
  const spoofed = jsonRequest({});
  spoofed.headers['X-Museum-Client-IP']='203.0.113.77';
  spoofed.headers['X-Forwarded-For']='203.0.113.77';
  assert.equal((await request('/users/ott',spoofed)).status,429,'Rate limit did not survive restart or trusted-IP spoofing');
  console.log(`Museum Worker (${tursoURL ? 'Turso' : 'PostgreSQL'}): database health, routing, CORS, SMTP email, signup, account details, collections, presigned S3 uploads, outbound HEAD checks, file registration/download, session revocation, alarms, and rate limits across restarts passed.`);
} finally { await worker.dispose(); smtp.close(); storage.close(); }
