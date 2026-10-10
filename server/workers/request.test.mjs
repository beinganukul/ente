import assert from 'node:assert/strict';
import {test} from 'node:test';
import {uncompressedRequest} from './request.mjs';

test('Only Go compression negotiation is removed; request authentication/body survive', async () => {
  const original = new Request('https://museum.example/files',{method:'POST',headers:{'Accept-Encoding':'gzip, br','X-Auth-Token':'test-token','Content-Type':'application/json'},body:'{"encrypted":"data"}'});
  const request = uncompressedRequest(original);
  assert.equal(request.headers.get('Accept-Encoding'),null);
  assert.equal(original.headers.get('Accept-Encoding'),'gzip, br');
  assert.equal(request.headers.get('X-Auth-Token'),'test-token');
  assert.equal(request.headers.get('Content-Type'),'application/json');
  assert.equal(request.method,'POST');
  assert.equal(request.url,original.url);
  assert.equal(await request.text(),'{"encrypted":"data"}');
});
