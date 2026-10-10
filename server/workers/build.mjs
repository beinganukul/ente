import { execFileSync } from 'node:child_process';
import { mkdirSync, readFileSync, writeFileSync, readdirSync, existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { resolve, relative } from 'node:path';
import {rootCertificates} from 'node:tls';

const root = fileURLToPath(new URL('../', import.meta.url));
const build = fileURLToPath(new URL('build/', import.meta.url));
mkdirSync(build, {recursive:true});
const run = (args, env = {}) => execFileSync('go', args, {cwd:root, env:{...process.env, ...env}, stdio:'inherit'});
run(['run','github.com/syumai/workers-go/cmd/workers-assets-gen','-mode=go','-durable-objects=Museum','-o',build]);
function migrationVersion(dialect) {
  const version = Math.max(...readdirSync(resolve(root,`migrations/${dialect}`)).filter(name => name.endsWith('.up.sql')).map(name => Number(name.split('_')[0])));
  if (!Number.isInteger(version) || version < 1) throw new Error(`No ${dialect} migrations found`);
  return version;
}
const versions = `-X main.workerMigrationVersion=${migrationVersion('postgres')} -X main.workerTursoMigrationVersion=${migrationVersion('sqlite')}`;
run(['build','-buildvcs=false','-tags=nomsgpack','-trimpath',`-ldflags=-s -w ${versions}`,'-o',resolve(build,'app.wasm'),'./cmd/museum'], {GOOS:'js',GOARCH:'wasm',CGO_ENABLED:'0'});
const core = resolve(build,'core.mjs');
let source = readFileSync(core,'utf8');
if (!source.includes('const go = new Go();')) throw new Error('workers-go runtime changed: review environment initialization');
source = source.replace('const go = new Go();', 'const go = new Go();\n  go.env = Object.fromEntries(Object.entries(ctx.env).filter(([key, value]) => key.startsWith("ENTE_") && typeof value === "string"));\n  go.env.GOMEMLIMIT = "64MiB";\n  go.env.ENVIRONMENT = "workers";\n  go.env.SSL_CERT_FILE = "/tmp/ca-certificates.pem";\n  go.env.ENTE_INTERNAL_TRUSTED_CLIENT_IP_HEADER = "X-Museum-Client-IP";');
writeFileSync(core,source);
const files = {'ca-certificates.pem':rootCertificates.join('\n')};
function collect(directory) {
  for (const file of readdirSync(directory,{withFileTypes:true})) {
    const path = resolve(directory,file.name);
    if (file.isDirectory()) collect(path);
    else files[relative(root,path)] = readFileSync(path,'utf8');
  }
}
collect(resolve(root,'mail-templates'));
collect(resolve(root,'web-templates'));
for (const name of ['us.json','in.json','black-friday.json']) {
  const path = resolve(root,'data/billing',name);
  if (existsSync(path)) files[`data/billing/${name}`] = readFileSync(path,'utf8');
}
files['configurations/workers.yaml'] = readFileSync(resolve(root,'configurations/local.yaml'),'utf8');
writeFileSync(resolve(build,'resources.mjs'), `export default ${JSON.stringify(files)};\n`);

const bytes = readdirSync(build).reduce((total,name) => total+readFileSync(resolve(build,name)).length,0);
if (bytes >= 64*1024*1024) throw new Error(`Worker build ${bytes} bytes exceeds the uncompressed size limit`);
console.log(`Worker build: ${(bytes/1024/1024).toFixed(2)} MiB uncompressed`);
