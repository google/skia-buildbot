import './index';
import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { DebuggerAppSk } from './debugger-app-sk';
import { exampleTraceString } from './demo_data';
import CodeMirror from 'codemirror';

function getMarkedLines(app: DebuggerAppSk, markType: string): number[] {
  const editor: CodeMirror.Editor = app.getEditor()!;
  assert.isNotNull(editor);

  // Search for marks with the given class. We expect marks to cover an entire line (from
  // char 0 of one line, to char 0 of the following line).
  let markedLines: number[] = [];
  editor.getAllMarks().forEach((marker: CodeMirror.TextMarker) => {
    if (marker.className === markType) {
      const pos: CodeMirror.MarkerRange = marker.find() as CodeMirror.MarkerRange;
      assert.isDefined(pos);
      assert.equal(pos.from.ch, 0);
      assert.equal(pos.to.ch, 0);
      assert.equal(pos.to.line, pos.from.line + 1);

      // CodeMirror line numbers are zero-indexed, so add 1 to compensate.
      markedLines.push(pos.from.line + 1);
    }
  });

  return markedLines;
}

function getCurrentLine(app: DebuggerAppSk): number | null {
  const markedLines = getMarkedLines(app, 'cm-current-line');
  assert.isAtMost(markedLines.length, 1);
  return (markedLines.length > 0) ? markedLines[0] : null;
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
