import {execFileSync} from 'node:child_process';
import {fileURLToPath} from 'node:url';
import {join} from 'node:path';

const root = fileURLToPath(new URL('../',import.meta.url));
const goRoot = execFileSync('go',['env','GOROOT'],{encoding:'utf8'}).trim();
const preload = fileURLToPath(new URL('test-context.cjs',import.meta.url));
execFileSync('go',['test',`-exec=${join(goRoot,'lib/wasm/go_js_wasm_exec')}`,
  '-run=^Test(WorkerWebPushDNS|VerifySubscription|StreamObject|SizedReaderRejectsTruncatedAndOversizedData|SendWithClient.*|BatchSize|NextDrainsOverdueJobsBeforeRepeating)$',
  './pkg/utils/s3config','./pkg/utils/playstore','./pkg/utils/email','./pkg/utils/workerjobs','./space/controller'],{
  cwd:root,stdio:'inherit',env:{...process.env,GOOS:'js',GOARCH:'wasm',CGO_ENABLED:'0',NODE_OPTIONS:`${process.env.NODE_OPTIONS || ''} --require=${preload}`},
});
