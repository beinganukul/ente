// Mock context for unit tests that import workers-go without using its runtime APIs.
globalThis.context = {binding: {}, env: {}, ctx: {}};
// Go's Wasm runner has a small argv/env buffer; unit tests need no host environment.
for (const key of Object.keys(process.env)) delete process.env[key];
