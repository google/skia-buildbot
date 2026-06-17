import { mockMeta, mockParams, mockTraces } from './explore-multi-v2-sk-mocks';

const originalFetch = self.fetch;

self.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
  const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url;

  if (url.includes('/_/wasm/meta.json')) {
    return new Response(JSON.stringify(mockMeta), {
      headers: { 'content-type': 'application/json' },
    });
  }
  if (url.includes('/_/wasm/params.json')) {
    return new Response(JSON.stringify(mockParams), {
      headers: { 'content-type': 'application/json' },
    });
  }
  if (url.includes('/_/wasm/traces.bin')) {
    return new Response(mockTraces.buffer);
  }

  return originalFetch(input, init);
};

import './filter.worker';
