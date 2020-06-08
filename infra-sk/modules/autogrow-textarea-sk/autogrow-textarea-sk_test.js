import './index';

import { $$ } from 'common-sk/modules/dom';

import { setUpElementUnderTest } from '../test_util';

describe('autogrow-textarea-sk', () => {
  // Function to create a new autogrow-textarea-sk.
  const newInstance = setUpElementUnderTest('autogrow-textarea-sk');

  let agTextAreaSk;
  let textarea;
  beforeEach(() => {
    agTextAreaSk = newInstance((el) => {
      el.minRows = 4;
      el.placeholder = 'example text';
    });
    textarea = $$('textarea', agTextAreaSk);
  });

  const checkNoScrollBar = () => {
    expect(textarea.clientHeight).to.equal(textarea.scrollHeight);
  };
  const inputText = (text) => {
    textarea.value = text;
    textarea.dispatchEvent(new Event('input', { bubbles: true, cancelable: true }));
  };

  it('plumbs through attributes', () => {
    expect(textarea).to.have.attribute('placeholder', 'example text');
    expect(textarea).to.have.property('rows', 4);
    checkNoScrollBar();
  });

  it('reflects value', () => {
    inputText('foo');
    expect(agTextAreaSk).to.have.property('value', 'foo');
  });

  it('expands and shrinks to number of lines', () => {
    inputText('\n\n\n\n\n  six lines');
    expect(textarea).to.have.property('rows', 6);
    checkNoScrollBar();
    inputText('\n\n\n\n\n\n\n\n\n  ten lines');
    expect(textarea).to.have.property('rows', 10);
    checkNoScrollBar();
    inputText('\n\n\n\n\n\n  seven lines');
    expect(textarea).to.have.property('rows', 7);
    checkNoScrollBar();
    inputText('one line, but doesn\'t shrink below minRows of 4');
    expect(textarea).to.have.property('rows', 4);
    checkNoScrollBar();
  });
});
