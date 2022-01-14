import './index';
import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { DebuggerAppSk } from './debugger-app-sk';
import { exampleTraceString } from './demo_data';
import CodeMirror from 'codemirror';

function makeFakeLocalStorage(store: Record<string, string>): Storage {
  const fakeLocalStorage: Storage = {
    length: 0,
    getItem: (key: string): string | null => {
      return key in store ? store[key] as string : null;
    },
    setItem: (key: string, value: string) => {
      store[key] = `${value}`;
      length = Object.keys(store).length;
    },
    removeItem: (key: string) => {
      delete store[key];
      length = Object.keys(store).length;
    },
    clear: () => {
      store = {};
    },
    key: function (index: number): string | null {
      return Object.keys(store)[index];
    }
  };
  return fakeLocalStorage;
}

function getLinesWithBgClass(app: DebuggerAppSk, expectedType: string): number[] {
  const editor: CodeMirror.Editor = app.getEditor()!;
  assert.isNotNull(editor);

  // Search for lines with the given background class.
  let result: number[] = [];
  for (let index = 0; index < editor.lineCount(); ++index) {
    const info = editor!.lineInfo(index);
    if (info.bgClass === expectedType) {
      // CodeMirror line numbers are zero-indexed, so add 1 to compensate.
      result.push(index + 1);
    }
  }

  return result;
}

function getCurrentLine(app: DebuggerAppSk): number | null {
  const lines: number[] = getLinesWithBgClass(app, 'cm-current-line');
  assert.isAtMost(lines.length, 1);
  return (lines.length > 0) ? lines[0] : null;
}

function getLinesWithBreakpointMarker(app: DebuggerAppSk, expectedMarker: string): number[] {
  const editor: CodeMirror.Editor = app.getEditor()!;
  assert.isNotNull(editor);

  // Search for lines with the given background class.
  let result: number[] = [];
  for (let index = 0; index < editor.lineCount(); ++index) {
    const info = editor.lineInfo(index);
    if ('cm-breakpoints' in (info.gutterMarkers ?? {})) {
      if (info.gutterMarkers['cm-breakpoints'].classList.contains(expectedMarker)) {
        // CodeMirror line numbers are zero-indexed, so add 1 to compensate.
        result.push(index + 1);
      }
    }
  }

  return result;
}

function getBreakpointableLines(app: DebuggerAppSk): number[] {
  // Returns line which could have a breakpoint set, but currently don't.
  return getLinesWithBreakpointMarker(app, 'cm-reachable');
}

function getBreakpointLines(app: DebuggerAppSk): number[] {
  // Returns line which currently have a breakpoint set.
  return getLinesWithBreakpointMarker(app, 'cm-breakpoint');
}

const newInstance = setUpElementUnderTest<DebuggerAppSk>('debugger-app-sk');

describe('debugger app', () => {
  let debuggerAppSk: DebuggerAppSk;

  beforeEach(() => {
    debuggerAppSk = newInstance();
  });

  it('shows the code after valid data is loaded', () => {
    debuggerAppSk.loadJSONData(exampleTraceString);

    const codeAreaText = $$<HTMLDivElement>('#codeEditor')?.innerText;
    assert.include(codeAreaText, 'half4 convert(float2 c) {');
    assert.include(codeAreaText, 'half4 c = convert(p * 0.001);');
    assert.notInclude(codeAreaText, 'Invalid');
    assert.notInclude(codeAreaText, 'Unexpected token');
  });

  it('shows an error message after invalid data is loaded', () => {
    debuggerAppSk.loadJSONData('This is invalid data');

    assert.include($$<HTMLDivElement>('#codeEditor')?.innerText, 'Unexpected token');
  });

  const entrypointLine = 6;
  const helperFunctionLine = 2;

  it('shows breakpointable markers on lines with code', () => {
    debuggerAppSk.loadJSONData(exampleTraceString);
    assert.sameDeepMembers(getBreakpointableLines(debuggerAppSk),
                           [entrypointLine, entrypointLine + 1,
                            helperFunctionLine, helperFunctionLine + 1]);
  });

  it('shows breakpoint markers on lines after breakpoints are set', () => {
    debuggerAppSk.loadJSONData(exampleTraceString);
    debuggerAppSk.toggleBreakpoint(helperFunctionLine);
    debuggerAppSk.toggleBreakpoint(entrypointLine + 1);
    assert.sameDeepMembers(getBreakpointableLines(debuggerAppSk),
                           [entrypointLine, helperFunctionLine + 1]);
    assert.sameDeepMembers(getBreakpointLines(debuggerAppSk),
                           [entrypointLine + 1, helperFunctionLine]);
  });

  it('shows an error message after invalid data is loaded', () => {
    debuggerAppSk.loadJSONData('This is invalid data');

    assert.include($$<HTMLDivElement>('#codeEditor')?.innerText, 'Unexpected token');
  });


  it('highlights the entrypoint after valid data is loaded', () => {
    debuggerAppSk.loadJSONData(exampleTraceString);
    assert.equal(getCurrentLine(debuggerAppSk), entrypointLine);
  });

  it('highlights the next line after stepping over', () => {
    debuggerAppSk.loadJSONData(exampleTraceString);
    debuggerAppSk.stepOver();
    assert.equal(getCurrentLine(debuggerAppSk), entrypointLine + 1);
  });

  it('highlights first line of the helper function after stepping in', () => {
    debuggerAppSk.loadJSONData(exampleTraceString);
    debuggerAppSk.step();
    assert.equal(getCurrentLine(debuggerAppSk), helperFunctionLine);
  });

  it('completes the trace after stepping out', () => {
    debuggerAppSk.loadJSONData(exampleTraceString);
    debuggerAppSk.stepOut();
    assert.equal(getCurrentLine(debuggerAppSk), null);
  });

  it('completes the trace after running without a breakpoint set', () => {
    debuggerAppSk.loadJSONData(exampleTraceString);
    debuggerAppSk.run();
    assert.equal(getCurrentLine(debuggerAppSk), null);
  });

  it('runs until a breakpoint is hit', () => {
    debuggerAppSk.loadJSONData(exampleTraceString);
    debuggerAppSk.toggleBreakpoint(helperFunctionLine);
    debuggerAppSk.run();
    assert.equal(getCurrentLine(debuggerAppSk), helperFunctionLine);
  });

  it('highlights each line in sequential order when single-stepping', () => {
    debuggerAppSk.loadJSONData(exampleTraceString);
    assert.equal(getCurrentLine(debuggerAppSk), entrypointLine);
    debuggerAppSk.step();
    assert.equal(getCurrentLine(debuggerAppSk), helperFunctionLine);
    debuggerAppSk.step();
    assert.equal(getCurrentLine(debuggerAppSk), helperFunctionLine + 1);
    debuggerAppSk.step();
    assert.equal(getCurrentLine(debuggerAppSk), entrypointLine);
    debuggerAppSk.step();
    assert.equal(getCurrentLine(debuggerAppSk), entrypointLine + 1);
    debuggerAppSk.step();
    assert.equal(getCurrentLine(debuggerAppSk), null);
  });
});

describe('local storage', () => {
  let debuggerAppSk: DebuggerAppSk;

  it('loads a trace when populated and ?local-storage query param exists', () => {
    debuggerAppSk = newInstance((self: DebuggerAppSk) => {
      self.setLocalStorageForTest(makeFakeLocalStorage({'sksl-debug-trace': exampleTraceString}));
      self.setQueryParameterForTest('?local-storage');
    });

    const codeAreaText = $$<HTMLDivElement>('#codeEditor')?.innerText;
    assert.include(codeAreaText, 'half4 convert(float2 c) {');
    assert.include(codeAreaText, 'half4 c = convert(p * 0.001);');
    assert.notInclude(codeAreaText, 'Drag in a DebugTrace JSON file to start the debugger.');
  });

  it('does nothing when ?local-storage query param is not present', () => {
    debuggerAppSk = newInstance((self: DebuggerAppSk) => {
      self.setLocalStorageForTest(makeFakeLocalStorage({'sksl-debug-trace': exampleTraceString}));
    });

    const codeAreaText = $$<HTMLDivElement>('#codeEditor')?.innerText;
    assert.notInclude(codeAreaText, 'half4 convert(float2 c) {');
    assert.include(codeAreaText, 'Drag in a DebugTrace JSON file to start the debugger.');
  });

  it('does nothing when local storage is invalid, even if ?local-storage is set', () => {
    debuggerAppSk = newInstance((self: DebuggerAppSk) => {
      self.setLocalStorageForTest(makeFakeLocalStorage({'sksl-debug-trace': '{}'}));
      self.setQueryParameterForTest('?local-storage');
    });

    const codeAreaText = $$<HTMLDivElement>('#codeEditor')?.innerText;
    assert.notInclude(codeAreaText, 'half4 convert(float2 c) {');
    assert.include(codeAreaText, 'Drag in a DebugTrace JSON file to start the debugger.');
  });

  it('does nothing when local storage is empty, even if ?local-storage is set', () => {
    debuggerAppSk = newInstance((self: DebuggerAppSk) => {
      self.setLocalStorageForTest(makeFakeLocalStorage({}));
      self.setQueryParameterForTest('?local-storage');
    });

    const codeAreaText = $$<HTMLDivElement>('#codeEditor')?.innerText;
    assert.notInclude(codeAreaText, 'half4 convert(float2 c) {');
    assert.include(codeAreaText, 'Drag in a DebugTrace JSON file to start the debugger.');
  });
});
