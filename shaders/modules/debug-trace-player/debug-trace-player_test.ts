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
});
