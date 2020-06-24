import './index';
import { $, $$ } from 'common-sk/modules/dom';
import { eventPromise, noEventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { ParamSet } from 'common-sk/modules/query';
import { TraceFilterSk } from './trace-filter-sk';

const expect = chai.expect;

describe('trace-filter-sk', () => {
  const newInstance = setUpElementUnderTest<TraceFilterSk>('trace-filter-sk');

  let traceFilterSk: TraceFilterSk;

  beforeEach(() => {
    traceFilterSk = newInstance();
    traceFilterSk.paramSet = {
      'car make': ['chevrolet', 'dodge', 'ford', 'lincoln motor company'],
      'color': ['blue', 'green', 'red'],
      'used': ['yes', 'no'],
      'year': ['2020', '2019', '2018', '2017', '2016', '2015']
    };
  });

  it('clicking the "edit query" button opens the query dialog', () => {
    clickEditBtn();
    expect(isQueryDialogOpen()).to.be.true;
  });

  describe('empty selection', () => {
    it('shows empty selection message', () => {
      expect(isEmptySelectionMessageVisible()).to.be.true;
      expect(isSelectionVisible()).to.be.false;
    });

    it('query dialog shows an empty selection', () => {
      clickEditBtn();
      expect(getQueryDialogSelection()).to.deep.equal({});
    });
  });

  describe('non-empty selection', () => {
    const selection: ParamSet = {'car make': ['dodge', 'ford'], 'color': ['blue']};

    beforeEach(() => { traceFilterSk.selection = selection; });

    it('shows the current selection', () => {
      expect(isEmptySelectionMessageVisible()).to.be.false;
      expect(getSelection()).to.deep.equal(selection);
    });

    it('query dialog shows the current selection', () => {
      clickEditBtn();
      expect(getSelection()).to.deep.equal(selection);
    });
  });

  describe('applying changes via the query dialog', () => {
    const oldSelection: ParamSet = {'car make': ['dodge', 'ford'], 'color': ['blue']};

    beforeEach(() => { traceFilterSk.selection = oldSelection; });

    it('updates the selection', () => {
      clickEditBtn();
      const newSelection = changeQueryDialogSelection();
      clickQueryDialogSubmitBtn();

      expect(traceFilterSk.selection).to.deep.equal(newSelection);
      expect(getSelection()).to.deep.equal(newSelection);
    });

    it('emits event "trace-filter-sk-change" with the new selection', async () => {
      clickEditBtn();
      const newSelection = changeQueryDialogSelection();

      const event = eventPromise<CustomEvent<ParamSet>>('trace-filter-sk-change');
      clickQueryDialogSubmitBtn();
      expect(((await event) as CustomEvent<ParamSet>).detail).to.deep.equal(newSelection);
    });
  });

  describe('dismissing the query dialog after making changes', () => {
    const selection: ParamSet = {'car make': ['dodge', 'ford'], 'color': ['blue']};

    beforeEach(() => { traceFilterSk.selection = selection; });

    it('leaves the current selection intact', () => {
      clickEditBtn();
      changeQueryDialogSelection();
      clickQueryDialogCancelBtn();

      expect(traceFilterSk.selection).to.deep.equal(selection);
      expect(getSelection()).to.deep.equal(selection);
    });

    it('does not emit the "trace-filter-sk-change" event', async () => {
      clickEditBtn();
      changeQueryDialogSelection();

      const noEvent = noEventPromise('trace-filter-sk-change');
      clickQueryDialogCancelBtn();
      await noEvent;
    });
  })

  const clickEditBtn = () => $$<HTMLButtonElement>('.edit-query')!.click();

  const isEmptySelectionMessageVisible = () => $$('.empty-placeholder', traceFilterSk) !== null;

  const isSelectionVisible = () => $$('.selection paramset-sk', traceFilterSk) !== null;

  const getSelection = () => getParamSetContents('.selection');

  const getQueryDialogSelection = () => getParamSetContents('query-dialog-sk');

  const getParamSetContents = (containerSelector: string): ParamSet => {
    const paramSet: ParamSet = {};
    $(`${containerSelector} paramset-sk tr`, traceFilterSk).forEach((tr, i) => {
      if (i === 0) return; // Skip the first row, which usually displays titles (empty in our case).
      const key = $$('th', tr)!.textContent!;
      const values = $('div', tr).map(div => div.textContent!);
      paramSet[key] = values;
    })
    return paramSet;
  };

  const isQueryDialogOpen = () => $$<HTMLDialogElement>('dialog', traceFilterSk)!.open;

  const changeQueryDialogSelection = (): ParamSet => {
    const queryDialogSk = $$('query-dialog-sk', traceFilterSk)!;
    $$<HTMLButtonElement>('.clear_selections', queryDialogSk)!.click();
    $$<HTMLDivElement>('select-sk div:nth-child(2)', queryDialogSk)!.click(); // Color.
    $$<HTMLDivElement>('multi-select-sk div:nth-child(2)', queryDialogSk)!.click(); // Green.
    $$<HTMLDivElement>('select-sk div:nth-child(3)', queryDialogSk)!.click(); // Used.
    $$<HTMLDivElement>('multi-select-sk div:nth-child(1)', queryDialogSk)!.click(); // Yes.
    $$<HTMLDivElement>('multi-select-sk div:nth-child(2)', queryDialogSk)!.click(); // No.
    return {'color': ['green'], 'used': ['yes', 'no']};
  };

  const clickQueryDialogSubmitBtn =
    () => $$<HTMLButtonElement>('query-dialog-sk .show-matches', traceFilterSk)!.click();

  const clickQueryDialogCancelBtn = () => null;
    () => $$<HTMLButtonElement>('query-dialog-sk .cancel', traceFilterSk)!.click();
});
