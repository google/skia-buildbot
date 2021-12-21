import { assert, expect } from 'chai';
import { Convert, DebugTrace } from './debug-trace';

describe('DebugTrace JSON parsing', () => {
  it('converts a valid JSON string to a DebugTrace struct', () => {
    const text = String.raw`
    {
      "source": [
        "\t// first line",
        "// \"second line\"",
        "//\\\\//\\\\ third line"
      ],
      "slots": [
        {
          "slot": 0,
          "name": "SkVM_DebugTrace",
          "columns": 1,
          "rows": 2,
          "index": 3,
          "kind": 4,
          "line": 5
        },
        {
          "slot": 1,
          "name": "Unit_Test",
          "columns": 6,
          "rows": 7,
          "index": 8,
          "kind": 9,
          "line": 10,
          "retval": 11
        }
      ],
      "functions": [{ "slot": 0, "name": "void testFunc();" }],
      "trace": [[2], [0, 5], [1, 10, 15], [3, 20]]
    }`;
    const trace: DebugTrace = Convert.toDebugTrace(text);

    assert.equal(trace.source.length, 3);
    assert.equal(trace.slots.length, 2);
    assert.equal(trace.functions.length, 1);
    assert.equal(trace.trace.length, 4);

    assert.equal(trace.source[0], '\t// first line');
    assert.equal(trace.source[1], '// "second line"');
    assert.equal(trace.source[2], '//\\\\//\\\\ third line');

    assert.equal(trace.slots[0].name, 'SkVM_DebugTrace');
    assert.equal(trace.slots[0].columns, 1);
    assert.equal(trace.slots[0].rows, 2);
    assert.equal(trace.slots[0].index, 3);
    assert.equal(trace.slots[0].kind, 4);
    assert.equal(trace.slots[0].line, 5);
    assert.isUndefined(trace.slots[0].retval);

    assert.equal(trace.slots[1].name, 'Unit_Test');
    assert.equal(trace.slots[1].columns, 6);
    assert.equal(trace.slots[1].rows, 7);
    assert.equal(trace.slots[1].index, 8);
    assert.equal(trace.slots[1].kind, 9);
    assert.equal(trace.slots[1].line, 10);
    assert.equal(trace.slots[1].retval, 11);

    assert.equal(trace.functions[0].slot, 0);
    assert.equal(trace.functions[0].name, 'void testFunc();');

    // Verify that trailing zeros are re-added to the trace arrays.
    assert.deepEqual(trace.trace[0], [2, 0, 0]);
    assert.deepEqual(trace.trace[1], [0, 5, 0]);
    assert.deepEqual(trace.trace[2], [1, 10, 15]);
    assert.deepEqual(trace.trace[3], [3, 20, 0]);
  });

  it('throws when parsing invalid JSON', () => {
    // Some trailing bracket close characters have been removed, so the JSON isn't valid.
    const text = String.raw`
    {
      "source": [
        "\t// first line",
        "// \"second line\"",
        "//\\\\//\\\\ third line"
      ],
      "slots": [
        {
          "slot": 0,
          "name": "SkVM_DebugTrace",
          "columns": 1,
          "rows": 2,
          "index": 3,
          "kind": 4,
          "line": 5
        },
        {
          "slot": 1,
          "name": "Unit_Test",
          "columns": 6,
          "rows": 7,
          "index": 8,
          "kind": 9,
          "line": 10,
          "retval": 11
        }
      ],
      "functions": [{ "slot": 0, "name": "void testFunc();" }],
      "trace": [[2], [0, 5], [1, 10, 15], [3, 20
    }`;
    expect(() => { Convert.toDebugTrace(text); }).to.throw();
  });

  it('throws when missing a key', () => {
    // This is valid JSON, but it is missing a required key ("functions").
    const text = String.raw`
    {
      "source": [
        "\t// first line",
        "// \"second line\"",
        "//\\\\//\\\\ third line"
      ],
      "slots": [
        {
          "slot": 0,
          "name": "SkVM_DebugTrace",
          "columns": 1,
          "rows": 2,
          "index": 3,
          "kind": 4,
          "line": 5
        },
        {
          "slot": 1,
          "name": "Unit_Test",
          "columns": 6,
          "rows": 7,
          "index": 8,
          "kind": 9,
          "line": 10,
          "retval": 11
        }
      ],
      "trace": [[2], [0, 5], [1, 10, 15], [3, 20]]
    }`;
    expect(() => { Convert.toDebugTrace(text); }).to.throw();
  });

  it('throws when finding an invalid key', () => {
    // This is valid JSON, but it has an extra key we don't know about ("bogus").
    const text = String.raw`
    {
      "source": [
        "\t// first line",
        "// \"second line\"",
        "//\\\\//\\\\ third line"
      ],
      "slots": [
        {
          "slot": 0,
          "name": "SkVM_DebugTrace",
          "columns": 1,
          "rows": 2,
          "index": 3,
          "kind": 4,
          "line": 5
        },
        {
          "slot": 1,
          "name": "Unit_Test",
          "columns": 6,
          "rows": 7,
          "index": 8,
          "kind": 9,
          "line": 10,
          "retval": 11
        }
      ],
      "trace": [[2], [0, 5], [1, 10, 15], [3, 20]],
      "bogus": [9999]
    }`;
    expect(() => { Convert.toDebugTrace(text); }).to.throw();
  });
});
