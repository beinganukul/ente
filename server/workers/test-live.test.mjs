import assert from 'node:assert/strict';
import {test} from 'node:test';
import {endpointOrigin,liveRequest} from './test-live.mjs';

test('live checks require an explicit HTTPS origin and never send URL credentials', () => {
  assert.equal(endpointOrigin('https://museum.example/'), 'https://museum.example');
  for (const value of [undefined, '/relative', 'http://museum.example',
    'https://user:secret@museum.example', 'https://museum.example/path',
    'https://museum.example?token=secret', 'https://museum.example/#fragment']) {
    assert.throws(() => endpointOrigin(value));
  }
});

test('live diagnostics retain the failing stage and only allowlisted transport identifiers', async t => {
  const checks = [];
  t.mock.method(globalThis, 'fetch', async () => {
    throw Object.assign(new TypeError('fetch failed https://private.example/?token=secret'),
      {cause: {code: 'ECONNRESET', message: 'secret'}});
  });
  await assert.rejects(liveRequest('https://museum.example', checks, 'ping', '/ping', 200));
  assert.equal(checks[0].stage, 'fetch');
  assert.equal(checks[0].errorName, 'TypeError');
  assert.equal(checks[0].errorCode, 'ECONNRESET');
  assert.ok(!JSON.stringify(checks).includes('secret'));
  globalThis.fetch.mock.mockImplementation(async () => ({status: 200, text: async () => {
    throw Object.assign(new Error('private body'), {cause: {code: 'SECRET'}});
  }}));
  await assert.rejects(liveRequest('https://museum.example', checks, 'ping', '/ping', 200));
  assert.equal(checks[1].stage, 'body');
  assert.ok(!('errorCode' in checks[1]) && !('errorName' in checks[1]));
  assert.ok(!JSON.stringify(checks).includes('private'));
});
