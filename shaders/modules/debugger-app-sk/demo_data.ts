import { DebugTrace } from '../debug-trace/generate/debug-trace-quicktype';

export const exampleTrace: DebugTrace = {
  functions: [{ name: 'half4 main(float2 p)', slot: 0 }],
  slots: [
    { columns: 4, index: 0, kind: 0, line: 1, name: '[main].result', retval: 0, rows: 1, slot: 0 },
    { columns: 4, index: 1, kind: 0, line: 1, name: '[main].result', retval: 0, rows: 1, slot: 1 },
    { columns: 4, index: 2, kind: 0, line: 1, name: '[main].result', retval: 0, rows: 1, slot: 2 },
    { columns: 4, index: 3, kind: 0, line: 1, name: '[main].result', retval: 0, rows: 1, slot: 3 },
    { columns: 2, index: 0, kind: 0, line: 1, name: 'p', rows: 1, slot: 4 },
    { columns: 2, index: 1, kind: 0, line: 1, name: 'p', rows: 1, slot: 5 },
  ],
  source: [
    'half4 main(float2 p) {',
    '    return (p.xy * 0.001).xy11;',
    '}'
  ],
  trace: [
    [2],
    [1, 4, 1048576000],
    [1, 5, 1048576000],
    [4, 1],
    [0, 2],
    [1, 0, 964891247],
    [1, 1, 964891247],
    [1, 2, 1065353216],
    [1, 3, 1065353216],
    [4, -1],
    [3],
  ],
};

export const exampleTraceString: string = JSON.stringify(exampleTrace);
