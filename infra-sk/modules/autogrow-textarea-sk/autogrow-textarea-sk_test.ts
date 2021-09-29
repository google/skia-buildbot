import './index';

import { expect } from 'chai';
import { setUpElementUnderTest } from '../test_util';
import { AutogrowTextareaSk } from './autogrow-textarea-sk';

describe('autogrow-textarea-sk', () => {
  // Function to create a new autogrow-textarea-sk.
  const newInstance = setUpElementUnderTest<AutogrowTextareaSk>('autogrow-textarea-sk');

  let autogrowTextareaSk: AutogrowTextareaSk;
  let textarea: HTMLTextAreaElement;

  beforeEach(() => {
    autogrowTextareaSk = newInstance((el) => {
      el.minRows = 4;
      el.placeholder = 'example text';
    });
    textarea = autogrowTextareaSk.querySelector('textarea')!;
  });

  const checkNoScrollBar = () => {
    expect(textarea.clientHeight).to.equal(textarea.scrollHeight);
  };
  const inputText = (text: string) => {
    textarea.value = text;
    textarea.dispatchEvent(new Event('input', { bubbles: true, cancelable: true }));
  };

  it('plumbs through attributes', () => {
    expect(textarea.getAttribute('placeholder')).to.equal('example text');
    expect(textarea).to.have.property('rows', 4);
    checkNoScrollBar();
  });

  it('reflects value', () => {
    inputText('foo');
    expect(autogrowTextareaSk).to.have.property('value', 'foo');
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
