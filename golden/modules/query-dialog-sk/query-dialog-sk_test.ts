import './index';

import { setUpElementUnderTest, eventPromise, noEventPromise } from '../../../infra-sk/modules/test_util';
import { QueryDialogSk } from './query-dialog-sk';
import { ParamSet, fromParamSet } from 'common-sk/modules/query';
import { $, $$ } from 'common-sk/modules/dom';

const expect = chai.expect;

describe('query-dialog-sk', () => {
  const newInstance = setUpElementUnderTest<QueryDialogSk>('query-dialog-sk');

  let queryDialogSk: QueryDialogSk;
  beforeEach(() => {
    queryDialogSk = newInstance();
  });

  describe('open/close/edit events', () => {
    it('should emit "query-dialog-open" when opened', async () => {
      const event = eventPromise('query-dialog-open');
      queryDialogSk.open({}, '');
      await event;
    });

    it('should emit "query-dialog-close" but not "edit" when closed via the "Cancel" button',
        async () => {
      queryDialogSk.open({}, '');
      const events = Promise.all([eventPromise('query-dialog-close'), noEventPromise('edit')]);
      clickCancelBtn();
      await events;
    });

    it('should emit "query-dialog-close" and "edit" when closed via the "Show Matches" button',
        async () => {
      queryDialogSk.open({}, '');
      const events = Promise.all([eventPromise('query-dialog-close'), eventPromise('edit')]);
      clickShowMatchesBtn();
      await events;
    });
  })

  describe('opened with an empty selection', () => {
    const paramSet: ParamSet = {'a': ['1', '2', '3'], 'b': ['4', '5'], 'c': ['6']};

    beforeEach(() => {
      queryDialogSk.open(paramSet, /* selection= */ '');
    });

    it('should have an empty selection', () => {
      // The query-sk component correctly shows the ParamSet.
      expect(querySkContents()).to.deep.equal(paramSet);

      // But none of the ParamSet items are selected.
      expect(querySkSelection()).to.deep.equal({});

      // The "empty selection" placeholder text is visible instead of the paramset-sk component.
      expect(isEmptySelectionPlaceholderTextVisible()).to.be.true;
      expect(isParamSetSkVisible()).to.be.false;
    });

    it('should update paramset-sk when selection changes', async () => {
      // Select a=1.
      clickQuerySkKey('a');
      clickQuerySkValue('1');
      expect(querySkSelection()).to.deep.equal({'a': ['1']});
      expect(paramSetSkContents()).to.deep.equal({'a': ['1']});

      // The placeholder text should not be visible. It suffices to assert this just once.
      expect(isEmptySelectionPlaceholderTextVisible()).to.be.false;

      // Select a=2.
      clickQuerySkValue('2');
      expect(querySkSelection()).to.deep.equal({'a': ['1', '2']});
      expect(paramSetSkContents()).to.deep.equal({'a': ['1', '2']});

      // Select b=4.
      clickQuerySkKey('b');
      clickQuerySkValue('4', );
      expect(querySkSelection()).to.deep.equal({'a': ['1', '2'], 'b': ['4']});
      expect(paramSetSkContents()).to.deep.equal({'a': ['1', '2'], 'b': ['4']});
    });

    it('should emit event "edit" containing the current selection when "Show Matches" is clicked',
        async () => {
      // Select a=1.
      clickQuerySkKey('a');
      clickQuerySkValue('1');

      // Select a=2.
      clickQuerySkValue('2');

      // Select b=4.
      clickQuerySkKey('b');
      clickQuerySkValue('4', );

      // Click "Show Matches" button and catch the "edit" event.
      const event = eventPromise<CustomEvent<string>>('edit');
      clickShowMatchesBtn();
      const eventSelection = (await event).detail;

      // The event contents should match the selection.
      expect(eventSelection).to.equal('a=1&a=2&b=4');
    });

    it('should clear the previous selection when reopened with an empty selection', async () => {
      // Select a=1.
      clickQuerySkKey('a');
      clickQuerySkValue('1');

      // It should have selected a=1.
      expect(querySkSelection()).to.deep.equal({'a': ['1']});
      expect(paramSetSkContents()).to.deep.equal({'a': ['1']});

      // Close dialog.
      clickShowMatchesBtn();

      // Reopen with same ParamSet and empty selection.
      queryDialogSk.open(paramSet, /* selection= */ '');

      // Selection should be empty.
      expect(querySkSelection()).to.deep.equal({});
      expect(paramSetSkContents()).to.deep.equal({});
    });
  })

  describe('opened with a non-empty selection', () => {
    const paramSet: ParamSet = {'a': ['1', '2', '3'], 'b': ['4', '5'], 'c': ['6']};
    const selection: ParamSet = {'a': ['1', '2'], 'b': ['4']};

    beforeEach(() => {
      queryDialogSk.open(paramSet, fromParamSet(selection));
    });

    it('shows the passed in selection', () => {
      // Both query-sk and paramset-sk show the passed in selection.
      expect(querySkSelection()).to.deep.equal(selection);
      expect(paramSetSkContents()).to.deep.equal(selection);

      // The "empty selection" placeholder text is not visible.
      expect(isEmptySelectionPlaceholderTextVisible()).to.be.false;
    });

    it('can be reopened with a different selection', () => {
      const differentSelection: ParamSet = {'a': ['2', '3'], 'c': ['6']};

      // Close dialog and reopen it with a different selection.
      clickCancelBtn();
      queryDialogSk.open(paramSet, fromParamSet(differentSelection));

      // Both query-sk and paramset-sk show the passed in selection.
      expect(querySkSelection()).to.deep.equal(differentSelection);
      expect(paramSetSkContents()).to.deep.equal(differentSelection);
    });
  });

  describe('reopened with a different ParamSet', () => {
    const paramSet: ParamSet = {'a': ['1', '2', '3'], 'b': ['4', '5'], 'c': ['6']};
    const selection: ParamSet = {'a': ['3'], 'b': ['4']};

    const differentParamSet: ParamSet = {'a': ['3', '4', '5'], 'b': ['6'], 'z': ['7']};
    const differentSelection: ParamSet = {'a': ['3', '4'], 'b': ['6']};

    beforeEach(() => {
      queryDialogSk.open(paramSet, fromParamSet(selection));
    })

    it('can be reopened with a different ParamSet and an empty selection', () => {
      // Close dialog and reopen it with a different ParamSet.
      clickCancelBtn();
      queryDialogSk.open(differentParamSet, /* selection= */ '');

      // The query-sk component shows the new ParamSet, and the selection is empty.
      expect(querySkContents()).to.deep.equal(differentParamSet);
      expect(querySkSelection()).to.deep.equal({});

      // The "empty selection" placeholder text is visible instead of the paramset-sk component.
      expect(isEmptySelectionPlaceholderTextVisible()).to.be.true;
      expect(isParamSetSkVisible()).to.be.false;
    });

    it('can be reopened with a different ParamSet and a non-empty selection', () => {
      // Close dialog and reopen it with a different ParamSet and a non-empty selection.
      clickCancelBtn();
      queryDialogSk.open(differentParamSet, fromParamSet(differentSelection));

      // Both query-sk and paramset-sk show the passed in selection.
      expect(querySkSelection()).to.deep.equal(differentSelection);
      expect(paramSetSkContents()).to.deep.equal(differentSelection);

      // The placeholder text should not be visible.
      expect(isEmptySelectionPlaceholderTextVisible()).to.be.false;
    });
  });

  it('rationalizes an invalid selection', () => {
    const paramSet: ParamSet = {'a': ['1', '2', '3'], 'b': ['4', '5'], 'c': ['6']};

    // This contains the invalid value "a=4" and a value for the invalid key "d".
    const invalidSelection: ParamSet = {'a': ['2', '3', '4'], 'b': ['5'], 'd': ['7']};

    // This is the invalidSelection with the invalid key/value pairs removed.
    const rationalizedSelection: ParamSet = {'a': ['2', '3'], 'b': ['5']};

    // Open dialog with invalid selection.
    queryDialogSk.open(paramSet, fromParamSet(invalidSelection));

    // The dialog should rationalize the invalid selection.
    expect(querySkSelection()).to.deep.equal(rationalizedSelection);
    expect(paramSetSkContents()).to.deep.equal(rationalizedSelection);
  });

  it('can customize the submit button label', () => {
    // Dialog contents do not matter.
    queryDialogSk.open({'a': ['1']}, '');

    const showMatchesBtn = $$<HTMLButtonElement>('button.show-matches', queryDialogSk)!;

    // Shows "Show Matches" by default.
    expect(showMatchesBtn.innerText).to.equal('Show Matches');

    // Button label can be changed.
    queryDialogSk.submitButtonLabel = 'Submit';
    expect(showMatchesBtn.innerText).to.equal('Submit');
  });

  // Clicks the given key in the query-sk component.
  const clickQuerySkKey =
    (key: string) =>
      $<HTMLDivElement>('query-sk select-sk div', queryDialogSk)
        .find(div => div.textContent === key)!
        .click();

  // Clicks the given value in the query-sk component.
  const clickQuerySkValue =
    (value: string) =>
      $$<HTMLDivElement>(`query-sk multi-select-sk div[value="${value}"]`, queryDialogSk)!.click();

  // Clicks the "Show Matches" button. This closes the dialog.
  const clickShowMatchesBtn =
    () => $$<HTMLButtonElement>('button.show-matches', queryDialogSk)!.click();

  // Clicks the "Cancel" button. This closes the dialog.
  const clickCancelBtn =
    () => $$<HTMLButtonElement>('button.cancel', queryDialogSk)!.click();

  // Returns the ParamSet displayed by the query-sk component.
  const querySkContents = (onlySelected=false): ParamSet => {
    const paramSet: ParamSet = {};

    // We'll restore the original selected key after we're done.
    const originalSelectedKey =
      $$<HTMLDivElement>('query-sk select-sk div[selected]', queryDialogSk);

    // Iterate over all keys.
    $<HTMLDivElement>('query-sk select-sk div', queryDialogSk).forEach(keyDiv => {
      const key = keyDiv.innerText; // Extract key.
      keyDiv.click(); // Select the current key.

      // Iterate over all values for the current key.
      $<HTMLDivElement>('query-sk multi-select-sk div', queryDialogSk).forEach(valueDiv => {
        const value = valueDiv.getAttribute('value')!;
        const isSelected = valueDiv.hasAttribute('selected');

        if (onlySelected && !isSelected) {
          return;
        }

        // Insert current key/value pair into the ParamSet.
        if (paramSet[key] === undefined) {
          paramSet[key] = [];
        }
        paramSet[key].push(value);
      })
    });

    // Restore original selected key, if any.
    originalSelectedKey?.click();

    return paramSet;
  }

  // Returns the query-sk component's selection.
  const querySkSelection = () => querySkContents(/* onlySelected= */ true);

  // Returns the ParamSet displayed by the paramset-sk component.
  const paramSetSkContents = (): ParamSet => {
    const paramSet: ParamSet = {};
    $('paramset-sk tr', queryDialogSk).forEach((tr, i) => {
      if (i === 0) return; // Skip the first row, which usually displays titles (empty in our case).
      const key = $$('th', tr)!.textContent!;
      const values = $('div', tr).map(div => div.textContent!);
      paramSet[key] = values;
    })
    return paramSet;
  }

  const isEmptySelectionPlaceholderTextVisible =
    () => $$('p.empty-selection', queryDialogSk) !== null;

  const isParamSetSkVisible =
    () => $$('param-set-sk', queryDialogSk) !== null;
});
