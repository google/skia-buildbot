import './index';
import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { DebuggerAppSk } from './debugger-app-sk';
import { exampleTraceString } from './demo_data';

describe('debugger-app-sk', () => {
  const newInstance = setUpElementUnderTest<DebuggerAppSk>('debugger-app-sk');

  let debuggerAppSk: DebuggerAppSk;

  beforeEach(() => {
    debuggerAppSk = newInstance();
  });

  it('shows the code after valid data is loaded', () => {
    debuggerAppSk.loadJSONData(exampleTraceString);

    const codeAreaText = $$<HTMLDivElement>('#codeEditor')?.innerText;
    assert.include(codeAreaText, 'first line');
    assert.include(codeAreaText, 'second line');
    assert.notInclude(codeAreaText, 'Invalid');
    assert.notInclude(codeAreaText, 'Unexpected token');
  });

  it('shows an error message after invalid data is loaded', () => {
    debuggerAppSk.loadJSONData('This is invalid data');

    assert.include($$<HTMLDivElement>('#codeEditor')?.innerText, 'Unexpected token');
  });
});
