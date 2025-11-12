import './index';
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { RegressionsPageSk } from './regressions-page-sk';

import { setUpElementUnderTest, setQueryString } from '../../../infra-sk/modules/test_util';

import { GetSheriffListResponse, GetAnomaliesResponse } from '../json';

describe('regressions-page-sk', () => {
  window.perf = {
    instance_url: '',
    instance_name: 'chrome-perf-test',
    commit_range_url: 'http://example.com/range/{begin}/{end}',
    key_order: ['config'],
    demo: true,
    radius: 7,
    num_shift: 10,
    interesting: 25,
    step_up_only: false,
    display_group_by: true,
    hide_list_of_commits_on_explore: false,
    notifications: 'none',
    fetch_chrome_perf_anomalies: false,
    feedback_url: '',
    chat_url: '',
    help_url_override: '',
    trace_format: '',
    need_alert_action: false,
    bug_host_url: '',
    git_repo_url: '',
    keys_for_commit_range: [],
    keys_for_useful_links: [],
    skip_commit_detail_display: false,
    image_tag: 'fake-tag',
    remove_default_stat_value: false,
    enable_skia_bridge_aggregation: false,
    show_json_file_display: false,
    always_show_commit_info: false,
    show_triage_link: true,
    show_bisect_btn: true,
    app_version: 'test-version',
  };

  const sheriffListResponse: GetSheriffListResponse = {
    sheriff_list: ['Sheriff Config 1', 'Sheriff Config 2', 'Sheriff Config 3', 'Sheriff Config 4'],
    error: '',
  };

  const sheriffListResponseUnSorted: GetSheriffListResponse = {
    sheriff_list: [
      'Chrome Perf Sheriff 3',
      'Blink Config 3',
      'Angle Sheriff Perf ',
      'Angle Perf 1',
    ],
    error: '',
  };

  const sheriffListResponseSorted: string[] = [
    'Angle Perf 1',
    'Angle Sheriff Perf ',
    'Blink Config 3',
    'Chrome Perf Sheriff 3',
  ];

  const anomalyListResponse: GetAnomaliesResponse = {
    anomaly_list: [
      {
        id: '123',
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
        bisect_ids: [],
      },
    ],
    anomaly_cursor: '',
    error: '',
    alerts: [],
    subscription: null,
  };

  const anomalyListResponseWithAnomalyCursor: GetAnomaliesResponse = {
    anomaly_list: [
      {
        id: '123',
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
        bisect_ids: [],
      },
    ],
    anomaly_cursor: 'query_cursor',
    error: '',
    alerts: [],
    subscription: null,
  };

  const newInstance = setUpElementUnderTest<RegressionsPageSk>('regressions-page-sk');

  beforeEach(() => {
    setQueryString('');
    localStorage.clear();
    fetchMock.reset();

    fetchMock.get('/_/anomalies/sheriff_list', { body: sheriffListResponse });
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
  });

  describe('RegressionsPageSk - Filter Changes and API Calls', () => {
    let element: RegressionsPageSk;
    beforeEach(async () => {
      element = newInstance();
      await fetchMock.flush(true);
    });

    it('Loads associated regressions when subscription selected', async () => {
      const dropdown = element.querySelector('[id^="filter"]') as HTMLSelectElement;
      assert.isNotNull(dropdown);

      // 4 loaded configs and the default options
      assert.equal(dropdown.options.length, 5);
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

  describe('RegressionsPageSk - Anomaly Cursor Handling', () => {
    let element: RegressionsPageSk;
    beforeEach(async () => {
      element = newInstance();
      await fetchMock.flush(true);
    });

    it('Loads anomaly_cursor when the anomaly_cursor is returned in the response', async () => {
      const dropdown = element.querySelector('[id^="filter"]') as HTMLSelectElement;
      assert.isNotNull(dropdown);

      // 4 loaded configs and the default options
      assert.equal(dropdown.options.length, 5);

      await element.filterChange('Sheriff Config 2');
      fetchMock.getOnce(
        '/_/anomalies/anomaly_list?sheriff=Sheriff%20Config%202',
        anomalyListResponseWithAnomalyCursor,
        { overwriteRoutes: true }
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

  describe('RegressionsPageSK - Sorting', () => {
    it('Sheriff List is displayed in an rescending way', async () => {
      fetchMock.getOnce(
        '/_/anomalies/sheriff_list',
        { body: sheriffListResponseUnSorted },
        { overwriteRoutes: true }
      );
      const element = newInstance();
      await fetchMock.flush(true);

      const dropdown = element.querySelector('[id^="filter"]') as HTMLSelectElement;
      assert.isNotNull(dropdown);

      // 4 loaded configs and the default options
      assert.equal(dropdown.options.length, 5);
      element.subscriptionList.every((sheriff, index) => {
        assert.equal(sheriff, sheriffListResponseSorted.at(index));
      });
    });
  });

  describe('Selector Persistence', () => {
    const LAST_SELECTED_SHERIFF_KEY = 'perf-last-selected-sheriff';

    let originalPath: string;
    let element: RegressionsPageSk;

    beforeEach(async () => {
      // Store the original path to restore later
      originalPath = window.location.pathname + window.location.search;
      element = newInstance();
      await fetchMock.flush(true);
    });

    afterEach(() => {
      // Clean up: Restore the original URL
      window.history.pushState({}, '', originalPath);
    });

    it('should save selection to localStorage and restore it on new instances', async () => {
      // 1. Initial load.
      assert.strictEqual(element.state.selectedSubscription, '');
      assert.isNull(localStorage.getItem(LAST_SELECTED_SHERIFF_KEY));

      // 2. Simulate user selecting an option.
      await element.filterChange('Sheriff Config 2');
      await fetchMock.flush(true);

      // 3. Verify value is saved to localStorage.
      assert.strictEqual(localStorage.getItem(LAST_SELECTED_SHERIFF_KEY), 'Sheriff Config 2');
      assert.strictEqual(element.state.selectedSubscription, 'Sheriff Config 2');

      // 4. Simulate "Reloading Page" by creating a new instance.
      element = newInstance();
      await fetchMock.flush(true);

      // 5. On "reloaded" Page, check if the selector has the value from localStorage.
      assert.strictEqual(element.state.selectedSubscription, 'Sheriff Config 2');

      // Also check that the correct anomaly list was fetched.
      assert.equal(fetchMock.lastUrl(), '/_/anomalies/anomaly_list?sheriff=Sheriff%20Config%202');
      const select = element.querySelector<HTMLSelectElement>('[id^="filter"]')!;
      assert.isNotNull(select);
      assert.strictEqual(select.value, 'Sheriff Config 2');
    });

    it('should initialize with default when no value is in uri nor in LocalCtorage', async () => {
      assert.strictEqual(element.state.selectedSubscription, '');
      const select = element.querySelector<HTMLSelectElement>('[id^="filter"]')!;
      assert.isNotNull(select);
      assert.strictEqual(select.value, '');
    });

    it('should initialize with query parameter from uri if present', async () => {
      const testSearch = '?selectedSubscription=Sheriff%20Config%202';

      // Change the URL search part
      window.history.pushState({}, '', window.location.pathname + testSearch);
      // Verify that window.location.search is updated
      assert.strictEqual(window.location.search, testSearch);

      element = newInstance();
      await fetchMock.flush(true);

      assert.strictEqual(element.state.selectedSubscription, 'Sheriff Config 2');
    });
  });
});
