import './index';

import { $, $$ } from 'common-sk/modules/dom';

import { languageList } from './test_data';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

const DOWN_ARROW = 40;
const UP_ARROW = 38;
const ENTER = 13;

describe('suggest-input-sk', () => {
  // Function to create a new suggest-input-sk, options to give the element
  // can be passed in, and default to a list of programming languages.
  const newInstance = setUpElementUnderTest('suggest-input-sk');

  let suggestInput;
  beforeEach(() => {
    suggestInput = newInstance((el) => {
      el.options = languageList;
    });
  });

  // Simulates up/down/enter navigation through suggestion list.
  const simulateKeyboardNavigation = (code) => {
    $$('input', suggestInput).dispatchEvent(
      new KeyboardEvent('keyup', { keyCode: code }),
    );
  };

  const simulateUserClick = () => {
    $$('input', suggestInput).focus();
  };

  const simulateUserClickAway = () => {
    $$('input', suggestInput).blur();
  };

  // Simulates a user typing 'value' into the input element by setting its
  // value and triggering the built-in 'input' event.
  const simulateUserTyping = (value) => {
    const ele = $$('input', suggestInput);
    ele.focus();
    ele.value = value;
    ele.dispatchEvent(new Event('input', {
      bubbles: true,
      cancelable: true,
    }));
  };

  it('hides suggestions initially', () => {
    expect($$('.suggest-list', suggestInput)).to.have.property('hidden', true);
  });

  it('shows suggestions when in focus', () => {
    simulateUserClick();
    expect($$('input', suggestInput)).to.equal(document.activeElement);
    expect($$('.suggest-list', suggestInput)).to.have.property('hidden', false);
    expect($('li', suggestInput).length).to.equal(languageList.length);
    // Expect doesn't handle real JS arrays well in all cases, we need the
    // original NodeList.
    expect(suggestInput.querySelectorAll('li')).to.contain.text([
      'golang',
      'c++',
      'JavaScript',
      'TypeScript',
      'Python',
      'Python2.7',
      'Python3',
      'IronPython']);
  });

  it('hides suggestions when loses focus', () => {
    simulateUserClick();
    simulateUserClickAway();
    expect($$('.suggest-list', suggestInput)).to.have.property('hidden', true);
  });

  it('shows only suggestions that substring match', () => {
    simulateUserTyping('script');

    expect($$('.suggest-list', suggestInput)).to.have.property('hidden', false);
    expect($('li', suggestInput).length).to.equal(2);
    // Expect doesn't handle real JS arrays well in all cases, we need the
    // original NodeList.
    expect(suggestInput.querySelectorAll('li'))
      .to.contain.text(['JavaScript', 'TypeScript']);
    expect(suggestInput.querySelectorAll('li'))
      .to.not.contain.text(['Python', 'golang', 'c++']);

    simulateUserTyping('scriptz');
    expect($('li', suggestInput).length).to.equal(0);
    simulateUserTyping('');
    expect($('li', suggestInput).length).to.equal(languageList.length);
  });

  it('shows only suggestions that regex match', () => {
    simulateUserTyping('.*');
    expect($('li', suggestInput).length).to.equal(languageList.length);
    simulateUserTyping('[0-9]');
    expect($('li', suggestInput).length).to.equal(2);
    // Expect doesn't handle real JS arrays well in all cases, we need the
    // original NodeList.
    expect(suggestInput.querySelectorAll('li'))
      .to.contain.text(['Python2.7', 'Python3']);
  });

  it('selects suggestion by click', () => {
    simulateUserTyping('Python');
    expect($('li', suggestInput).length).to.equal(4);
    // Expect doesn't handle real JS arrays well in all cases, we need the
    // original NodeList.
    expect(suggestInput.querySelectorAll('li'))
      .to.contain.text(['Python', 'Python2.7', 'Python3', 'IronPython']);
    // Click 'Python2.7'.
    $('li', suggestInput)[1].dispatchEvent(
      new MouseEvent('click', { bubbles: true, cancelable: true }),
    );
    expect($$('input', suggestInput).value).to.equal('Python2.7');
  });

  it('select suggestion by arrows/enter', () => {
    simulateUserTyping('Python[0-9]');
    expect($('li', suggestInput).length).to.equal(2);
    // Expect doesn't handle real JS arrays well in all cases, we need the
    // original NodeList.
    expect(suggestInput.querySelectorAll('li'))
      .to.contain.text(['Python2.7', 'Python3']);
    // Helper to check that only the expected list item is selected.
    const checkSelected = (expected) => {
      const selected = suggestInput.querySelectorAll('li.selected');
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
    expect($$('input', suggestInput).value).to.equal('Python3');
  });

  it('select suggestion by arrows/blur', () => {
    simulateUserTyping('Python[0-9]');
    // Go to first suggestion (Python2.7)
    simulateKeyboardNavigation(DOWN_ARROW);
    simulateUserClickAway();
    expect($$('input', suggestInput).value).to.equal('Python2.7');
  });

  it('clears on unlisted value without acceptCustomValue', () => {
    simulateUserTyping('blarg');
    simulateUserClickAway();
    expect($$('input', suggestInput).value).to.equal('');
  });

  it('accepts unlisted value with acceptCustomValue', () => {
    suggestInput.acceptCustomValue = true;
    simulateUserTyping('blarg');
    simulateUserClickAway();
    expect($$('input', suggestInput).value).to.equal('blarg');
  });
});
