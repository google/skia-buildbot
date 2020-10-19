import './index';
import { PerfStatusSk } from './perf-status-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';
import fetchMock from 'fetch-mock';
import { $$ } from 'common-sk/modules/dom';

describe('perf-status-sk', () => {
  const newInstance = setUpElementUnderTest<PerfStatusSk>('perf-status-sk');

  let element: PerfStatusSk;
  beforeEach(async () => {
    fetchMock.getOnce('https://perf.skia.org/_/alerts/', { alerts: 5 });
    element = newInstance();
    await fetchMock.flush(true);
  });

  describe('some displays', () => {
    it('perf regressions', () => {
      expect($$('.value', element)).to.have.property('innerText', '5');
    });
  });
});
