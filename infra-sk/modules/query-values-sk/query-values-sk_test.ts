import './index'
import { QueryValuesSk, QueryValuesSkQueryValuesChangedEventDetail } from './query-values-sk';
import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk';
import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';
import { setUpElementUnderTest, eventPromise } from '../test_util';
import { MultiSelectSk } from 'elements-sk/multi-select-sk/multi-select-sk';

describe('query-values-sk', () => {
  const newInstance = setUpElementUnderTest<QueryValuesSk>('query-values-sk');

  let queryValuesSk: QueryValuesSk;
  let regexCheckbox: CheckOrRadio;
  let invertCheckbox: CheckOrRadio;

  beforeEach(() => {
    queryValuesSk = newInstance();
    queryValuesSk.options = ['x86', 'arm'];

    regexCheckbox = $$<CheckOrRadio>('#regex', queryValuesSk)!;
    invertCheckbox = $$<CheckOrRadio>('#invert', queryValuesSk)!;
  })

  describe('with regex', () => {
    // We mark the beforeEach hook as async to give the MultiSelectSk's MutationObserver microtask
    // a chance to run and detect the selection change before we execute the tests.
    beforeEach(async () => {
      queryValuesSk.selected = ['~ar'];
    });

    it('toggles a regex correctly on invert click', async () => {
      assert.isTrue(regexCheckbox.checked);

      const value = await clickAndWaitForQueryValuesChangedEvent(invertCheckbox);
      assert.deepEqual([], value, 'Event was sent.');

      // Regex and Invert are mutually exclusive.
      assert.isFalse(regexCheckbox.checked, 'Regex checkbox is unchecked.');
      assert.isTrue(invertCheckbox.checked);
    });

    it('toggles a regex correctly for regex click', async () => {
      assert.isTrue(regexCheckbox.checked);

      let value = await clickAndWaitForQueryValuesChangedEvent(regexCheckbox);
      assert.deepEqual([], value, 'Event was sent.');
      assert.isFalse(regexCheckbox.checked, 'Regex is unchecked');
      assert.isFalse(invertCheckbox.checked, 'Invert stays unchecked');

      // Now go back to regex.
      value = await clickAndWaitForQueryValuesChangedEvent(regexCheckbox);
      assert.deepEqual(['~ar'], value, 'Event was sent.');
      assert.isTrue(regexCheckbox.checked, 'Regex is checked');
      assert.isFalse(invertCheckbox.checked, 'Invert stays unchecked');
    });
  });

  describe('normal input', () => {
    // We mark the beforeEach hook as async to give the MultiSelectSk's MutationObserver microtask
    // a chance to run and detect the selection change before we execute the tests.
    beforeEach(async () => {
      queryValuesSk.selected = ['arm'];
    });

    it('toggles invert correctly for invert click', async () => {
      assert.isFalse(regexCheckbox.checked);
      assert.isFalse(invertCheckbox.checked);

      let value = await clickAndWaitForQueryValuesChangedEvent(invertCheckbox);
      assert.deepEqual(['!arm'], value, 'Event was sent.');
      assert.isFalse(regexCheckbox.checked);
      assert.isTrue(invertCheckbox.checked);

      value = await clickAndWaitForQueryValuesChangedEvent(invertCheckbox);
      assert.deepEqual(['arm'], value, 'Event was sent.');
      assert.isFalse(regexCheckbox.checked);
      assert.isFalse(invertCheckbox.checked);
    });
  });

  describe('with inverted input', () => {
    // We mark the beforeEach hook as async to give the MultiSelectSk's MutationObserver microtask
    // a chance to run and detect the selection change before we execute the tests.
    beforeEach(async () => {
      queryValuesSk.selected = ['!arm'];
    });

    it('toggles correctly for invert click when starting inverted', async () => {
      assert.isFalse(regexCheckbox.checked);
      assert.isTrue(invertCheckbox.checked);

      let value = await clickAndWaitForQueryValuesChangedEvent(invertCheckbox);
      assert.deepEqual(['arm'], value, 'Event was sent.');
      assert.isFalse(regexCheckbox.checked);
      assert.isFalse(invertCheckbox.checked);

      value = await clickAndWaitForQueryValuesChangedEvent(invertCheckbox);
      assert.deepEqual(['!arm'], value, 'Event was sent.');
      assert.isFalse(regexCheckbox.checked);
      assert.isTrue(invertCheckbox.checked);
    });

    it('sends right event when value is clicked', async () => {
      assert.isFalse(regexCheckbox.checked);
      assert.isTrue(invertCheckbox.checked);

      const firstValue = $$<MultiSelectSk>('#values', queryValuesSk)!.children[0];
      let value = await clickAndWaitForQueryValuesChangedEvent(firstValue as HTMLElement);
      assert.deepEqual(['!x86', '!arm'], value, 'Event was sent.');

      value = await clickAndWaitForQueryValuesChangedEvent(invertCheckbox);
      assert.deepEqual(['x86', 'arm'], value, 'Event was sent.');
      assert.isFalse(regexCheckbox.checked);
      assert.isFalse(invertCheckbox.checked);
    });
  });

  describe('with invert and regex hidden', () => {
    beforeEach(() => {
      queryValuesSk.setAttribute('hide_invert', 'true');
      queryValuesSk.setAttribute('hide_regex', 'true');
    });

    it('can hide the regex and invert boxes', () => {
      assert.isTrue(regexCheckbox.hasAttribute('hidden'));
      assert.isTrue(invertCheckbox.hasAttribute('hidden'));
    });
  });
});

const clickAndWaitForQueryValuesChangedEvent = async (el: HTMLElement) => {
  const e =
    eventPromise<CustomEvent<QueryValuesSkQueryValuesChangedEventDetail>>(
      'query-values-changed');
  el.click();
  return (await e).detail;
};
