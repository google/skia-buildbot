import './index';
import { eventPromise, noEventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { ParamSet } from 'common-sk/modules/query';
import { TraceFilterSk } from './trace-filter-sk';
import { TraceFilterSkPO } from './trace-filter-sk_po';

const expect = chai.expect;

const paramSet: ParamSet = {
  'car make': ['chevrolet', 'dodge', 'ford', 'lincoln motor company'],
  'color': ['blue', 'green', 'red'],
  'used': ['yes', 'no'],
  'year': ['2020', '2019', '2018', '2017', '2016', '2015']
};

const selection: ParamSet = {'car make': ['dodge', 'ford'], 'color': ['blue']};
const differentSelection: ParamSet = {'color': ['green'], 'used': ['yes', 'no']};

describe('trace-filter-sk', () => {
  const newInstance = setUpElementUnderTest<TraceFilterSk>('trace-filter-sk');

  let traceFilterSk: TraceFilterSk;
  let traceFilterSkPO: TraceFilterSkPO;

  beforeEach(() => {
    traceFilterSk = newInstance();
    traceFilterSk.paramSet = paramSet;

    traceFilterSkPO = new TraceFilterSkPO(traceFilterSk);
  });

  it('opens the query dialog with the given ParamSet when clicking the "edit query" button',
      async () => {
    await traceFilterSkPO.clickEditBtn();
    expect(await traceFilterSkPO.isQueryDialogSkOpen()).to.be.true;
  });

  describe('empty selection', () => {
    it('shows empty selection message', async () => {
      expect(await traceFilterSkPO.isEmptyFilterMessageVisible()).to.be.true;
      expect(await traceFilterSkPO.isParamSetSkVisible()).to.be.false;
      expect(await traceFilterSkPO.getSelection()).to.deep.equal({});
    });

    it('query dialog shows an empty selection', async () => {
      await traceFilterSkPO.clickEditBtn();
      expect(await traceFilterSkPO.getQueryDialogSkSelection()).to.deep.equal({});
    });
  });

  describe('non-empty selection', () => {
    beforeEach(() => {
      traceFilterSk.selection = selection;
    });

    it('shows the current selection', async () => {
      expect(await traceFilterSkPO.isEmptyFilterMessageVisible()).to.be.false;
      expect(await traceFilterSkPO.getParamSetSkContents()).to.deep.equal(selection);
      expect(await traceFilterSkPO.getSelection()).to.deep.equal(selection);
    });

    it('shows the current selection in the query dialog', async () => {
      await traceFilterSkPO.clickEditBtn();
      expect(await traceFilterSkPO.getQueryDialogSkSelection()).to.deep.equal(selection);
    });
  });

  describe('applying changes via the query dialog', () => {
    beforeEach(() => {
      traceFilterSk.selection = selection;
    });

    it('updates the selection', async () => {
      await traceFilterSkPO.clickEditBtn();
      await traceFilterSkPO.setQueryDialogSkSelection(differentSelection);
      await traceFilterSkPO.clickQueryDialogSkShowMatchesBtn();

      expect(traceFilterSk.selection).to.deep.equal(differentSelection);
      expect(await traceFilterSkPO.getParamSetSkContents()).to.deep.equal(differentSelection);
    });

    it('emits event "trace-filter-sk-change" with the new selection', async () => {
      await traceFilterSkPO.clickEditBtn();
      await traceFilterSkPO.setQueryDialogSkSelection(differentSelection);

      const event = eventPromise<CustomEvent<ParamSet>>('trace-filter-sk-change');
      await traceFilterSkPO.clickQueryDialogSkShowMatchesBtn();
      expect(((await event) as CustomEvent<ParamSet>).detail).to.deep.equal(differentSelection);
    });
  });

  describe('dismissing the query dialog after making changes', () => {
    beforeEach(() => {
      traceFilterSk.selection = selection;
    });

    it('leaves the current selection intact', async () => {
      await traceFilterSkPO.clickEditBtn();
      await traceFilterSkPO.setQueryDialogSkSelection(differentSelection);
      await traceFilterSkPO.clickQueryDialogSkCancelBtn();

      expect(traceFilterSk.selection).to.deep.equal(selection);
      expect(await traceFilterSkPO.getParamSetSkContents()).to.deep.equal(selection);
    });

    it('does not emit the "trace-filter-sk-change" event', async () => {
      await traceFilterSkPO.clickEditBtn();
      await traceFilterSkPO.setQueryDialogSkSelection(differentSelection);

      const noEvent = noEventPromise('trace-filter-sk-change');
      await traceFilterSkPO.clickQueryDialogSkCancelBtn();
      await noEvent;
    });
  })
});
