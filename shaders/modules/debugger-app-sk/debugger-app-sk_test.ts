import './index';
import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { DebuggerAppSk } from './debugger-app-sk';
import { exampleTraceString } from './demo_data';
import CodeMirror from 'codemirror';

describe('debugger-app-sk', () => {
  const newInstance = setUpElementUnderTest<DebuggerAppSk>('debugger-app-sk');

  let debuggerAppSk: DebuggerAppSk;

  beforeEach(() => {
    debuggerAppSk = newInstance();
  });

  it('shows the code after valid data is loaded', () => {
    debuggerAppSk.loadJSONData(exampleTraceString);

    const codeAreaText = $$<HTMLDivElement>('#codeEditor')?.innerText;
    assert.include(codeAreaText, 'half4 main(float2 p) {');
    assert.include(codeAreaText, 'return (p.xy * 0.001).xy11;');
    assert.notInclude(codeAreaText, 'Invalid');
    assert.notInclude(codeAreaText, 'Unexpected token');
  });

  it('shows an error message after invalid data is loaded', () => {
    debuggerAppSk.loadJSONData('This is invalid data');

    assert.include($$<HTMLDivElement>('#codeEditor')?.innerText, 'Unexpected token');
  });


  it('highlights the second line after valid data is loaded', () => {
    debuggerAppSk.loadJSONData(exampleTraceString);

    const editor: CodeMirror.Editor = debuggerAppSk.getEditor()!;
    assert.isNotNull(editor);

    // Search for marks with a class of 'cm-current-line'. There should only be a single mark
    // and it should cover the entire line (from char 0, to char 0 on the next line).
    let currentLine: number[] = [];
    editor.getAllMarks().forEach((marker: CodeMirror.TextMarker) => {
      if (marker.className === 'cm-current-line') {
        const pos: CodeMirror.MarkerRange = marker.find() as CodeMirror.MarkerRange;
        assert.isDefined(pos);
        assert.equal(pos.from.ch, 0);
        assert.equal(pos.to.ch, 0);
        assert.equal(pos.to.line, pos.from.line + 1);
        currentLine.push(pos.from.line);
      }
    });

    // CodeMirror line numbers are zero-indexed, so [1] indicates line 2.
    assert.deepEqual(currentLine, [1]);
  });
});
