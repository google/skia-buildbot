import './index';
import fetchMock from 'fetch-mock';
import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { BulkTriageSk } from './bulk-triage-sk';
import { examplePageData, exampleAllData, expectedPageData, expectedAllData } from './test_data';
import { $$ } from 'common-sk/modules/dom';
import { expect } from 'chai';
import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk';

describe('bulk-triage-sk', () => {
  const newInstance = setUpElementUnderTest<BulkTriageSk>('bulk-triage-sk');

  let bulkTriageSk: BulkTriageSk;

  beforeEach(() => {
    bulkTriageSk = newInstance();
    bulkTriageSk.currentPageDigests = examplePageData;
    bulkTriageSk.allDigests = exampleAllData;
  });

  it('defaults to bulk-triaging to closest', () => {
    expectValueAndToggledButtonToBe(bulkTriageSk, 'closest');
  });

  it('has value respond to button clicks', () => {
    $$<HTMLButtonElement>('button.untriaged', bulkTriageSk)!.click();
    expectValueAndToggledButtonToBe(bulkTriageSk, 'untriaged');
    $$<HTMLButtonElement>('button.positive', bulkTriageSk)!.click();
    expectValueAndToggledButtonToBe(bulkTriageSk, 'positive');
    $$<HTMLButtonElement>('button.negative', bulkTriageSk)!.click();
    expectValueAndToggledButtonToBe(bulkTriageSk, 'negative');
    $$<HTMLButtonElement>('button.closest', bulkTriageSk)!.click();
    expectValueAndToggledButtonToBe(bulkTriageSk, 'closest');
  });

  it('emits a bulk_triage_cancelled event when the cancel button is clicked', async () => {
    const cancelEvent = eventPromise('bulk_triage_cancelled', 100);
    $$<HTMLButtonElement>('button.cancel', bulkTriageSk)!.click();
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

      $$<HTMLButtonElement>('button.triage', bulkTriageSk)!.click();
      await finishedPromise;
    });

    it('POSTs for all results', async () => {
      bulkTriageSk.changeListID = 'someCL';
      bulkTriageSk.crs = 'gerrit';
      $$<CheckOrRadio>('checkbox-sk.toggle_all', bulkTriageSk)!.click();
      const finishedPromise = eventPromise('bulk_triage_finished');
      fetchMock.post('/json/v1/triage', (url, req) => {
        expect(req.body).to.equal(expectedAllData);
        return 200;
      });

      $$<HTMLButtonElement>('button.triage', bulkTriageSk)!.click();
      await finishedPromise;
    });
  });
});

const expectValueAndToggledButtonToBe = (bulkTriageSk: BulkTriageSk, value: string) => {
  expect(bulkTriageSk.value).to.equal(value);
  expect($$<HTMLButtonElement>(`button.${value}`, bulkTriageSk)!.className).to.contain('selected');
};
