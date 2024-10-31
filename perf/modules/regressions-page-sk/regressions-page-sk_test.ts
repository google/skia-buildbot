import './index';
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { RegressionsPageSk } from './regressions-page-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

import { GetSheriffListResponse } from '../json';

describe('regressions-page-sk', () => {
  const sheriffListResponse: GetSheriffListResponse = {
    sheriff_list: ['Sheriff Config 1', 'Sheriff Config 2', 'Sheriff Config 3'],
    error: '',
  };
  fetchMock.get('/_/anomalies/sheriff_list', { body: sheriffListResponse });

  fetchMock.get(`/_/regressions?sub_name=Sheriff%20Config%202&limit=10&offset=0`, () => [
    {
      id: 'id2',
      commit_number: 1235,
      prev_commit_number: 1237,
      alert_id: 1,
      creation_time: '',
      median_before: 123,
      median_after: 135,
      is_improvement: true,
      cluster_type: 'high',
      frame: {
        dataframe: {
          paramset: {
            bot: ['bot1'],
            benchmark: ['benchmark1'],
            test: ['test1'],
            improvement_direction: ['up'],
          },
          traceset: {},
          header: null,
          skip: 1,
        },
        skps: [1],
        msg: '',
        anomalymap: null,
      },
      high: {
        centroid: null,
        shortcut: 'shortcut 1',
        param_summaries2: null,
        step_fit: {
          status: 'High',
          least_squares: 123,
          regression: 12,
          step_size: 345,
          turning_point: 1234,
        },
        step_point: null,
        num: 156,
        ts: 'test',
      },
    },
  ]);

  describe('RegressionsPageSK', () => {
    const newInstance = setUpElementUnderTest<RegressionsPageSk>('regressions-page-sk');

    const element = newInstance((_el: RegressionsPageSk) => {});

    it('Loads associated regressions when subscription selected', async () => {
      const dropdown = document.getElementById('filter') as HTMLSelectElement;
      // 3 loaded configs and the default options
      assert.equal(dropdown?.options.length, 4);
      assert.equal(element.regressions.length, 0);

      await element.filterChange('Sheriff Config 2');
      assert.equal(element.regressions.length, 1);
    });
  });
});
