import './index';
import { expect } from 'chai';
import fetchMock from 'fetch-mock';
import { $$ } from '../../../infra-sk/modules/dom';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { PerfStatusSk } from './perf-status-sk';
import { AlertsStatus } from '../../../perf/modules/json';

describe('perf-status-sk', () => {
  const newInstance = setUpElementUnderTest<PerfStatusSk>('perf-status-sk');

  let element: PerfStatusSk;
  beforeEach(async () => {
    fetchMock.getOnce('https://skia-perf.luci.app/_/alerts/', <AlertsStatus>{
      alerts: 5,
    });
    element = newInstance();
    await fetchMock.flush(true);
  });

  describe('displays', () => {
    it('perf regressions', () => {
      expect($$('.value', element)).to.have.property('innerText', '5');
    });
  });
});
