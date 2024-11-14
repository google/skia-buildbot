import './index';
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { RegressionsPageSk } from './regressions-page-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

import { GetSheriffListResponse, GetAnomaliesResponse } from '../json';

describe('regressions-page-sk', () => {
  const sheriffListResponse: GetSheriffListResponse = {
    sheriff_list: ['Sheriff Config 1', 'Sheriff Config 2', 'Sheriff Config 3'],
    error: '',
  };
  fetchMock.get('/_/anomalies/sheriff_list', { body: sheriffListResponse });

  const anomalyListResponse: GetAnomaliesResponse = {
    anomaly_list: [
      {
        id: 123,
        test_path: 'mm/bb/kk/tt',
        bug_id: 789,
        start_revision: 1235,
        end_revision: 1237,
        median_before_anomaly: 123,
        median_after_anomaly: 135,
        is_improvement: true,
        recovered: true,
        state: '',
        statistic: 'max',
        units: 'ms',
        degrees_of_freedom: 1.0,
        p_value: 0.1,
        segment_size_before: 6,
        segment_size_after: 16,
        std_dev_before_anomaly: 0.2,
        t_statistic: 3.3,
        subscription_name: 'V8 Perf Sheriff',
        bug_component: 'v8',
        bug_labels: [],
        bug_cc_emails: [],
      },
    ],
    anomaly_cursor: '',
    error: '',
  };

  const anomalyListResponseWithAnomalyCursor: GetAnomaliesResponse = {
    anomaly_list: [
      {
        id: 123,
        test_path: 'mm/bb/kk/tt',
        bug_id: 789,
        start_revision: 1235,
        end_revision: 1237,
        median_before_anomaly: 123,
        median_after_anomaly: 135,
        is_improvement: true,
        recovered: true,
        state: '',
        statistic: 'max',
        units: 'ms',
        degrees_of_freedom: 1.0,
        p_value: 0.1,
        segment_size_before: 6,
        segment_size_after: 16,
        std_dev_before_anomaly: 0.2,
        t_statistic: 3.3,
        subscription_name: 'V8 Perf Sheriff',
        bug_component: 'v8',
        bug_labels: [],
        bug_cc_emails: [],
      },
    ],
    anomaly_cursor: 'query_cursor',
    error: '',
  };

  fetchMock.get(
    '/_/anomalies/anomaly_list?sheriff=Sheriff%20Config%202&anomaly_cursor=query_cursor',
    {
      body: anomalyListResponseWithAnomalyCursor,
    }
  );
  fetchMock.get('/_/anomalies/anomaly_list?sheriff=Sheriff%20Config%202', {
    body: anomalyListResponse,
  });
  fetchMock.get('/_/anomalies/anomaly_list', {
    body: anomalyListResponse,
  });
  fetchMock.get('/_/anomalies/anomaly_list?triaged=true', {
    body: anomalyListResponse,
  });
  fetchMock.get('/_/anomalies/anomaly_list?improvements=true', {
    body: anomalyListResponse,
  });

  describe('RegressionsPageSK', () => {
    const newInstance = setUpElementUnderTest<RegressionsPageSk>('regressions-page-sk');

    const element = newInstance((_el: RegressionsPageSk) => {});

    const dropdown = document.getElementById('filter') as HTMLSelectElement;

    it('Loads associated regressions when subscription selected', async () => {
      // 3 loaded configs and the default options
      assert.equal(dropdown?.options.length, 4);
      // /anomaly_list is not called without a sheriff selected.
      assert.equal(fetchMock.lastCall()![0], '/_/anomalies/sheriff_list');

      await element.triagedChange();
      assert.equal(fetchMock.lastCall()![0], '/_/anomalies/anomaly_list?triaged=true');
      await element.triagedChange();
      assert.equal(fetchMock.lastCall()![0], '/_/anomalies/anomaly_list');

      await element.improvementChange();
      assert.equal(fetchMock.lastCall()![0], '/_/anomalies/anomaly_list?improvements=true');
      await element.improvementChange();
      assert.equal(fetchMock.lastCall()![0], '/_/anomalies/anomaly_list');

      await element.filterChange('Sheriff Config 2');
      assert.equal(element.cpAnomalies.length, 1);
      assert.equal(
        fetchMock.lastCall()![0],
        '/_/anomalies/anomaly_list?sheriff=Sheriff%20Config%202'
      );
    });
  });

  describe('RegressionsPageSK', () => {
    fetchMock.config.overwriteRoutes = true;
    const newInstance = setUpElementUnderTest<RegressionsPageSk>('regressions-page-sk');

    const element = newInstance((_el: RegressionsPageSk) => {});

    const dropdown = document.getElementById('filter') as HTMLSelectElement;

    it('Loads anomaly_cursor when the anomaly_cursor is returned in the response', async () => {
      // 3 loaded configs and the default options
      assert.equal(dropdown?.options.length, 4);

      await element.filterChange('Sheriff Config 2');
      fetchMock.getOnce(
        '/_/anomalies/anomaly_list?sheriff=Sheriff%20Config%202',
        anomalyListResponseWithAnomalyCursor
      );

      await element.fetchRegressions();
      assert.equal(element.cpAnomalies.length, 2);
      assert.equal(
        fetchMock.lastCall()![0],
        '/_/anomalies/anomaly_list?sheriff=Sheriff%20Config%202'
      );
      assert.equal(element.anomalyCursor, 'query_cursor');

      await element.fetchRegressions();
      assert.equal(
        fetchMock.lastCall()![0],
        '/_/anomalies/anomaly_list?sheriff=Sheriff%20Config%202&anomaly_cursor=query_cursor'
      );
      assert.equal(element.anomalyCursor, 'query_cursor');
      assert.equal(element.showMoreAnomalies, true);
    });
  });
});
