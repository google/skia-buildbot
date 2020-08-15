import './index';
import {
  QueryValuesSk,
  QueryValuesSkQueryValuesChangedEventDetail,
} from './query-values-sk';
import { QueryValuesSkPO } from './query-values-sk_po';
import { assert } from 'chai';
import { setUpElementUnderTest, eventPromise } from '../test_util';

describe('query-values-sk', () => {
  const newInstance = setUpElementUnderTest<QueryValuesSk>('query-values-sk');

  let queryValuesSk: QueryValuesSk;
  let queryValuesSkPO: QueryValuesSkPO;

  beforeEach(() => {
    queryValuesSk = newInstance();
    queryValuesSk.options = ['x86', 'arm'];

    queryValuesSkPO = new QueryValuesSkPO(queryValuesSk);
  });

  describe('selected property setter', () => {
    it('removes tildes', () => {
      queryValuesSk.selected = ['~ar'];
      assert.deepEqual(['ar'], queryValuesSk.selected);
    });

    it('removes checks', () => {
      queryValuesSk.selected = ['!x86', '!arm'];
      assert.deepEqual(['x86', 'arm'], queryValuesSk.selected);
    });

    it('leaves the value unchanged if it does not contain checks or tildes', () => {
      queryValuesSk.selected = ['x86', 'arm'];
      assert.deepEqual(['x86', 'arm'], queryValuesSk.selected);
    });
  });

  describe('with regex', () => {
    // We mark the beforeEach hook as async to give the MultiSelectSk's MutationObserver microtask
    // a chance to run and detect the selection change before we execute the tests.
    beforeEach(async () => {
      queryValuesSk.selected = ['~ar'];
    });

    it('shows the options and initial selection', async () => {
      assert.deepEqual(['x86', 'arm'], await queryValuesSkPO.getOptions());
      assert.deepEqual(['~ar'], await queryValuesSkPO.getSelected());
    });

    it('allows the user to set the regex', async () => {
      assert.isTrue(await queryValuesSkPO.isRegexCheckboxChecked());

      const event = queryValuesChangedEventPromise();
      await queryValuesSkPO.setRegexValue('x8');
      const value = (await event).detail;

      assert.deepEqual({ invert: false, regex: true, values: ['~x8'] }, value);
    });

    it('toggles a regex correctly on invert click', async () => {
      assert.isTrue(await queryValuesSkPO.isRegexCheckboxChecked());

      const value = await clickInvertAndWaitForEvent();
      assert.deepEqual(value.values, []);
      assert.isTrue(value.invert);
      assert.isFalse(value.regex);

      // Regex and Invert are mutually exclusive.
      assert.isFalse(
        await queryValuesSkPO.isRegexCheckboxChecked(),
        'Regex checkbox is unchecked'
      );
      assert.isTrue(
        await queryValuesSkPO.isInvertCheckboxChecked(),
        'Invert is checked'
      );
    });

    it('toggles a regex correctly for regex click', async () => {
      assert.isTrue(await queryValuesSkPO.isRegexCheckboxChecked());

      let value = await clickRegexAndWaitForEvent();
      assert.deepEqual([], value.values);
      assert.isFalse(value.invert);
      assert.isFalse(value.regex);
      assert.isFalse(
        await queryValuesSkPO.isRegexCheckboxChecked(),
        'Regex is unchecked'
      );
      assert.isFalse(
        await queryValuesSkPO.isInvertCheckboxChecked(),
        'Invert stays unchecked'
      );

      // Now go back to regex.
      value = await clickRegexAndWaitForEvent();
      assert.deepEqual(
        { invert: false, regex: true, values: ['~ar'] },
        value,
        'Event was sent.'
      );
      assert.isTrue(
        await queryValuesSkPO.isRegexCheckboxChecked(),
        'Regex is checked'
      );
      assert.isFalse(
        await queryValuesSkPO.isInvertCheckboxChecked(),
        'Invert stays unchecked'
      );
    });
  });

  describe('normal input', () => {
    // We mark the beforeEach hook as async to give the MultiSelectSk's MutationObserver microtask
    // a chance to run and detect the selection change before we execute the tests.
    beforeEach(async () => {
      queryValuesSk.selected = ['arm'];
    });

    it('shows the options and initial selection', async () => {
      assert.deepEqual(['x86', 'arm'], await queryValuesSkPO.getOptions());
      assert.deepEqual(['arm'], await queryValuesSkPO.getSelected());
    });

    it('can change the selection via the UI', async () => {
      await queryValuesSkPO.setSelected(['x86']);
      assert.deepEqual(['x86'], queryValuesSk.selected);
    });

    it('toggles invert correctly for invert click', async () => {
      assert.isFalse(await queryValuesSkPO.isRegexCheckboxChecked());
      assert.isFalse(await queryValuesSkPO.isInvertCheckboxChecked());

      let value = await clickInvertAndWaitForEvent();
      assert.deepEqual(
        { invert: true, regex: false, values: ['!arm'] },
        value,
        'Event was sent.'
      );
      assert.isFalse(await queryValuesSkPO.isRegexCheckboxChecked());
      assert.isTrue(await queryValuesSkPO.isInvertCheckboxChecked());

      value = await clickInvertAndWaitForEvent();
      assert.deepEqual(
        { invert: false, regex: false, values: ['arm'] },
        value,
        'Event was sent.'
      );
      assert.isFalse(await queryValuesSkPO.isRegexCheckboxChecked());
      assert.isFalse(await queryValuesSkPO.isInvertCheckboxChecked());
    });
  });

  describe('with inverted input', () => {
    // We mark the beforeEach hook as async to give the MultiSelectSk's MutationObserver microtask
    // a chance to run and detect the selection change before we execute the tests.
    beforeEach(async () => {
      queryValuesSk.selected = ['!arm'];
    });

    it('shows the options and initial selection', async () => {
      assert.deepEqual(['x86', 'arm'], await queryValuesSkPO.getOptions());
      assert.deepEqual(['!arm'], await queryValuesSkPO.getSelected());
    });

    it('toggles correctly for invert click when starting inverted', async () => {
      assert.isFalse(await queryValuesSkPO.isRegexCheckboxChecked());
      assert.isTrue(await queryValuesSkPO.isInvertCheckboxChecked());

      let value = await clickInvertAndWaitForEvent();
      assert.deepEqual(
        { invert: false, regex: false, values: ['arm'] },
        value,
        'Event was sent.'
      );
      assert.isFalse(await queryValuesSkPO.isRegexCheckboxChecked());
      assert.isFalse(await queryValuesSkPO.isInvertCheckboxChecked());

      value = await clickInvertAndWaitForEvent();
      assert.deepEqual(
        { invert: true, regex: false, values: ['!arm'] },
        value,
        'Event was sent.'
      );
      assert.isFalse(await queryValuesSkPO.isRegexCheckboxChecked());
      assert.isTrue(await queryValuesSkPO.isInvertCheckboxChecked());
    });

    it('sends right event when value is clicked', async () => {
      assert.isFalse(await queryValuesSkPO.isRegexCheckboxChecked());
      assert.isTrue(await queryValuesSkPO.isInvertCheckboxChecked());

      const event = queryValuesChangedEventPromise();
      await queryValuesSkPO.clickOption('x86');
      let value = (await event).detail;
      assert.deepEqual(
        { invert: true, regex: false, values: ['!x86', '!arm'] },
        value,
        'Event was sent.'
      );

      value = await clickInvertAndWaitForEvent();
      assert.deepEqual(
        { invert: false, regex: false, values: ['x86', 'arm'] },
        value,
        'Event was sent.'
      );
      assert.isFalse(await queryValuesSkPO.isRegexCheckboxChecked());
      assert.isFalse(await queryValuesSkPO.isInvertCheckboxChecked());
    });
  });

  describe('with invert and regex hidden', () => {
    beforeEach(() => {
      queryValuesSk.setAttribute('hide_invert', 'true');
      queryValuesSk.setAttribute('hide_regex', 'true');
    });

    it('can hide the regex and invert boxes', async () => {
      assert.isTrue(await queryValuesSkPO.isInvertCheckboxHidden());
      assert.isTrue(await queryValuesSkPO.isRegexCheckboxHidden());
    });
  });

  const queryValuesChangedEventPromise = () =>
    eventPromise<CustomEvent<QueryValuesSkQueryValuesChangedEventDetail>>(
      'query-values-changed'
    );

  const clickInvertAndWaitForEvent = async () => {
    const event = queryValuesChangedEventPromise();
    await queryValuesSkPO.clickInvertCheckbox();
    return (await event).detail;
  };

  const clickRegexAndWaitForEvent = async () => {
    const event = queryValuesChangedEventPromise();
    await queryValuesSkPO.clickRegexCheckbox();
    return (await event).detail;
  };
});
