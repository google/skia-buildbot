import './index';

import { setUpElementUnderTest, eventPromise, noEventPromise } from '../../../infra-sk/modules/test_util';
import { QueryDialogSk } from './query-dialog-sk';
import { QueryDialogSkPO } from './query-dialog-sk_po';
import { ParamSet, fromParamSet } from 'common-sk/modules/query';
import { $$ } from 'common-sk/modules/dom';

const expect = chai.expect;

describe('query-dialog-sk', () => {
  const newInstance = setUpElementUnderTest<QueryDialogSk>('query-dialog-sk');

  let queryDialogSk: QueryDialogSk;
  let queryDialogSkPO: QueryDialogSkPO;

  beforeEach(() => {
    queryDialogSk = newInstance();
    queryDialogSkPO = new QueryDialogSkPO(queryDialogSk);
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
      await queryDialogSkPO.clickCancelBtn();
      await events;
    });

    it('should emit "query-dialog-close" and "edit" when closed via the "Show Matches" button',
        async () => {
      queryDialogSk.open({}, '');
      const events = Promise.all([eventPromise('query-dialog-close'), eventPromise('edit')]);
      await queryDialogSkPO.clickShowMatchesBtn();
      await events;
    });
  })

  describe('opened with an empty selection', () => {
    const paramSet: ParamSet = {'a': ['1', '2', '3'], 'b': ['4', '5'], 'c': ['6']};

    beforeEach(() => {
      queryDialogSk.open(paramSet, /* selection= */ '');
    });

    it('should have an empty selection', async() => {
      // The query-sk component correctly shows the ParamSet.
      expect(await queryDialogSkPO.getParamSet()).to.deep.equal(paramSet);

      // But none of the ParamSet items are selected.
      expect(await queryDialogSkPO.getSelection()).to.deep.equal({});

      // The "empty selection" placeholder text is visible instead of the paramset-sk component.
      expect(await queryDialogSkPO.isEmptySelectionMessageVisible()).to.be.true;
      expect(await queryDialogSkPO.isParamSetSkVisible()).to.be.false;
    });

    it('should update paramset-sk when selection changes', async () => {
      await queryDialogSkPO.setSelection({'a': ['1']});
      expect(await queryDialogSkPO.getParamSetSkContents()).to.deep.equal({'a': ['1']});

      // The placeholder text should not be visible. It suffices to assert this just once.
      expect(await queryDialogSkPO.isEmptySelectionMessageVisible()).to.be.false;

      await queryDialogSkPO.setSelection({'a': ['1', '2']});
      expect(await queryDialogSkPO.getParamSetSkContents()).to.deep.equal({'a': ['1', '2']});

      await queryDialogSkPO.setSelection({'a': ['1', '2'], 'b': ['4']});
      expect(await queryDialogSkPO.getParamSetSkContents())
        .to.deep.equal({'a': ['1', '2'], 'b': ['4']});
    });

    it('should emit event "edit" containing the current selection when "Show Matches" is clicked',
        async () => {
      await queryDialogSkPO.setSelection({'a': ['1', '2'], 'b': ['4']});

      // Click "Show Matches" button and catch the "edit" event.
      const event = eventPromise<CustomEvent<string>>('edit');
      await queryDialogSkPO.clickShowMatchesBtn();
      const eventSelection = (await event).detail;

      // The event contents should match the selection.
      expect(eventSelection).to.equal('a=1&a=2&b=4');
    });

    it('should clear the previous selection when reopened with an empty selection', async () => {
      // Select a=1
      await queryDialogSkPO.setSelection({'a': ['1']});

      // It should have selected a=1.
      expect(await queryDialogSkPO.getParamSetSkContents()).to.deep.equal({'a': ['1']});

      // Close dialog.
      await queryDialogSkPO.clickCancelBtn();

      // Reopen with same ParamSet and empty selection.
      queryDialogSk.open(paramSet, /* selection= */ '');

      // Selection should be empty.
      expect(await queryDialogSkPO.getSelection()).to.deep.equal({});
      expect(await queryDialogSkPO.isEmptySelectionMessageVisible()).to.be.true;
      expect(await queryDialogSkPO.isParamSetSkVisible()).to.be.false;
    });
  })

  describe('opened with a non-empty selection', () => {
    const paramSet: ParamSet = {'a': ['1', '2', '3'], 'b': ['4', '5'], 'c': ['6']};
    const selection: ParamSet = {'a': ['1', '2'], 'b': ['4']};

    beforeEach(() => {
      queryDialogSk.open(paramSet, fromParamSet(selection));
    });

    it('shows the passed in selection', async () => {
      // Both query-sk and paramset-sk show the passed in selection.
      expect(await queryDialogSkPO.getSelection()).to.deep.equal(selection);
      expect(await queryDialogSkPO.getParamSetSkContents()).to.deep.equal(selection);

      // The "empty selection" placeholder text is not visible.
      expect(await queryDialogSkPO.isEmptySelectionMessageVisible()).to.be.false;
    });

    it('can be reopened with a different selection', async () => {
      const differentSelection: ParamSet = {'a': ['2', '3'], 'c': ['6']};

      // Close dialog and reopen it with a different selection.
      await queryDialogSkPO.clickCancelBtn();
      queryDialogSk.open(paramSet, fromParamSet(differentSelection));

      // Both query-sk and paramset-sk show the passed in selection.
      expect(await queryDialogSkPO.getSelection()).to.deep.equal(differentSelection);
      expect(await queryDialogSkPO.getParamSetSkContents()).to.deep.equal(differentSelection);
    });
  });

  describe('reopened with a different ParamSet', async () => {
    const paramSet: ParamSet = {'a': ['1', '2', '3'], 'b': ['4', '5'], 'c': ['6']};
    const selection: ParamSet = {'a': ['3'], 'b': ['4']};

    const differentParamSet: ParamSet = {'a': ['3', '4', '5'], 'b': ['6'], 'z': ['7']};
    const differentSelection: ParamSet = {'a': ['3', '4'], 'b': ['6']};

    beforeEach(() => {
      queryDialogSk.open(paramSet, fromParamSet(selection));
    })

    it('can be reopened with a different ParamSet and an empty selection', async () => {
      // Close dialog and reopen it with a different ParamSet.
      await queryDialogSkPO.clickCancelBtn();
      queryDialogSk.open(differentParamSet, /* selection= */ '');

      // The query-sk component shows the new ParamSet, and the selection is empty.
      expect(await queryDialogSkPO.getParamSet()).to.deep.equal(differentParamSet);
      expect(await queryDialogSkPO.getSelection()).to.deep.equal({});

      // The "empty selection" placeholder text is visible instead of the paramset-sk component.
      expect(await queryDialogSkPO.isEmptySelectionMessageVisible()).to.be.true;
      expect(await queryDialogSkPO.isParamSetSkVisible()).to.be.false;
    });

    it('can be reopened with a different ParamSet and a non-empty selection', async () => {
      // Close dialog and reopen it with a different ParamSet and a non-empty selection.
      await queryDialogSkPO.clickCancelBtn();
      queryDialogSk.open(differentParamSet, fromParamSet(differentSelection));

      // Both query-sk and paramset-sk show the passed in selection.
      expect(await queryDialogSkPO.getSelection()).to.deep.equal(differentSelection);
      expect(await queryDialogSkPO.getParamSetSkContents()).to.deep.equal(differentSelection);

      // The placeholder text should not be visible.
      expect(await queryDialogSkPO.isEmptySelectionMessageVisible()).to.be.false;
    });
  });

  it('rationalizes an invalid selection', async () => {
    const paramSet: ParamSet = {'a': ['1', '2', '3'], 'b': ['4', '5'], 'c': ['6']};

    // This contains the invalid value "a=4" and a value for the invalid key "d".
    const invalidSelection: ParamSet = {'a': ['2', '3', '4'], 'b': ['5'], 'd': ['7']};

    // This is the invalidSelection with the invalid key/value pairs removed.
    const rationalizedSelection: ParamSet = {'a': ['2', '3'], 'b': ['5']};

    // Open dialog with invalid selection.
    queryDialogSk.open(paramSet, fromParamSet(invalidSelection));

    // The dialog should rationalize the invalid selection.
    expect(await queryDialogSkPO.getSelection()).to.deep.equal(rationalizedSelection);
    expect(await queryDialogSkPO.getParamSetSkContents()).to.deep.equal(rationalizedSelection);
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
});
