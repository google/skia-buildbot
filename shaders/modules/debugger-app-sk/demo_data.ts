import { DebugTrace } from '../debug-trace/generate/debug-trace-quicktype';

export const exampleTrace: DebugTrace = {
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
   {slot: 0, name: '[main].result', columns: 4, rows: 1, index: 0, kind: 0, line: 5, retval: 0},
   {slot: 1, name: '[main].result', columns: 4, rows: 1, index: 1, kind: 0, line: 5, retval: 0},
   {slot: 2, name: '[main].result', columns: 4, rows: 1, index: 2, kind: 0, line: 5, retval: 0},
   {slot: 3, name: '[main].result', columns: 4, rows: 1, index: 3, kind: 0, line: 5, retval: 0},
   {slot: 4, name: 'p', columns: 2, rows: 1, index: 0, kind: 0, line: 5},
   {slot: 5, name: 'p', columns: 2, rows: 1, index: 1, kind: 0, line: 5},
   {slot: 6, name: 'c', columns: 4, rows: 1, index: 0, kind: 0, line: 6},
   {slot: 7, name: 'c', columns: 4, rows: 1, index: 1, kind: 0, line: 6},
   {slot: 8, name: 'c', columns: 4, rows: 1, index: 2, kind: 0, line: 6},
   {slot: 9, name: 'c', columns: 4, rows: 1, index: 3, kind: 0, line: 6},
   {slot: 10, name: '[convert].result', columns: 4, rows: 1, index: 0, kind: 0, line: 1, retval: 1},
   {slot: 11, name: '[convert].result', columns: 4, rows: 1, index: 1, kind: 0, line: 1, retval: 1},
   {slot: 12, name: '[convert].result', columns: 4, rows: 1, index: 2, kind: 0, line: 1, retval: 1},
   {slot: 13, name: '[convert].result', columns: 4, rows: 1, index: 3, kind: 0, line: 1, retval: 1},
   {slot: 14, name: 'c', columns: 2, rows: 1, index: 0, kind: 0, line: 1},
   {slot: 15, name: 'c', columns: 2, rows: 1, index: 1, kind: 0, line: 1},
   {slot: 16, name: 'color', columns: 4, rows: 1, index: 0, kind: 0, line: 2},
   {slot: 17, name: 'color', columns: 4, rows: 1, index: 1, kind: 0, line: 2},
   {slot: 18, name: 'color', columns: 4, rows: 1, index: 2, kind: 0, line: 2},
   {slot: 19, name: 'color', columns: 4, rows: 1, index: 3, kind: 0, line: 2}
  ],
  functions: [
    { slot: 0, name: 'half4 main(float2 p)' },
    { slot: 1, name: 'half4 convert(float2 c)' }
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
