import './index';

import { $, $$ } from 'common-sk/modules/dom';

import { setUpElementUnderTest } from '../../../golden/modules/test_util';

const DOWN_ARROW = 40;
const UP_ARROW = 38;
const ENTER = 13;

describe('expandable-textarea-sk', () => {
  // Function to create a new expandable-textarea-sk.
  const newInstance = setUpElementUnderTest('expandable-textarea-sk');

  let et;
  beforeEach(() => {
    et = newInstance((el) => {
      el.displayText = 'Click to toggle';
    });
  });

  // Simulates up/down/enter navigation through suggestion list.
  const simulateKeyboardNavigation = (code) => {
    $$('input', et).dispatchEvent(
      new KeyboardEvent('keyup', { keyCode: code }),
    );
  };

  const simulateUserClick = () => {
    $$('a', et).click();
  };

  const simulateUserClickAway = () => {
    $$('input', et).blur();
  };

  // Simulates a user typing 'value' into the input element by setting its
  // value and triggering the built-in 'input' event.
  const simulateUserTyping = (value) => {
    const ele = $$('input', et);
    ele.focus();
    ele.value = value;
    ele.dispatchEvent(new Event('input', {
      bubbles: true,
      cancelable: true,
    }));
  };

  it('displays clickable text', () => {
    expect(et).to.not.have.attribute('open');
    expect($$('collapse-sk', et)).to.have.attribute('closed');
    expect(et).to.contain('expand-more-icon-sk').and.not.contain('expand-less-icon-sk');
    expect($$('a', et)).to.contain.text('Click to toggle');
  });

  it('expands on click with textarea in focus', () => {
    simulateUserClick();
    expect(et).to.have.attribute('open');
    expect($$('collapse-sk', et)).to.not.have.attribute('closed');
    expect(et).to.contain('expand-less-icon-sk').and.not.contain('expand-more-icon-sk');
    expect($$('textarea', et)).to.equal(document.activeElement);
  });

  it('collapses on second click ', () => {
    simulateUserClick();
    simulateUserClick();
    expect(et).to.not.have.attribute('open');
    expect($$('collapse-sk', et)).to.have.attribute('closed');
    expect(et).to.contain('expand-more-icon-sk').and.not.contain('expand-less-icon-sk');
  });

/*
  it('hides suggestions when loses focus', () => {
    simulateUserClick();
    simulateUserClickAway();
    expect($$('div', et)).to.have.property('hidden', true);
  });

  it('shows only suggestions that substring match', () => {
    simulateUserTyping('script');

    expect($$('div', et)).to.have.property('hidden', false);
    expect($('li', et).length).to.equal(2);
    // Expect doesn't handle real JS arrays well in all cases, we need the
    // original NodeList.
    expect(et.querySelectorAll('li'))
      .to.contain.text(['JavaScript', 'TypeScript']);
    expect(et.querySelectorAll('li'))
      .to.not.contain.text(['Python', 'golang', 'c++']);

    simulateUserTyping('scriptz');
    expect($('li', et).length).to.equal(0);
    simulateUserTyping('');
    expect($('li', et).length).to.equal(languageList.length);
  });

  it('shows only suggestions that regex match', () => {
    simulateUserTyping('.*');
    expect($('li', et).length).to.equal(languageList.length);
    simulateUserTyping('[0-9]');
    expect($('li', et).length).to.equal(2);
    // Expect doesn't handle real JS arrays well in all cases, we need the
    // original NodeList.
    expect(et.querySelectorAll('li'))
      .to.contain.text(['Python2.7', 'Python3']);
  });

  it('selects suggestion by click', () => {
    simulateUserTyping('Python');
    expect($('li', et).length).to.equal(4);
    // Expect doesn't handle real JS arrays well in all cases, we need the
    // original NodeList.
    expect(et.querySelectorAll('li'))
      .to.contain.text(['Python', 'Python2.7', 'Python3', 'IronPython']);
    // Click 'Python2.7'.
    $('li', et)[1].dispatchEvent(
      new MouseEvent('click', { bubbles: true, cancelable: true }),
    );
    expect($$('input', et).value).to.equal('Python2.7');
  });

  it('select suggestion by arrows/enter', () => {
    simulateUserTyping('Python[0-9]');
    expect($('li', et).length).to.equal(2);
    // Expect doesn't handle real JS arrays well in all cases, we need the
    // original NodeList.
    expect(et.querySelectorAll('li'))
      .to.contain.text(['Python2.7', 'Python3']);
    // Helper to check that only the expected list item is selected.
    const checkSelected = (expected) => {
      const selected = et.querySelectorAll('li.selected');
      expect(selected).to.have.lengthOf(1).and.have.text([expected]);
    };
    // Navigate down the list.
    simulateKeyboardNavigation(DOWN_ARROW);
    checkSelected('Python2.7');
    simulateKeyboardNavigation(DOWN_ARROW);
    checkSelected('Python3');
    // Wrap around.
    simulateKeyboardNavigation(DOWN_ARROW);
    checkSelected('Python2.7');
    // And back up.
    simulateKeyboardNavigation(UP_ARROW);
    checkSelected('Python3');
    // Select.
    simulateKeyboardNavigation(ENTER);
    expect($$('input', et).value).to.equal('Python3');
  });

  it('select suggestion by arrows/blur', () => {
    simulateUserTyping('Python[0-9]');
    // Go to first suggestion (Python2.7)
    simulateKeyboardNavigation(DOWN_ARROW);
    simulateUserClickAway();
    expect($$('input', et).value).to.equal('Python2.7');
  });

  it('clears on unlisted value without acceptCustomValue', () => {
    simulateUserTyping('blarg');
    simulateUserClickAway();
    expect($$('input', et).value).to.equal('');
  });

  it('accepts unlisted value with acceptCustomValue', () => {
    et.acceptCustomValue = true;
    simulateUserTyping('blarg');
    simulateUserClickAway();
    expect($$('input', et).value).to.equal('blarg');
  });*/
});
