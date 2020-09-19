import './index';
import fetchMock from 'fetch-mock';
import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { BulkTriageSk } from './bulk-triage-sk';
import { BulkTriageSkPO } from './bulk-triage-sk_po';
import { examplePageData, exampleAllData, expectedPageData, expectedAllData } from './test_data';
import { expect } from 'chai';

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

  describe('RPC requests', () => {
    afterEach(() => {
      expect(fetchMock.done()).to.be.true; // All mock RPCs called at least once.
      fetchMock.reset();
    });

    it('POSTs for just this page of results', async () => {
      const finishedPromise = eventPromise('bulk_triage_finished');
      fetchMock.post('/json/v1/triage', (url, req) => {
        expect(req.body).to.equal(expectedPageData);
        return 200;
      });

      await bulkTriageSkPO.clickTriageBtn();
      await finishedPromise;
    });

    it('POSTs for all results', async () => {
      bulkTriageSk.changeListID = 'someCL';
      bulkTriageSk.crs = 'gerrit';
      await bulkTriageSkPO.clickToggleAllCheckbox();
      const finishedPromise = eventPromise('bulk_triage_finished');
      fetchMock.post('/json/v1/triage', (url, req) => {
        expect(req.body).to.equal(expectedAllData);
        return 200;
      });

      bulkTriageSkPO.clickTriageBtn();
      await finishedPromise;
    });
  });
});
