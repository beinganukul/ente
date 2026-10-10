// Mock context for unit tests that import workers-go without using its runtime APIs.
globalThis.context = {binding: {}, env: {}, ctx: {}};
