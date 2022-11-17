import './index';
import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';
import { hiddenClassName, targetAriaAttribute, TooltipSk } from './tooltip-sk';

import { setUpElementUnderTest } from '../test_util';

const toolTipText = 'This is the tooltip text.';

describe('tooltip-sk', () => {
  const newInstance = setUpElementUnderTest<TooltipSk>('tooltip-sk');

  let element: TooltipSk;
  let input: HTMLElement;
  beforeEach(() => {
    input = document.createElement('input');
    input.id = 'tooltip-target';
    $$('body')!.appendChild(input);

    element = newInstance((el: TooltipSk) => {
      el.target = input.id;
      el.value = toolTipText;
    });
  });

  afterEach(() => {
    $$('body')!.removeChild(input);
  });

  describe('on construction', () => {
    it('is hidden', () => {
      assert.isTrue(element.classList.contains(hiddenClassName));
    });

    it('has has an id', () => {
      assert.isNotEmpty(element.id);
    });

    it('has updated the targets aria-describedby', () => {
      assert.equal(input.getAttribute(targetAriaAttribute), element.id);
    });
  });

  describe('target gets focus', () => {
    it('is no longer hidden', () => {
      input.focus();
      assert.isFalse(element.classList.contains(hiddenClassName));
    });
  });

  describe('change the value', () => {
    it('changes the displayed text', () => {
      assert.equal($$('.content', element)!.textContent, element.value);
      element.value = 'foo';
      assert.equal($$('.content', element)!.textContent, 'foo');
    });
  });
});
