// Durable Object fetch can decode bodies while retaining Content-Encoding.
// Have Go emit plain bytes and let Cloudflare negotiate client compression.
export function uncompressedRequest(request) {
  const headers = new Headers(request.headers);
  headers.delete('Accept-Encoding');
  return new Request(request,{headers});
}
