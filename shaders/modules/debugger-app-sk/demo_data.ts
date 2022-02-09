import { DebugTrace } from '../debug-trace/generate/debug-trace-quicktype';

export const exampleTrace: DebugTrace = {
  version: '20220209',
  source: [
    'half4 convert(float2 c) {',
    '    float4 color = c.xy11;',
    '    return half4(color);',
    '}',
    'half4 main(float2 p) {',
    '    half4 c = convert(p * 0.001);',
    '    return c;',
    '}'
  ],
  slots: [
   {name: '[main].result', columns: 4, rows: 1, index: 0, kind: 0, line: 5, retval: 0},
   {name: '[main].result', columns: 4, rows: 1, index: 1, kind: 0, line: 5, retval: 0},
   {name: '[main].result', columns: 4, rows: 1, index: 2, kind: 0, line: 5, retval: 0},
   {name: '[main].result', columns: 4, rows: 1, index: 3, kind: 0, line: 5, retval: 0},
   {name: 'p', columns: 2, rows: 1, index: 0, kind: 0, line: 5},
   {name: 'p', columns: 2, rows: 1, index: 1, kind: 0, line: 5},
   {name: 'c', columns: 4, rows: 1, index: 0, kind: 0, line: 6},
   {name: 'c', columns: 4, rows: 1, index: 1, kind: 0, line: 6},
   {name: 'c', columns: 4, rows: 1, index: 2, kind: 0, line: 6},
   {name: 'c', columns: 4, rows: 1, index: 3, kind: 0, line: 6},
   {name: '[convert].result', columns: 4, rows: 1, index: 0, kind: 0, line: 1, retval: 1},
   {name: '[convert].result', columns: 4, rows: 1, index: 1, kind: 0, line: 1, retval: 1},
   {name: '[convert].result', columns: 4, rows: 1, index: 2, kind: 0, line: 1, retval: 1},
   {name: '[convert].result', columns: 4, rows: 1, index: 3, kind: 0, line: 1, retval: 1},
   {name: 'c', columns: 2, rows: 1, index: 0, kind: 0, line: 1},
   {name: 'c', columns: 2, rows: 1, index: 1, kind: 0, line: 1},
   {name: 'color', columns: 4, rows: 1, index: 0, kind: 0, line: 2},
   {name: 'color', columns: 4, rows: 1, index: 1, kind: 0, line: 2},
   {name: 'color', columns: 4, rows: 1, index: 2, kind: 0, line: 2},
   {name: 'color', columns: 4, rows: 1, index: 3, kind: 0, line: 2}
  ],
  functions: [
    { name: 'half4 main(float2 p)' },
    { name: 'half4 convert(float2 c)' }
  ],
  trace: [
    [2], [1, 4, 1048576000], [1, 5, 1048576000], [4, 1], [0, 6], [2, 1], [1, 14, 964891247],
    [1, 15, 964891247], [4, 1], [0, 2], [1, 16, 964891247], [1, 17, 964891247],
    [1, 18, 1065353216], [1, 19, 1065353216], [0, 3], [1, 10, 964891247], [1, 11, 964891247],
    [1, 12, 1065353216], [1, 13, 1065353216], [4, -1], [3, 1], [1, 6, 964891247],
    [1, 7, 964891247], [1, 8, 1065353216], [1, 9, 1065353216], [0, 7], [1, 0, 964891247],
    [1, 1, 964891247], [1, 2, 1065353216], [1, 3, 1065353216], [4, -1], [3]
  ]
};

export const exampleTraceString: string = JSON.stringify(exampleTrace);
