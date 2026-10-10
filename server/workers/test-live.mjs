import assert from 'node:assert/strict';
import {randomBytes} from 'node:crypto';
import {setTimeout} from 'node:timers/promises';
import {pathToFileURL} from 'node:url';

export function endpointOrigin(value) {
  const url = new URL(value);
  assert.ok(url.protocol === 'https:' && !url.username && !url.password &&
    url.pathname === '/' && !url.search && !url.hash,
  'Provide an HTTPS origin without credentials, a path, query, or fragment');
  return url.origin;
}

export async function liveRequest(origin, checks, check, path, expected, init = {}) {
  const started = performance.now();
  const result = {check, stage: 'fetch'};
  checks.push(result);
  try {
    const response = await fetch(new URL(path, origin), {...init, signal: AbortSignal.timeout(30000)});
    result.status = response.status;
    result.stage = 'body';
    const body = await response.text();
    result.stage = 'validate';
    assert.equal(response.status, expected, check);
    if (path === '/ping') assert.ok(JSON.parse(body).message === 'pong', 'Invalid ping JSON');
    result.stage = 'complete';
    return response;
  } catch (error) {
    // Only fixed transport identifiers are safe; messages may contain URLs or secrets.
    if (['TypeError', 'AbortError', 'TimeoutError'].includes(error?.name)) result.errorName = error.name;
    if (['ENOTFOUND', 'EAI_AGAIN', 'ECONNREFUSED', 'ECONNRESET', 'ETIMEDOUT',
      'UND_ERR_CONNECT_TIMEOUT', 'UND_ERR_HEADERS_TIMEOUT', 'UND_ERR_BODY_TIMEOUT',
      'UND_ERR_SOCKET'].includes(error?.cause?.code)) result.errorCode = error.cause.code;
    throw error;
  } finally {
    result.latencyMs = Math.round(performance.now()-started);
  }
}

async function checkLive(origin, checks) {
  const lookup = `/users/srp/attributes?email=worker-live-${randomBytes(12).toString('hex')}%40example.com`;
  const request = (...args) => liveRequest(origin, checks, ...args);
  for (const encoding of [undefined, 'identity', 'gzip', 'br']) {
    await request(`ping/${encoding ?? 'default'}`, '/ping', 200,
      encoding ? {headers: {'Accept-Encoding': encoding}} : {});
  }
  await request('unauthorized', '/users/details/v2', 401);
  await request('missing-srp', lookup, 404);
  await request('unknown-route', '/__museum_live_unknown_route__', 404);
  const cors = await request('cors', '/users/details/v2', 200, {
    method: 'OPTIONS', headers: {Origin: 'https://photos.ente.io', 'Access-Control-Request-Method': 'GET'},
  });
  assert.ok(cors.headers.get('Access-Control-Allow-Origin') === 'https://photos.ente.io', 'Invalid CORS origin');
  for (let round = 0; round < 10; round++) {
    const results = await Promise.allSettled(
      Array.from({length: 4}, () => request('concurrent-ping', '/ping', 200)));
    assert.ok(results.every(result => result.status === 'fulfilled'), 'Concurrent checks failed');
  }
  console.log(JSON.stringify({phase: 'idle', passed: checks.length, idleSeconds: 70}));
  await setTimeout(70000);
  await request('idle-srp', lookup, 404);
  await request('idle-ping', '/ping', 200);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const checks = [];
  try {
    assert.equal(process.argv.length, 3, 'Usage: npm run test:live -- https://museum.example');
    const origin = endpointOrigin(process.argv[2]);
    await checkLive(origin, checks);
    const latencies = checks.map(check => check.latencyMs).sort((a, b) => a-b);
    console.log(JSON.stringify({endpoint: origin, passed: true, requests: checks.length,
      latencyMs: {p50: latencies[Math.floor(latencies.length*0.5)], p95: latencies[Math.floor(latencies.length*0.95)], max: latencies.at(-1)}}));
  } catch {
    // Response bodies and transport errors can contain private configuration.
    console.error(JSON.stringify({passed: false, error: 'Live check failed; verify the HTTPS origin and inspect the live tail', checks}));
    process.exitCode = 1;
  }
}
