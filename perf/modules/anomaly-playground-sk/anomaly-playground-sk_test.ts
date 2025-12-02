import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import sinon from 'sinon';
import { AnomalyPlaygroundSk } from './anomaly-playground-sk';
import { setUpElementUnderTest, waitForRender } from '../../../infra-sk/modules/test_util';
import { ExploreSimpleSk } from '../explore-simple-sk/explore-simple-sk';
import {
  Anomaly,
  TraceSet,
  Trace,
  CommitNumber,
  TimestampSeconds,
  ColumnHeader,
  ReadOnlyParamSet,
  FrameResponse,
} from '../json';

// Import for side effects.
import './anomaly-playground-sk';

fetchMock.config.overwriteRoutes = true;

// Define window.perf to avoid runtime errors if code accesses it.
window.perf = {
  dev_mode: false,
  instance_url: '',
  radius: 2,
  key_order: null,
  num_shift: 50,
  interesting: 2,
  step_up_only: false,
  commit_range_url: '',
  demo: true,
  display_group_by: false,
  hide_list_of_commits_on_explore: false,
  notifications: 'none',
  fetch_chrome_perf_anomalies: false,
  fetch_anomalies_from_sql: false,
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
  instance_name: 'chrome-perf-test',
  header_image_url: '',
  enable_v2_ui: false,
};

describe('anomaly-playground-sk', () => {
  const newInstance = setUpElementUnderTest<AnomalyPlaygroundSk>('anomaly-playground-sk');

  let element: AnomalyPlaygroundSk;

  beforeEach(() => {
    // Setup common mocks.
    fetchMock.get('/_/login/status', {
      email: 'someone@example.org',
      roles: ['editor'],
    });
    fetchMock.get('path:/_/defaults/', {
      status: 200,
      body: JSON.stringify({}),
    });
    fetchMock.get(/_\/initpage\/.*/, {
      dataframe: {
        traceset: null,
        header: null,
        paramset: {},
        skip: 0,
      },
      ticks: [],
      skps: [],
      msg: '',
    });
    fetchMock.post('/_/count/', {
      count: 0,
      paramset: {},
    });
    fetchMock.post('/_/fe_telemetry/', 200);

    element = newInstance();
  });

  afterEach(() => {
    sinon.restore();
    fetchMock.reset();
  });

  it('instantiates successfully', () => {
    assert.isNotNull(element);
  });

  it('clears anomalies when detect is clicked', async () => {
    await waitForRender(element);
    await new Promise((resolve) => setTimeout(resolve, 0));

    // Find the injected explore-simple-sk.
    let explore = element.querySelector('explore-simple-sk') as ExploreSimpleSk;
    if (!explore && element.shadowRoot) {
      explore = element.shadowRoot.querySelector('explore-simple-sk') as ExploreSimpleSk;
    }

    assert.isNotNull(explore, 'explore-simple-sk should be found');

    // Setup initial data with anomalies.
    const initialAnomalies = {
      trace1: {
        100: {
          id: '1',
          bug_id: 123,
          start_revision: 100,
          end_revision: 105,
          median_before_anomaly: 10,
          median_after_anomaly: 20,
          is_improvement: false,
          recovered: false,
        } as Anomaly,
      },
    };

    const frameResponse: FrameResponse = {
      dataframe: {
        traceset: TraceSet({ trace1: Trace([1, 2, 3]) }),
        header: [
          { offset: CommitNumber(100), timestamp: TimestampSeconds(1000) } as ColumnHeader,
          { offset: CommitNumber(101), timestamp: TimestampSeconds(1001) } as ColumnHeader,
          { offset: CommitNumber(102), timestamp: TimestampSeconds(1002) } as ColumnHeader,
        ],
        paramset: ReadOnlyParamSet({}),
        skip: 0,
        traceMetadata: [],
      },
      anomalymap: initialAnomalies,
      skps: [],
      msg: '',
      display_mode: 'display_plot',
    };

    const frameRequest = {
      begin: 1000,
      end: 1002,
      tz: '',
    };

    // Load data and anomalies.
    await explore.UpdateWithFrameResponse(frameResponse, frameRequest, false);
    await waitForRender(element);

    // Verify anomalies are there.
    assert.isNotEmpty(explore.getAnomalyMap());

    // Mock detection to return empty anomalies.
    fetchMock.post('/_/playground/anomaly/v1/detect', {
      anomalies: [],
    });

    // Trigger Detect.
    // The detect button is disabled by default because algorithm is empty.
    const algoSelect = element.querySelector('#algorithm-selector') as any;
    algoSelect.value = 'percent';
    algoSelect.dispatchEvent(new Event('change', { bubbles: true }));
    algoSelect.dispatchEvent(new Event('input', { bubbles: true }));

    await waitForRender(element);

    let detectBtn = element.querySelector('.buttons md-filled-button') as any;
    if (!detectBtn && element.shadowRoot) {
      detectBtn = element.shadowRoot.querySelector('.buttons md-filled-button') as any;
    }
    assert.isNotNull(detectBtn, 'Detect button not found');
    assert.isFalse(detectBtn.disabled, 'Detect button should be enabled');

    detectBtn.click();

    // Wait for detection to complete.
    await fetchMock.flush(true);
    await waitForRender(element);

    // Verify anomalies are cleared.
    const anomalyMapAfter = explore.getAnomalyMap();
    assert.isEmpty(
      anomalyMapAfter,
      'Anomaly map should be empty after detection returns no anomalies'
    );
  });
});
