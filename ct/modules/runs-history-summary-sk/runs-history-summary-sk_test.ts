import './index';

import { $, $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';

import {
  summaryResults3, summaryResults5, summaryResults15, summaryResults33,
} from './test_data';
import {
  setUpElementUnderTest,
} from '../../../infra-sk/modules/test_util';

describe('runs-history-summary-sk', () => {
  const newInstance = setUpElementUnderTest('runs-history-summary-sk');
  fetchMock.config.overwriteRoutes = false;

  let summary;
  beforeEach(async () => {
    fetchMock.post('begin:/_/completed_tasks', () => {
      // Cheat so we don't have to compute timestamps to determine the period
      // on this fake backend.
      switch ($$('runs-history-summary-sk').period) {
        case 7:
          return summaryResults3;
        case 30:
          return summaryResults5;
        case 365:
          return summaryResults15;
        default:
          return summaryResults33;
      }
    });
    summary = newInstance();
    await fetchMock.flush(true);
  });

  afterEach(() => {
    //  Check all mock fetches called at least once and reset.
    expect(fetchMock.done()).to.be.true;
    fetchMock.reset();
  });

  // Take index of desired button [week, month, year, ever].
  const clickTimePeriodButton = async (i) => {
    $('button', summary)[i].click();
    await fetchMock.flush(true);
  };

  it('shows summary entries', async () => {
    expect($('td', summary).length).to.equal(3 * 4);
  });

  it('summary updates with time period selection', async () => {
    await clickTimePeriodButton(2);
    expect($('td', summary).length).to.equal(15 * 4);
    await clickTimePeriodButton(3);
    expect($('td', summary).length).to.equal(33 * 4);
  });
});
