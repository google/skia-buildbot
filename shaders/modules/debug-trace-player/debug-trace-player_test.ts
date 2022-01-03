import { assert } from 'chai';
import { Convert, DebugTrace, SlotInfo } from '../debug-trace/debug-trace';
import { DebugTracePlayer, VariableData } from './debug-trace-player';

function getStack(trace: DebugTrace, player: DebugTracePlayer): string[] {
  return player.getCallStack().map((funcIdx: number) => trace.functions[funcIdx].name);
}

function makeVarsString(trace: DebugTrace, player: DebugTracePlayer,
                        vars: VariableData[]): string[] {
  return vars.map((varData: VariableData) => {
    if (varData.slotIndex < 0 || varData.slotIndex >= trace.slots.length) {
      return '???';
    }

    const slot: SlotInfo = trace.slots[varData.slotIndex];
    let text: string = varData.dirty ? '##' : '';
    text += slot.name;
    text += player.getSlotComponentSuffix(varData.slotIndex);
    text += ' = ';
    text += varData.value.toString();
    return text;
  });
}

function getLocalVariables(trace: DebugTrace, player: DebugTracePlayer): string[] {
  const frame: number = player.getStackDepth() - 1;
  return makeVarsString(trace, player, player.getLocalVariables(frame));
}

function getGlobalVariables(trace: DebugTrace, player: DebugTracePlayer): string[] {
  return makeVarsString(trace, player, player.getGlobalVariables());
}

const trivialGreenShader = String.raw`
{
  "functions": [{"name": "float4 main(float2 i)", "slot": 0}],
  "slots": [
    {"columns": 4, "index": 0, "kind": 0, "line": 1,
     "name": "[main].result", "retval": 0, "rows": 1, "slot": 0},
    {"columns": 4, "index": 1, "kind": 0, "line": 1,
     "name": "[main].result", "retval": 0, "rows": 1, "slot": 1},
    {"columns": 4, "index": 2, "kind": 0, "line": 1,
     "name": "[main].result", "retval": 0, "rows": 1, "slot": 2},
    {"columns": 4, "index": 3, "kind": 0, "line": 1,
     "name": "[main].result", "retval": 0, "rows": 1, "slot": 3},
    {"columns": 2, "index": 0, "kind": 0, "line": 1,
     "name": "i", "rows": 1, "slot": 4},
    {"columns": 2, "index": 1, "kind": 0, "line": 1,
     "name": "i", "rows": 1, "slot": 5}
  ],
  "source": [
    "vec4 main(vec2 i) {     // Line 1",
    "  return vec4(0,1,0,1); // Line 2",
    "}                       // Line 3"
  ],
  "trace": [
    [2],
    [1, 4, 1109721088],
    [1, 5, -1035403264],
    [4, 1],
    [0, 2],
    [1],
    [1, 1, 1065353216],
    [1, 2],
    [1, 3, 1065353216],
    [4, -1],
    [3]
  ]
}`;

const functionsShader = String.raw`
{
  "functions": [
    {"name": "half4 main(float2 f2)", "slot": 0},
    {"name": "half fnA()", "slot": 1},
    {"name": "half fnB()", "slot": 2}
  ],
  "slots": [
    {"columns": 4, "index": 0, "kind": 0, "line": 7,
     "name": "[main].result", "retval": 0, "rows": 1, "slot": 0},
    {"columns": 4, "index": 1, "kind": 0, "line": 7,
     "name": "[main].result", "retval": 0, "rows": 1, "slot": 1},
    {"columns": 4, "index": 2, "kind": 0, "line": 7,
     "name": "[main].result", "retval": 0, "rows": 1, "slot": 2},
    {"columns": 4, "index": 3, "kind": 0, "line": 7,
     "name": "[main].result", "retval": 0, "rows": 1, "slot": 3},
    {"columns": 2, "index": 0, "kind": 0, "line": 7,
     "name": "f2", "rows": 1, "slot": 4},
    {"columns": 2, "index": 1, "kind": 0, "line": 7,
     "name": "f2", "rows": 1, "slot": 5},
    {"columns": 1, "index": 0, "kind": 0, "line": 4,
     "name": "[fnA].result", "retval": 1, "rows": 1, "slot": 6},
    {"columns": 1, "index": 0, "kind": 0, "line": 1,
     "name": "[fnB].result", "retval": 2, "rows": 1, "slot": 7}
  ],
  "source": [
    "half fnB() {                    // Line 1",
    "    return 0.5;                 // Line 2",
    "}                               // Line 3",
    "half fnA() {                    // Line 4",
    "    return fnB();               // Line 5",
    "}                               // Line 6",
    "half4 main(float2 f2) {         // Line 7",
    "    return fnA().0x01;          // Line 8",
    "}"
  ],
  "trace": [
    [2],
    [1, 4, 1109852160],
    [1, 5, 1107755008],
    [4, 1],
    [0, 8],
    [2, 1],
    [4, 1],
    [0, 5],
    [2, 2],
    [4, 1],
    [0, 2],
    [1, 7, 1056964608],
    [4, -1],
    [3, 2],
    [1, 6, 1056964608],
    [4, -1],
    [3, 1],
    [1],
    [1, 1, 1056964608],
    [1, 2],
    [1, 3, 1065353216],
    [4, -1],
    [3]
  ]
}`;

const variablesShader = String.raw`
{
  "functions": [{"name": "half4 main(float2 p)", "slot": 0}, {"name": "float func()", "slot": 1}],
  "slots": [
    {"columns": 4, "index": 0, "kind": 0, "line": 6, "name": "[main].result", "retval": 0,
     "rows": 1, "slot": 0},
    {"columns": 4, "index": 1, "kind": 0, "line": 6, "name": "[main].result", "retval": 0,
     "rows": 1, "slot": 1},
    {"columns": 4, "index": 2, "kind": 0, "line": 6, "name": "[main].result", "retval": 0,
     "rows": 1, "slot": 2},
    {"columns": 4, "index": 3, "kind": 0, "line": 6, "name": "[main].result", "retval": 0,
     "rows": 1, "slot": 3},
    {"columns": 2, "index": 0, "kind": 0, "line": 6, "name": "p", "rows": 1, "slot": 4},
    {"columns": 2, "index": 1, "kind": 0, "line": 6, "name": "p", "rows": 1, "slot": 5},
    {"columns": 1, "index": 0, "kind": 1, "line": 7, "name": "a", "rows": 1, "slot": 6},
    {"columns": 1, "index": 0, "kind": 3, "line": 8, "name": "b", "rows": 1, "slot": 7},
    {"columns": 1, "index": 0, "kind": 0, "line": 2, "name": "[func].result", "retval": 1,
     "rows": 1, "slot": 8},
    {"columns": 1, "index": 0, "kind": 0, "line": 3, "name": "z", "rows": 1, "slot": 9},
    {"columns": 4, "index": 0, "kind": 0, "line": 10, "name": "c", "rows": 1, "slot": 10},
    {"columns": 4, "index": 1, "kind": 0, "line": 10, "name": "c", "rows": 1, "slot": 11},
    {"columns": 4, "index": 2, "kind": 0, "line": 10, "name": "c", "rows": 1, "slot": 12},
    {"columns": 4, "index": 3, "kind": 0, "line": 10, "name": "c", "rows": 1, "slot": 13},
    {"columns": 3, "index": 0, "kind": 0, "line": 11, "name": "d", "rows": 3, "slot": 14},
    {"columns": 3, "index": 1, "kind": 0, "line": 11, "name": "d", "rows": 3, "slot": 15},
    {"columns": 3, "index": 2, "kind": 0, "line": 11, "name": "d", "rows": 3, "slot": 16},
    {"columns": 3, "index": 3, "kind": 0, "line": 11, "name": "d", "rows": 3, "slot": 17},
    {"columns": 3, "index": 4, "kind": 0, "line": 11, "name": "d", "rows": 3, "slot": 18},
    {"columns": 3, "index": 5, "kind": 0, "line": 11, "name": "d", "rows": 3, "slot": 19},
    {"columns": 3, "index": 6, "kind": 0, "line": 11, "name": "d", "rows": 3, "slot": 20},
    {"columns": 3, "index": 7, "kind": 0, "line": 11, "name": "d", "rows": 3, "slot": 21},
    {"columns": 3, "index": 8, "kind": 0, "line": 11, "name": "d", "rows": 3, "slot": 22}
  ],
  "source": [
    "                                      // Line 1",
    "float func() {                        // Line 2",
    "    float z = 456;                    // Line 3",
    "    return z;                         // Line 4",
    "}                                     // Line 5",
    "half4 main(float2 p) {                // Line 6",
    "    int a = 123;                      // Line 7",
    "    bool b = true;                    // Line 8",
    "    func();                           // Line 9",
    "    float4 c = float4(0, 0.5, 1, -1); // Line 10",
    "    float3x3 d = float3x3(2);         // Line 11",
    "    return c.xyz1;                    // Line 12",
    "}                                     // Line 13"
  ],
  "trace": [
    [2],
    [1, 4, 1107361792],
    [1, 5, 1107361792],
    [4, 1],
    [0, 7],
    [1, 6, 123],
    [0, 8],
    [1, 7, -1],
    [0, 9],
    [2, 1],
    [4, 1],
    [0, 3],
    [1, 9, 1139015680],
    [0, 4],
    [1, 8, 1139015680],
    [4, -1],
    [3, 1],
    [0, 10],
    [1, 10],
    [1, 11, 1056964608],
    [1, 12, 1065353216],
    [1, 13, -1082130432],
    [0, 11],
    [1, 14, 1073741824],
    [1, 15],
    [1, 16],
    [1, 17],
    [1, 18, 1073741824],
    [1, 19],
    [1, 20],
    [1, 21],
    [1, 22, 1073741824],
    [0, 12],
    [1],
    [1, 1, 1056964608],
    [1, 2, 1065353216],
    [1, 3, 1065353216],
    [4, -1],
    [3]
  ]
}`;

const ifStatementShader = String.raw`
{
  "functions": [{"name": "half4 main(float2 p)", "slot": 0}],
  "slots": [
    {"columns": 4, "index": 0, "kind": 0, "line": 2, "name": "[main].result", "retval": 0,
     "rows": 1, "slot": 0},
    {"columns": 4, "index": 1, "kind": 0, "line": 2, "name": "[main].result", "retval": 0,
     "rows": 1, "slot": 1},
    {"columns": 4, "index": 2, "kind": 0, "line": 2, "name": "[main].result", "retval": 0,
     "rows": 1, "slot": 2},
    {"columns": 4, "index": 3, "kind": 0, "line": 2, "name": "[main].result", "retval": 0,
     "rows": 1, "slot": 3},
    {"columns": 2, "index": 0, "kind": 0, "line": 2, "name": "p", "rows": 1, "slot": 4},
    {"columns": 2, "index": 1, "kind": 0, "line": 2, "name": "p", "rows": 1, "slot": 5},
    {"columns": 1, "index": 0, "kind": 1, "line": 3, "name": "val", "rows": 1, "slot": 6},
    {"columns": 1, "index": 0, "kind": 1, "line": 5, "name": "temp", "rows": 1, "slot": 7}
  ],
  "source": [
    "                       // Line 1",
    "half4 main(float2 p) { // Line 2",
    "    int val;           // Line 3",
    "    if (true) {        // Line 4",
    "        int temp = 1;  // Line 5",
    "        val = temp;    // Line 6",
    "    } else {           // Line 7",
    "        val = 2;       // Line 8",
    "    }                  // Line 9",
    "    if (false) {       // Line 10",
    "        int temp = 3;  // Line 11",
    "        val = temp;    // Line 12",
    "    } else {           // Line 13",
    "        val = 4;       // Line 14",
    "    }                  // Line 15",
    "    return half4(val); // Line 16",
    "}                      // Line 17",
    ""
  ],
  "trace": [
    [2],
    [1, 4, 1107361792],
    [1, 5, 1107361792],
    [4, 1],
    [0, 3],
    [1, 6],
    [0, 4],
    [4, 1],
    [0, 5],
    [1, 7, 1],
    [0, 6],
    [1, 6, 1],
    [4, -1],
    [0, 10],
    [4, 1],
    [0, 14],
    [1, 6, 4],
    [4, -1],
    [0, 16],
    [1, 0, 1082130432],
    [1, 1, 1082130432],
    [1, 2, 1082130432],
    [1, 3, 1082130432],
    [4, -1],
    [3]
  ]
}`;

describe('DebugTrace playback', () => {
  it('Hello World: return green', () => {
    const trace: DebugTrace = Convert.toDebugTrace(trivialGreenShader);
    const player = new DebugTracePlayer();
    player.reset(trace);

    // We have not started tracing yet.
    assert.equal(player.getCursor(), 0);
    assert.equal(player.getCurrentLine(), -1);
    assert.deepEqual(player.getLineNumbersReached(), new Map([[2, 1]]));
    assert.isFalse(player.traceHasCompleted());
    assert.deepEqual(getStack(trace, player), []);
    assert.isEmpty(getGlobalVariables(trace, player));

    player.step();

    // We should now be inside main.
    assert.isAbove(player.getCursor(), 0);
    assert.equal(player.getCurrentLine(), 2);
    assert.deepEqual(player.getLineNumbersReached(), new Map([[2, 0]]));
    assert.isFalse(player.traceHasCompleted());
    assert.deepEqual(getStack(trace, player), ['float4 main(float2 i)']);
    assert.deepEqual(getGlobalVariables(trace, player), []);

    player.step();

    // We have now completed the trace.
    assert.isAbove(player.getCursor(), 0);
    assert.isTrue(player.traceHasCompleted());
    assert.equal(player.getCurrentLine(), -1);
    assert.deepEqual(getStack(trace, player), []);
    assert.deepEqual(getGlobalVariables(trace, player), ['##[main].result.x = 0',
                                                         '##[main].result.y = 1',
                                                         '##[main].result.z = 0',
                                                         '##[main].result.w = 1']);
  });

  it('reset() starts over from the beginning', () => {
    const trace: DebugTrace = Convert.toDebugTrace(trivialGreenShader);
    const player = new DebugTracePlayer();
    player.reset(trace);

    // We have not started tracing yet.
    assert.equal(player.getCursor(), 0);
    assert.equal(player.getCurrentLine(), -1);
    assert.isFalse(player.traceHasCompleted());
    assert.deepEqual(getStack(trace, player), []);

    player.step();

    // We should now be inside main.
    assert.isAbove(player.getCursor(), 0);
    assert.equal(player.getCurrentLine(), 2);
    assert.isFalse(player.traceHasCompleted());
    assert.deepEqual(getStack(trace, player), ['float4 main(float2 i)']);

    player.reset(trace);

    // We should be back to square one.
    assert.equal(player.getCursor(), 0);
    assert.equal(player.getCurrentLine(), -1);
    assert.isFalse(player.traceHasCompleted());
    assert.deepEqual(getStack(trace, player), []);
  });

  it('invoking functions', () => {
    const trace: DebugTrace = Convert.toDebugTrace(functionsShader);
    const player = new DebugTracePlayer();
    player.reset(trace);

    // We have not started tracing yet.
    assert.equal(player.getCursor(), 0);
    assert.equal(player.getCurrentLine(), -1);
    assert.isFalse(player.traceHasCompleted());
    assert.deepEqual(getStack(trace, player), []);
    assert.isEmpty(getGlobalVariables(trace, player));

    player.step();

    // We should now be inside main.
    assert.isFalse(player.traceHasCompleted());
    assert.equal(player.getCurrentLine(), 8);
    assert.deepEqual(getStack(trace, player), ['half4 main(float2 f2)']);
    assert.deepEqual(getLocalVariables(trace, player), ['##f2.x = 41.75',
                                                        '##f2.y = 33.75']);
    assert.deepEqual(getGlobalVariables(trace, player), []);

    player.stepOver();

    // We should now have completed execution.
    assert.isTrue(player.traceHasCompleted());
    assert.equal(player.getCurrentLine(), -1);
    assert.deepEqual(getStack(trace, player), []);
    assert.deepEqual(getGlobalVariables(trace, player), ['##[main].result.x = 0',
                                                         '##[main].result.y = 0.5',
                                                         '##[main].result.z = 0',
                                                         '##[main].result.w = 1']);

    // Watch the stack grow and shrink as single-step.
    player.reset(trace);
    player.step();

    assert.deepEqual(getStack(trace, player), ['half4 main(float2 f2)']);
    assert.deepEqual(getLocalVariables(trace, player), ['##f2.x = 41.75',
                                                        '##f2.y = 33.75']);
    assert.deepEqual(getGlobalVariables(trace, player), []);
    player.step();

    assert.deepEqual(getStack(trace, player), ['half4 main(float2 f2)',
                                               'half fnA()']);
    assert.deepEqual(getLocalVariables(trace, player), []);
    assert.deepEqual(getGlobalVariables(trace, player), []);
    player.step();

    assert.deepEqual(getStack(trace, player), ['half4 main(float2 f2)',
                                               'half fnA()',
                                               'half fnB()']);
    assert.deepEqual(getLocalVariables(trace, player), []);
    assert.deepEqual(getGlobalVariables(trace, player), []);
    player.step();

    assert.deepEqual(getStack(trace, player), ['half4 main(float2 f2)',
                                               'half fnA()']);
    assert.deepEqual(getLocalVariables(trace, player), ['##[fnB].result = 0.5']);
    assert.deepEqual(getGlobalVariables(trace, player), []);
    player.step();

    assert.deepEqual(getStack(trace, player), ['half4 main(float2 f2)']);
    assert.deepEqual(getLocalVariables(trace, player), ['##[fnA].result = 0.5',
                                                        'f2.x = 41.75',
                                                        'f2.y = 33.75']);
    assert.deepEqual(getGlobalVariables(trace, player), []);

    player.step();
    assert.isTrue(player.traceHasCompleted());
    assert.deepEqual(getGlobalVariables(trace, player), ['##[main].result.x = 0',
                                                         '##[main].result.y = 0.5',
                                                         '##[main].result.z = 0',
                                                         '##[main].result.w = 1']);
  });

  it('variable display', () => {
    const trace: DebugTrace = Convert.toDebugTrace(variablesShader);
    const player = new DebugTracePlayer();
    player.reset(trace);

    assert.deepEqual(player.getLineNumbersReached(), new Map([[3, 1], [4, 1], [7, 1],
                                                              [8, 1], [9, 1], [10, 1],
                                                              [11, 1], [12, 1]]));
    player.step();

    assert.equal(player.getCurrentLine(), 7);
    assert.deepEqual(getStack(trace, player), ['half4 main(float2 p)']);
    assert.deepEqual(getLocalVariables(trace, player), ['##p.x = 32.25', '##p.y = 32.25']);
    player.step();

    assert.equal(player.getCurrentLine(), 8);
    assert.deepEqual(getStack(trace, player), ['half4 main(float2 p)']);
    assert.deepEqual(getLocalVariables(trace, player), ['##a = 123', 'p.x = 32.25', 'p.y = 32.25']);
    player.step();

    assert.equal(player.getCurrentLine(), 9);
    assert.deepEqual(getStack(trace, player), ['half4 main(float2 p)']);
    assert.deepEqual(getLocalVariables(trace, player),
                     ['##b = true', 'a = 123', 'p.x = 32.25', 'p.y = 32.25']);
    player.step();

    assert.equal(player.getCurrentLine(), 3);
    assert.deepEqual(getStack(trace, player), ['half4 main(float2 p)', 'float func()']);
    assert.deepEqual(getLocalVariables(trace, player), []);
    player.step();

    assert.equal(player.getCurrentLine(), 4);
    assert.deepEqual(getStack(trace, player), ['half4 main(float2 p)', 'float func()']);
    assert.deepEqual(getLocalVariables(trace, player), ['##z = 456']);
    player.step();

    assert.equal(player.getCurrentLine(), 9);
    assert.deepEqual(getStack(trace, player), ['half4 main(float2 p)']);
    assert.deepEqual(getLocalVariables(trace, player),
                     ['##[func].result = 456', 'b = true', 'a = 123',
                      'p.x = 32.25', 'p.y = 32.25']);
    player.step();

    assert.equal(player.getCurrentLine(), 10);
    assert.deepEqual(getStack(trace, player), ['half4 main(float2 p)']);
    assert.deepEqual(getLocalVariables(trace, player),
                     ['b = true', 'a = 123', 'p.x = 32.25', 'p.y = 32.25']);
    player.step();

    assert.equal(player.getCurrentLine(), 11);
    assert.deepEqual(getStack(trace, player), ['half4 main(float2 p)']);
    assert.deepEqual(getLocalVariables(trace, player),
                     ['##c.x = 0', '##c.y = 0.5', '##c.z = 1', '##c.w = -1', 'b = true', 'a = 123',
                      'p.x = 32.25', 'p.y = 32.25']);
    player.step();

    assert.equal(player.getCurrentLine(), 12);
    assert.deepEqual(getStack(trace, player), ['half4 main(float2 p)']);
    assert.deepEqual(getLocalVariables(trace, player),
                     ['##d[0][0] = 2', '##d[0][1] = 0', '##d[0][2] = 0',
                      '##d[1][0] = 0', '##d[1][1] = 2', '##d[1][2] = 0',
                      '##d[2][0] = 0', '##d[2][1] = 0', '##d[2][2] = 2',
                      'c.x = 0', 'c.y = 0.5', 'c.z = 1', 'c.w = -1', 'b = true', 'a = 123',
                      'p.x = 32.25', 'p.y = 32.25']);

    player.step();
    assert.isTrue(player.traceHasCompleted());
    assert.deepEqual(getStack(trace, player), []);
    assert.deepEqual(getGlobalVariables(trace, player),
                     ['##[main].result.x = 0', '##[main].result.y = 0.5',
                      '##[main].result.z = 1', '##[main].result.w = 1']);
  });

  it('if-statement flow control', () => {
    const trace: DebugTrace = Convert.toDebugTrace(ifStatementShader);
    const player = new DebugTracePlayer();
    player.reset(trace);

    assert.deepEqual(player.getLineNumbersReached(), new Map([[3, 1], [4, 1], [5, 1], [6, 1],
                                                              [10, 1], [14, 1], [16, 1]]));
    player.step();

    assert.equal(player.getCurrentLine(), 3);
    assert.deepEqual(getLocalVariables(trace, player), ['##p.x = 32.25', '##p.y = 32.25']);
    player.step();

    assert.equal(player.getCurrentLine(), 4);
    assert.deepEqual(getLocalVariables(trace, player), ['##val = 0', 'p.x = 32.25', 'p.y = 32.25']);
    player.step();

    assert.equal(player.getCurrentLine(), 5);
    assert.deepEqual(getLocalVariables(trace, player), ['val = 0', 'p.x = 32.25', 'p.y = 32.25']);
    player.step();

    assert.equal(player.getCurrentLine(), 6);
    assert.deepEqual(getLocalVariables(trace, player),
                     ['##temp = 1', 'val = 0', 'p.x = 32.25', 'p.y = 32.25']);
    player.step();

    // We skip over the false-branch.
    assert.equal(player.getCurrentLine(), 10);
    assert.deepEqual(getLocalVariables(trace, player), ['##val = 1', 'p.x = 32.25', 'p.y = 32.25']);
    player.step();

    // We skip over the true-branch.
    assert.equal(player.getCurrentLine(), 14);
    assert.deepEqual(getLocalVariables(trace, player), ['val = 1', 'p.x = 32.25', 'p.y = 32.25']);
    player.step();

    assert.equal(player.getCurrentLine(), 16);
    assert.deepEqual(getLocalVariables(trace, player), ['##val = 4', 'p.x = 32.25', 'p.y = 32.25']);
    player.step();

    assert.isTrue(player.traceHasCompleted());
    assert.deepEqual(getGlobalVariables(trace, player),
                     ['##[main].result.x = 4', '##[main].result.y = 4',
                      '##[main].result.z = 4', '##[main].result.w = 4']);
  });
});
