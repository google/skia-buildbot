import './index';
import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { DebuggerAppSk } from './debugger-app-sk';
import { exampleTraceString } from './demo_data';
import CodeMirror from 'codemirror';

function getLinesWithBgClass(app: DebuggerAppSk, expectedType: string): number[] {
  const editor: CodeMirror.Editor = app.getEditor()!;
  assert.isNotNull(editor);

  // Search for lines with the given background class.
  let result: number[] = [];
  for (let index = 0; index < editor!.lineCount(); ++index) {
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

describe('debugger-app-sk', () => {
  const newInstance = setUpElementUnderTest<DebuggerAppSk>('debugger-app-sk');

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
