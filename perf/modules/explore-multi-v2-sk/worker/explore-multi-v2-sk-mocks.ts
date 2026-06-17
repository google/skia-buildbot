export const mockParams = [
  { id: 1, key: 'os', value: 'Android' },
  { id: 2, key: 'os', value: 'Ubuntu' },
  { id: 3, key: 'arch', value: 'arm' },
  { id: 4, key: 'arch', value: 'x86' },
  { id: 5, key: 'config', value: '8888' },
  { id: 6, key: 'config', value: 'gpu' },
];

const numTraces = 10;
const stride = 10;

const tracesBuffer = new Uint16Array(numTraces * stride);
for (let i = 0; i < numTraces; i++) {
  const offset = i * stride;
  tracesBuffer[offset] = (i % 2) + 1; // os
  tracesBuffer[offset + 1] = ((i >> 1) % 2) + 3; // arch
  tracesBuffer[offset + 2] = ((i >> 2) % 2) + 5; // config
}

export const mockTraces = tracesBuffer;

export const mockMeta = {
  version: 'test-version',
  count: numTraces,
  stride: stride,
  commonParams: { project: 'Skia' },
};
