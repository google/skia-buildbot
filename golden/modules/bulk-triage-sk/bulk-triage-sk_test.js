import './index';
import { fetchMock } from 'fetch-mock';
import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import {
  examplePageData, exampleAllData, expectedPageData, expectedAllData,
} from './test_data';
import { $$ } from 'common-sk/modules/dom';

describe('bulk-triage-sk', () => {
  const newInstance = setUpElementUnderTest('bulk-triage-sk');

  let bulkTriageSk;
  beforeEach(() => {
    bulkTriageSk = newInstance();
    bulkTriageSk.setDigests(examplePageData, exampleAllData);
  });

  it('defaults to bulk-triaging to closest', () => {
    expectValueAndToggledButtonToBe(bulkTriageSk, 'closest');
  });

  it('has value respond to button clicks', () => {
    $$('button.untriaged', bulkTriageSk).click();
    expectValueAndToggledButtonToBe(bulkTriageSk, 'untriaged');
    $$('button.positive', bulkTriageSk).click();
    expectValueAndToggledButtonToBe(bulkTriageSk, 'positive');
    $$('button.negative', bulkTriageSk).click();
    expectValueAndToggledButtonToBe(bulkTriageSk, 'negative');
    $$('button.closest', bulkTriageSk).click();
    expectValueAndToggledButtonToBe(bulkTriageSk, 'closest');
  });

  it('emits a bulk_triage_cancelled event when the cancel button is clicked', async () => {
    const cancelEvent = eventPromise('bulk_triage_cancelled', 100);
    $$('button.cancel', bulkTriageSk).click();
    await cancelEvent;
  });

  describe('RPC requests', () => {
    afterEach(() => {
      expect(fetchMock.done()).to.be.true; // All mock RPCs called at least once.
      fetchMock.reset();
    });

    it('POSTs for just this page of results', async () => {
      const finishedPromise = eventPromise('bulk_triage_finished');
      fetchMock.post('/json/triage', (url, req) => {
        expect(req.body).to.equal(expectedPageData);
        return 200;
      });

      $$('button.triage', bulkTriageSk).click();
      await finishedPromise;
    });

    it('POSTs for all results', async () => {
      bulkTriageSk.changeListID = 'someCL';
      bulkTriageSk.crs = 'gerrit';
      $$('checkbox-sk.toggle_all', bulkTriageSk).click();
      const finishedPromise = eventPromise('bulk_triage_finished');
      fetchMock.post('/json/triage', (url, req) => {
        expect(req.body).to.equal(expectedAllData);
        return 200;
      });

      $$('button.triage', bulkTriageSk).click();
      await finishedPromise;
    });
  });
});

const expectValueAndToggledButtonToBe = (bulkTriageSk, value) => {
  expect(bulkTriageSk.value).to.equal(value);
  expect($$(`button.${value}`, bulkTriageSk).className).to.contain('selected');
};
