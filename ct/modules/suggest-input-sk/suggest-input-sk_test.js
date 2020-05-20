import './index';

import { $, $$ } from 'common-sk/modules/dom';

import { languageList } from './test_data';
import { setUpElementUnderTest } from '../../../golden/modules/test_util';

const DOWN_ARROW = 40;
const UP_ARROW = 38;
const ENTER = 13;

describe('suggest-input-sk', () => {
  // Function to create a new suggest-input-sk, options to give the element
  // can be passed in, and default to a list of programming languages.
  const newInstance = (() => {
    const create = setUpElementUnderTest('suggest-input-sk');
    return (opts) => {
      const si = create();
      opts = opts || languageList;
      si.options = opts;
      return si;
    };
  }
  )();

  const simulateInput = (ele, value) => {
    ele.focus();
    ele.value = value;
    ele.dispatchEvent(new Event('input', {
      bubbles: true,
      cancelable: true,
    }));
  };

  it('hides suggestions initially', () => {
    const suggestInput = newInstance();
    expect($$('div', suggestInput)).to.have.property('hidden', true);
  });

  it('shows suggestions when in focus', () => {
    const suggestInput = newInstance();
    $$('input', suggestInput).focus();
    expect($$('input', suggestInput)).to.equal(document.activeElement);
    expect($$('div', suggestInput)).to.have.property('hidden', false);
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
    const suggestInput = newInstance();
    $$('input', suggestInput).focus();
    $$('input', suggestInput).blur();
    expect($$('div', suggestInput)).to.have.property('hidden', true);
  });

  it('shows only suggestions that substring match', () => {
    const suggestInput = newInstance();
    simulateInput($$('input', suggestInput), 'script');

    expect($$('div', suggestInput)).to.have.property('hidden', false);
    expect($('li', suggestInput).length).to.equal(2);
    // Expect doesn't handle real JS arrays well in all cases, we need the
    // original NodeList.
    expect(suggestInput.querySelectorAll('li'))
      .to.contain.text(['JavaScript', 'TypeScript']);
    expect(suggestInput.querySelectorAll('li'))
      .to.not.contain.text(['Python', 'golang', 'c++']);
    simulateInput($$('input', suggestInput), 'scriptz');
    expect($('li', suggestInput).length).to.equal(0);
    simulateInput($$('input', suggestInput), '');
    expect($('li', suggestInput).length).to.equal(languageList.length);
  });

  it('shows only suggestions that regex match', () => {
    const suggestInput = newInstance();
    simulateInput($$('input', suggestInput), '.*');
    expect($('li', suggestInput).length).to.equal(languageList.length);
    simulateInput($$('input', suggestInput), '[0-9]');
    expect($('li', suggestInput).length).to.equal(2);
    // Expect doesn't handle real JS arrays well in all cases, we need the
    // original NodeList.
    expect(suggestInput.querySelectorAll('li'))
      .to.contain.text(['Python2.7', 'Python3']);
  });

  it('selects suggestion by click', () => {
    const suggestInput = newInstance();
    simulateInput($$('input', suggestInput), 'Python');
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
    const suggestInput = newInstance();
    simulateInput($$('input', suggestInput), 'Python[0-9]');
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
    $$('input', suggestInput).dispatchEvent(
      new KeyboardEvent('keyup', { keyCode: DOWN_ARROW }),
    );
    checkSelected('Python2.7');
    $$('input', suggestInput).dispatchEvent(
      new KeyboardEvent('keyup', { keyCode: DOWN_ARROW }),
    );
    checkSelected('Python3');
    // Wrap around.
    $$('input', suggestInput).dispatchEvent(
      new KeyboardEvent('keyup', { keyCode: DOWN_ARROW }),
    );
    checkSelected('Python2.7');
    // And back up.
    $$('input', suggestInput).dispatchEvent(
      new KeyboardEvent('keyup', { keyCode: UP_ARROW }),
    );
    checkSelected('Python3');
    // Select.
    $$('input', suggestInput).dispatchEvent(
      new KeyboardEvent('keyup', { keyCode: ENTER }),
    );
    expect($$('input', suggestInput).value).to.equal('Python3');
  });

  it('select suggestion by arrows/blur', () => {
    const suggestInput = newInstance();
    simulateInput($$('input', suggestInput), 'Python[0-9]');
    // Go to first suggestion (Python2.7)
    $$('input', suggestInput).dispatchEvent(
      new KeyboardEvent('keyup', { keyCode: DOWN_ARROW }),
    );
    $$('input', suggestInput).blur();
    expect($$('input', suggestInput).value).to.equal('Python2.7');
  });

  it('clears on unlisted value without acceptCustomValue', () => {
    const suggestInput = newInstance();
    simulateInput($$('input', suggestInput), 'blarg');
    $$('input', suggestInput).blur();
    expect($$('input', suggestInput).value).to.equal('');
  });

  it('accepts unlisted value with acceptCustomValue', () => {
    const suggestInput = newInstance();
    suggestInput.acceptCustomValue = true;
    simulateInput($$('input', suggestInput), 'blarg');
    $$('input', suggestInput).blur();
    expect($$('input', suggestInput).value).to.equal('blarg');
  });
});
