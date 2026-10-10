import * as fs from 'node:fs';
import * as path from 'node:path';
import resources from './build/resources.mjs';
import {uncompressedRequest} from './request.mjs';

// workers-go exposes a Proxy as Go's global object. Cloudflare's native fetch
// requires the actual global receiver, including for net/http's fetch calls.
globalThis.fetch = globalThis.fetch.bind(globalThis);

// Go's wasm runtime consumes this Node-compatible filesystem interface.
const absolute = value => typeof value === 'string' && !value.startsWith('/') ? path.join('/tmp',value) : value;
globalThis.fs = {...fs};
for (const name of ['open','stat','lstat','mkdir','readdir','unlink','rmdir','chmod','chown','readlink','utimes','truncate']) {
  globalThis.fs[name] = (file,...args) => fs[name](absolute(file),...args);
}
for (const name of ['rename','link','symlink']) {
  globalThis.fs[name] = (source,target,...args) => fs[name](absolute(source),absolute(target),...args);
}
globalThis.path = {...path, resolve:(...parts) => path.resolve('/tmp',...parts)};
const offsets = new Map();
globalThis.fs.open = (file,flags,mode,callback) => fs.open(absolute(file),flags,mode,(error,fd) => {
  if (!error) offsets.set(fd,0);
  callback(error,fd);
});
globalThis.fs.read = (fd,buffer,offset,length,position,callback) => {
  const at = position ?? offsets.get(fd) ?? 0;
  fs.read(fd,buffer,offset,length,at,(error,count) => {
    if (!error && position == null) offsets.set(fd,at+count);
    callback(error,count,buffer);
  });
};
globalThis.fs.writeSync = (fd,buffer) => {
  if (fd === 1 || fd === 2) { console.log(new TextDecoder().decode(buffer)); return buffer.length; }
  const count = fs.writeSync(fd,buffer,0,buffer.length,offsets.get(fd) ?? 0);
  offsets.set(fd,(offsets.get(fd) ?? 0)+count);
  return count;
};
globalThis.fs.write = (fd,buffer,offset,length,position,callback) => {
  if (fd === 1 || fd === 2) { callback(null,globalThis.fs.writeSync(fd,buffer.subarray(offset,offset+length)),buffer); return; }
  const at = position ?? offsets.get(fd) ?? 0;
  fs.write(fd,buffer,offset,length,at,(error,count) => {
    if (!error && position == null) offsets.set(fd,at+count);
    callback(error,count,buffer);
  });
};
globalThis.fs.close = (fd,callback) => fs.close(fd,error => { offsets.delete(fd); callback(error); });
globalThis.process = {
  cwd: () => '/tmp',
  getuid: () => -1, getgid: () => -1, geteuid: () => -1, getegid: () => -1,
  pid: -1, ppid: -1,
};
function prepareFiles(env) {
  for (const [name, content] of Object.entries(resources)) {
    const target = path.join('/tmp',name);
    fs.mkdirSync(path.dirname(target),{recursive:true});
    fs.writeFileSync(target,content);
  }
  if (env.CA_CERTIFICATES) fs.appendFileSync('/tmp/ca-certificates.pem','\n'+env.CA_CERTIFICATES);
  for (const [key, name] of [['BILLING_US','us.json'], ['BILLING_IN','in.json'], ['BILLING_OFFERS','black-friday.json']]) {
    if (env[key]) {
      fs.mkdirSync('/tmp/data/billing',{recursive:true});
      fs.writeFileSync(`/tmp/data/billing/${name}`,env[key]);
    }
  }
  if (env.MUSEUM_CONFIG) fs.writeFileSync('/tmp/museum.yaml',env.MUSEUM_CONFIG);
  for (const [key, name] of [['FCM_SERVICE_ACCOUNT','fcm-service-account.json'], ['PLAYSTORE_SERVICE_ACCOUNT','pst-service-account.json']]) {
    if (env[key]) {
      fs.mkdirSync('/tmp/credentials',{recursive:true});
      fs.writeFileSync(`/tmp/credentials/${name}`,env[key]);
    }
  }
}

const { Museum: GoMuseum } = await import('./build/worker.mjs');
export class Museum extends GoMuseum {
  constructor(state, env) {
    super(state, env);
    this.env = env;
  }
  async fetch(request) {
    prepareFiles(this.env);
    return super.fetch(uncompressedRequest(request));
  }
  alarm(info) {
    prepareFiles(this.env);
    // workers-go expects all fields, including ones omitted by older runtimes.
    return super.alarm({isRetry:false,retryCount:0,scheduledTime:Date.now(),...info});
  }
}

export default {
  async fetch(request, env) {
    const headers = new Headers(request.headers);
    headers.set('X-Museum-Client-IP',request.headers.get('CF-Connecting-IP') || '127.0.0.1');
    return env.MUSEUM.get(env.MUSEUM.idFromName('museum')).fetch(new Request(request,{headers}));
  },
};
