import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import sinon from 'sinon';
import { ReportPageSk } from './report-page-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { Anomaly, Timerange } from '../json';
import { AnomaliesTableSk } from '../anomalies-table-sk/anomalies-table-sk';

describe('ReportPageSk', () => {
  const waitUntil = (condition: () => boolean, timeoutMs: number = 2000): Promise<void> => {
    return new Promise((resolve, reject) => {
      const interval = setInterval(() => {
        if (condition()) {
          clearInterval(interval);
          clearTimeout(timeout);
          resolve();
        }
      }, 10); // Check every 10ms

      const timeout = setTimeout(() => {
        clearInterval(interval);
        reject(new Error(`waitUntil timed out after ${timeoutMs}ms`));
      }, timeoutMs);
    });
  };

  let element: ReportPageSk;
  const mockExploreInstances: (HTMLElement & {
    extendRange: sinon.SinonSpy;
    updateChartHeight: sinon.SinonSpy;
    updateSelectedRangeWithPlotSummary: sinon.SinonSpy;
    state: object;
  })[] = [];

  // Helper to create a mock Anomaly.
  const createMockAnomaly = (id: number): Anomaly => ({
    id: id.toString(),
    test_path: '',
    bug_id: -1,
    start_revision: 0,
    end_revision: 3,
    is_improvement: false,
    recovered: true,
    state: '',
    statistic: '',
    units: '',
    degrees_of_freedom: 0,
    median_before_anomaly: 0,
    median_after_anomaly: 0,
    p_value: 0,
    segment_size_after: 0,
    segment_size_before: 0,
    std_dev_before_anomaly: 0,
    t_statistic: 0,
    subscription_name: '',
    bug_component: '',
    bug_labels: [],
    bug_cc_emails: [],
    bisect_ids: [],
  });

  // Helper to create a mock Timerange.
  const createMockTimerange = (): Timerange => ({
    begin: 1672531200, // Jan 1, 2023
    end: 1672617600, // Jan 2, 2023
  });

  beforeEach(() => {
    // Mock the window.perf global object.
    window.perf = {
      instance_url: '',
      commit_range_url: '',
      key_order: [],
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
      trace_format: 'chrome',
      need_alert_action: false,
      bug_host_url: '',
      git_repo_url: 'https://example.com/repo',
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

    fetchMock.config.overwriteRoutes = true;
    fetchMock.get('glob:/_/initpage/*', {});
    fetchMock.get('/_/defaults/', {
      default_param_selections: {},
      default_url_values: {},
    });
    fetchMock.get('/_/login/status', { email: 'test@google.com', roles: ['editor'] });
    fetchMock.post('/_/frame/start', {});

    // This spy will allow us to inspect calls to set the loading message.
    sinon.spy(ReportPageSk.prototype, 'setCurrentlyLoading' as any);

    // Mock lookupCids as it's called but not essential for this test's focus.
    fetchMock.post('/_/cid/', { commitSlice: [] });

    element = setUpElementUnderTest<ReportPageSk>('report-page-sk')();
    element.exploreSimpleSkFactory = () => {
      const mockInstance = document.createElement('div') as any;
      mockInstance.updateChartHeight = sinon.spy();
      mockInstance.state = {};
      mockInstance.extendRange = sinon.spy(() => Promise.resolve());
      mockInstance.updateSelectedRangeWithPlotSummary = sinon.spy();
      mockExploreInstances.push(mockInstance);
      return mockInstance;
    };

    // Stub methods on the child anomalies table to isolate the parent component.
    const table = element.querySelector<AnomaliesTableSk>('#anomaly-table')!;
    sinon.stub(table, 'populateTable').resolves();
    sinon.stub(table, 'checkSelectedAnomalies');
    sinon.stub(table, 'initialCheckAllCheckbox');

    const graphContainer = element.querySelector<HTMLDivElement>('#graph-container')!;
    sinon
      .stub(graphContainer, 'querySelectorAll')
      .withArgs('explore-simple-sk')
      .callsFake(() => mockExploreInstances as unknown as NodeListOf<Element>);
  });

  afterEach(() => {
    fetchMock.restore();
    sinon.restore();
    // Clear the array of mock instances for the next test.
    mockExploreInstances.length = 0;
  });

  describe('Graph Loading Functionality', () => {
    it('loads selected graphs in chunks and appends them to the bottom', async () => {
      const anomalyCount = 7;
      const chunkSize = 5;
      const anomalies = Array.from({ length: anomalyCount }, (_, i) => createMockAnomaly(i));
      const timerangeMap = anomalies.reduce(
        (acc, anom) => {
          acc[anom.id] = createMockTimerange();
          return acc;
        },
        {} as { [key: string]: Timerange }
      );

      fetchMock.post('/_/anomalies/group_report', {
        anomaly_list: anomalies,
        timerange_map: timerangeMap,
        selected_keys: anomalies.map((a) => a.id),
      });

      const graphContainer = element.querySelector<HTMLDivElement>('#graph-container')!;
      const appendSpy = sinon.spy(graphContainer, 'append');

      const connectedCallbackPromise = element.connectedCallback();
      await fetchMock.flush(true);

      // First chunk should start loading immediately.
      await waitUntil(() => appendSpy.callCount === chunkSize);

      // Simulate data-loaded events for the first chunk. Second chunk should
      // start loading.
      for (let i = 0; i < chunkSize; i++) {
        mockExploreInstances[i].dispatchEvent(new CustomEvent('data-loaded'));
      }
      await waitUntil(() => appendSpy.callCount === anomalyCount);

      // Simulate data-loaded events for the rest.
      for (let i = chunkSize; i < anomalyCount; i++) {
        mockExploreInstances[i].dispatchEvent(new CustomEvent('data-loaded'));
      }

      // This will be resolved only when all graphs are loaded.
      await connectedCallbackPromise;

      assert.strictEqual(
        appendSpy.callCount,
        anomalyCount,
        'append should be called for each graph'
      );
      assert.strictEqual(graphContainer.children.length, anomalyCount);
      assert.strictEqual(graphContainer.children[0], mockExploreInstances[0]);
      assert.strictEqual(graphContainer.children[6], mockExploreInstances[6]);
    });

    it('does not update URL until all graphs are loaded', async () => {
      const anomalyCount = 3;
      const anomalies = Array.from({ length: anomalyCount }, (_, i) => createMockAnomaly(i));
      const timerangeMap = anomalies.reduce(
        (acc, anom) => {
          acc[anom.id] = createMockTimerange();
          return acc;
        },
        {} as { [key: string]: Timerange }
      );

      fetchMock.post('/_/anomalies/group_report', {
        anomaly_list: anomalies,
        timerange_map: timerangeMap,
        selected_keys: anomalies.map((a) => a.id),
      });

      const graphContainer = element.querySelector<HTMLDivElement>('#graph-container')!;
      const appendSpy = sinon.spy(graphContainer, 'append');
      const connectedCallbackPromise = element.connectedCallback();
      await fetchMock.flush(true);
      await waitUntil(() => appendSpy.callCount === anomalyCount);

      // Simulate data-loaded for all graphs.
      for (let i = 0; i < anomalyCount; i++) {
        mockExploreInstances[i].dispatchEvent(new CustomEvent('data-loaded'));
      }
      await connectedCallbackPromise;
      await waitUntil(() => element['_allGraphsLoaded']);

      // URL should not be updated while graphs are still loading.
      assert.isFalse(mockExploreInstances[0].extendRange.called);
      assert.isFalse(mockExploreInstances[0].updateSelectedRangeWithPlotSummary.called);

      // And now it should be.
      const eventDetails = {
        detail: {
          value: { begin: 123, end: 456 },
          domain: 'commit',
          graphNumber: 1,
          start: 99,
          end: 999,
        },
      };
      element['syncChartSelection'](eventDetails as any);
      assert.isTrue(mockExploreInstances[0].updateSelectedRangeWithPlotSummary.called);
    });

    it('load no anomalies when anomalyGroupID is in URL params', async () => {
      const originalSearch = window.location.search;
      window.history.replaceState({}, '', '?anomalyGroupID=789');

      const anomalies = [createMockAnomaly(0), createMockAnomaly(1)];
      const timerangeMap = anomalies.reduce(
        (acc, anom) => {
          acc[anom.id] = createMockTimerange();
          return acc;
        },
        {} as { [key: string]: Timerange }
      );

      fetchMock.post('/_/anomalies/group_report', {
        anomaly_list: anomalies,
        timerange_map: timerangeMap,
        selected_keys: [],
      });

      const graphContainer = element.querySelector<HTMLDivElement>('#graph-container')!;
      const appendSpy = sinon.spy(graphContainer, 'append');

      const connectedCallbackPromise = element.connectedCallback();
      await fetchMock.flush(true);

      // We don't expect any graphs to be loaded, so no need to wait for appendSpy.
      // Instead, let the connectedCallbackPromise resolve.

      await connectedCallbackPromise;

      assert.strictEqual(appendSpy.callCount, 0, 'Should load no graphs');
      assert.strictEqual(
        element['anomalyTracker'].getSelectedAnomalies().length,
        0,
        'Should have no selected anomalies'
      );

      window.history.replaceState({}, '', originalSearch);
    });

    it('loads all anomaly graphs when sid is in URL params', async () => {
      const originalSearch = window.location.search;
      window.history.replaceState({}, '', '?sid=abc');

      const anomalies = [createMockAnomaly(0), createMockAnomaly(1)];
      const timerangeMap = anomalies.reduce(
        (acc, anom) => {
          acc[anom.id] = createMockTimerange();
          return acc;
        },
        {} as { [key: string]: Timerange }
      );

      fetchMock.post('/_/anomalies/group_report', {
        anomaly_list: anomalies,
        timerange_map: timerangeMap,
        selected_keys: [], // sid-based selection happens server-side
      });

      const graphContainer = element.querySelector<HTMLDivElement>('#graph-container')!;
      const appendSpy = sinon.spy(graphContainer, 'append');
      const table = element.querySelector<AnomaliesTableSk>('#anomaly-table')!;

      const connectedCallbackPromise = element.connectedCallback();
      await fetchMock.flush(true);

      await waitUntil(() => appendSpy.callCount === anomalies.length);
      mockExploreInstances.forEach((instance) =>
        instance.dispatchEvent(new CustomEvent('data-loaded'))
      );
      await connectedCallbackPromise;

      assert.strictEqual(appendSpy.callCount, anomalies.length);
      assert.isTrue(
        (table.initialCheckAllCheckbox as sinon.SinonStub).calledOnce,
        'initialCheckAllCheckbox should be called'
      );
      assert.strictEqual(element['anomalyTracker'].getSelectedAnomalies().length, anomalies.length);

      window.history.replaceState({}, '', originalSearch);
    });

    it('loads all anomaly graphs when specific anomalyIDs are in URL params', async () => {
      const originalSearch = window.location.search;
      window.history.replaceState({}, '', '?anomalyIDs=0,1');

      const anomalies = [createMockAnomaly(0), createMockAnomaly(1)];
      const timerangeMap = anomalies.reduce(
        (acc, anom) => {
          acc[anom.id] = createMockTimerange();
          return acc;
        },
        {} as { [key: string]: Timerange }
      );

      fetchMock.post('/_/anomalies/group_report', {
        anomaly_list: anomalies,
        timerange_map: timerangeMap,
        selected_keys: ['0', '1'],
      });

      const graphContainer = element.querySelector<HTMLDivElement>('#graph-container')!;
      const appendSpy = sinon.spy(graphContainer, 'append');
      const table = element.querySelector<AnomaliesTableSk>('#anomaly-table')!;

      const connectedCallbackPromise = element.connectedCallback();
      await fetchMock.flush(true);

      await waitUntil(() => appendSpy.callCount === anomalies.length);
      mockExploreInstances.forEach((instance) =>
        instance.dispatchEvent(new CustomEvent('data-loaded'))
      );
      await connectedCallbackPromise;

      assert.strictEqual(appendSpy.callCount, anomalies.length);
      assert.isTrue(
        (table.checkSelectedAnomalies as sinon.SinonStub).calledWith(anomalies),
        'checkSelectedAnomalies should be called with all anomalies'
      );
      assert.strictEqual(element['anomalyTracker'].getSelectedAnomalies().length, anomalies.length);

      window.history.replaceState({}, '', originalSearch);
    });
  });
});
