import './index';
import fetchMock from 'fetch-mock';
import { expect } from 'chai';
import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { BulkTriageSk } from './bulk-triage-sk';
import { BulkTriageSkPO } from './bulk-triage-sk_po';
import {
  examplePageData, exampleAllData, expectedPageDataTriageRequest, expectedAllDataTriageRequest,
} from './test_data';

describe('bulk-triage-sk', () => {
  const newInstance = setUpElementUnderTest<BulkTriageSk>('bulk-triage-sk');

  let bulkTriageSk: BulkTriageSk;
  let bulkTriageSkPO: BulkTriageSkPO;

  beforeEach(() => {
    bulkTriageSk = newInstance();
    bulkTriageSk.currentPageDigests = examplePageData;
    bulkTriageSk.allDigests = exampleAllData;
    bulkTriageSkPO = new BulkTriageSkPO(bulkTriageSk);
  });

  it('shows the correct digest counts', async () => {
    expect(await bulkTriageSkPO.getTriageBtnLabel()).to.equal('Triage 3 digests as closest');
    expect(await bulkTriageSkPO.getTriageAllCheckboxLabel()).to.equal('Triage all 6 digests');
  });

  it('does not show a changelist ID by default', async () => {
    expect(await bulkTriageSkPO.isAffectedChangelistIdVisible()).to.be.false;
  });

  it('show the changelist ID when provided', async () => {
    bulkTriageSk.changeListID = '123';
    expect(await bulkTriageSkPO.isAffectedChangelistIdVisible()).to.be.true;
    expect(await bulkTriageSkPO.getAffectedChangelistId()).to.equal('This affects Changelist 123.');
  });

  it('defaults to bulk-triaging to closest', async () => {
    expect(await bulkTriageSkPO.isClosestBtnSelected()).to.be.true;
    expect(bulkTriageSk.value).to.equal('closest');
  });

  it('has value respond to button clicks', async () => {
    await bulkTriageSkPO.clickUntriagedBtn();
    expect(await bulkTriageSkPO.isUntriagedBtnSelected()).to.be.true;
    expect(bulkTriageSk.value).to.equal('untriaged');

    await bulkTriageSkPO.clickPositiveBtn();
    expect(await bulkTriageSkPO.isPositiveBtnSelected()).to.be.true;
    expect(bulkTriageSk.value).to.equal('positive');

    await bulkTriageSkPO.clickNegativeBtn();
    expect(await bulkTriageSkPO.isNegativeBtnSelected()).to.be.true;
    expect(bulkTriageSk.value).to.equal('negative');

    await bulkTriageSkPO.clickClosestBtn();
    expect(await bulkTriageSkPO.isClosestBtnSelected()).to.be.true;
    expect(bulkTriageSk.value).to.equal('closest');
  });

  it('emits a bulk_triage_cancelled event when the cancel button is clicked', async () => {
    const cancelEvent = eventPromise('bulk_triage_cancelled', 100);
    await bulkTriageSkPO.clickCancelBtn();
    await cancelEvent;
  });

  describe('RPC requests v1', () => {
    afterEach(() => {
      expect(fetchMock.done()).to.be.true; // All mock RPCs called at least once.
      fetchMock.reset();
    });

    it('POSTs for just this page of results', async () => {
      fetchMock.post('/json/v1/triage', 200, { body: expectedPageDataTriageRequest });

      const finishedPromise = eventPromise('bulk_triage_finished');
      await bulkTriageSkPO.clickTriageBtn();
      await finishedPromise;
    });

    it('POSTs for all results', async () => {
      bulkTriageSk.changeListID = 'someCL';
      bulkTriageSk.crs = 'gerrit';

      fetchMock.post('/json/v1/triage', 200, { body: expectedAllDataTriageRequest });

      await bulkTriageSkPO.clickTriageAllCheckbox();

      const finishedPromise = eventPromise('bulk_triage_finished');
      await bulkTriageSkPO.clickTriageBtn();
      await finishedPromise;
    });
  });

  describe('RPC requests v2', () => {
    afterEach(() => {
      expect(fetchMock.done()).to.be.true; // All mock RPCs called at least once.
      fetchMock.reset();
    });

    it('POSTs for just this page of results', async () => {
      fetchMock.post('/json/v2/triage', 200, { body: expectedPageDataTriageRequest });

      const finishedPromise = eventPromise('bulk_triage_finished');
      bulkTriageSk.useNewAPI = true;
      await bulkTriageSkPO.clickTriageBtn();
      await finishedPromise;
    });

    it('POSTs for all results', async () => {
      bulkTriageSk.changeListID = 'someCL';
      bulkTriageSk.crs = 'gerrit';

      fetchMock.post('/json/v2/triage', 200, { body: expectedAllDataTriageRequest });

      await bulkTriageSkPO.clickTriageAllCheckbox();

      const finishedPromise = eventPromise('bulk_triage_finished');
      bulkTriageSk.useNewAPI = true;
      await bulkTriageSkPO.clickTriageBtn();
      await finishedPromise;
    });
  });
});
