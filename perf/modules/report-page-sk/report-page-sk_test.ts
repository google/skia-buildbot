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
  const mockExploreInstances: any[] = [];

  // Helper to create a mock Anomaly.
  const createMockAnomaly = (id: number): Anomaly => ({
    id: id.toString(),
    test_path: '',
    bug_id: -1,
    start_revision: 0,
    end_revision: 3,
    display_commit_number: 3,
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

  let factory: () => ReportPageSk;

  const initializeElement = async () => {
    element = factory();
    element.exploreSimpleSkFactory = () => {
      const mockInstance = document.createElement('div') as any;
      mockInstance.updateChartHeight = sinon.spy();
      mockInstance.state = {};
      mockInstance.extendRange = sinon.spy(() => Promise.resolve());
      mockInstance.updateSelectedRangeWithPlotSummary = sinon.spy();
      mockInstance.setUseDiscreteAxis = sinon.spy();
      mockInstance.render = sinon.spy();
      mockExploreInstances.push(mockInstance);
      return mockInstance;
    };

    // Stub methods on the child anomalies table to isolate the parent component.
    const table = element.querySelector<AnomaliesTableSk>('#anomaly-table')!;
    sinon.stub(table, 'populateTable').resolves();
    sinon.stub(table, 'checkSelectedAnomalies');
    sinon.stub(table, 'initialCheckAllCheckbox');

    await (element.querySelector('graph-list-sk') as any).updateComplete;
    const graphContainer = element.querySelector<HTMLDivElement>('#graph-container')!;
    sinon
      .stub(graphContainer, 'querySelectorAll')
      .withArgs('explore-simple-sk')
      .callsFake(() => mockExploreInstances as unknown as NodeListOf<Element>);
  };

  beforeEach(async () => {
    // Mock the window.perf global object.
    window.perf = {
      dev_mode: false,
      instance_url: '',
      instance_name: 'chrome-perf-test',
      header_image_url: '',
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
      fetch_anomalies_from_sql: false,
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
      enable_v2_ui: false,
      extra_links: null,
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
    fetchMock.post('/_/fe_telemetry', {});

    factory = setUpElementUnderTest<ReportPageSk>('report-page-sk');
  });

  afterEach(() => {
    fetchMock.restore();
    sinon.restore();
    // Clear the array of mock instances for the next test.
    mockExploreInstances.length = 0;
  });

  describe('Page Title', () => {
    it('updates the title with bug ID', async () => {
      const originalSearch = window.location.search;
      window.history.replaceState({}, '', '?bugID=12345');

      const anomalies = [createMockAnomaly(0)];
      const timerangeMap = { '0': createMockTimerange() };

      fetchMock.post('/_/anomalies/group_report', {
        anomaly_list: anomalies,
        timerange_map: timerangeMap,
        selected_keys: [],
      });

      await initializeElement();
      await fetchMock.flush(true);

      assert.equal(document.title, `Report for bug: 12345`);
      window.history.replaceState({}, '', originalSearch);
    });
  });

  describe('Guide Link', () => {
    it('renders the guide link with the correct URL and attributes', async () => {
      await initializeElement();
      const guideLink = element.querySelector<HTMLAnchorElement>('.title-container a');
      assert.isNotNull(guideLink, 'Guide link should be rendered');
      assert.strictEqual(
        guideLink!.getAttribute('href'),
        'https://skia.googlesource.com/buildbot/+/refs/heads/main/perf/report-page-guide.md'
      );
      assert.strictEqual(guideLink!.getAttribute('target'), '_blank');
      assert.strictEqual(guideLink!.getAttribute('rel'), 'noopener');
      assert.strictEqual(guideLink!.getAttribute('title'), 'Report Page Guide');
      assert.isNotNull(guideLink!.querySelector('help-icon-sk'), 'Should contain a help icon');
    });
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

      await initializeElement();
      const graphContainer = element.querySelector<HTMLDivElement>('#graph-container')!;
      const appendSpy = sinon.spy(graphContainer, 'appendChild');

      await fetchMock.flush(true);

      // First chunk should start loading immediately.
      await waitUntil(() => {
        return appendSpy.callCount === chunkSize;
      });

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
      await waitUntil(() => element['_allGraphsLoaded']);

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

      await initializeElement();
      const graphContainer = element.querySelector<HTMLDivElement>('#graph-container')!;
      const appendSpy = sinon.spy(graphContainer, 'appendChild');
      await fetchMock.flush(true);
      await waitUntil(() => appendSpy.callCount === anomalyCount);

      // Simulate data-loaded for all graphs.
      for (let i = 0; i < anomalyCount; i++) {
        mockExploreInstances[i].dispatchEvent(new CustomEvent('data-loaded'));
      }
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
      (element.querySelector('graph-list-sk') as any)['syncChartSelection'](eventDetails as any);
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

      await initializeElement();
      const graphContainer = element.querySelector<HTMLDivElement>('#graph-container')!;
      const appendSpy = sinon.spy(graphContainer, 'appendChild');

      await fetchMock.flush(true);

      // We don't expect any graphs to be loaded, so no need to wait for appendSpy.
      // Instead, let the connectedCallbackPromise resolve.

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

      await initializeElement();
      const graphContainer = element.querySelector<HTMLDivElement>('#graph-container')!;
      const appendSpy = sinon.spy(graphContainer, 'appendChild');
      const table = element.querySelector<AnomaliesTableSk>('#anomaly-table')!;

      await fetchMock.flush(true);

      await waitUntil(() => appendSpy.callCount === anomalies.length);
      mockExploreInstances.forEach((instance) =>
        instance.dispatchEvent(new CustomEvent('data-loaded'))
      );
      await waitUntil(() => element['_allGraphsLoaded']);

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

      await initializeElement();
      const graphContainer = element.querySelector<HTMLDivElement>('#graph-container')!;
      const appendSpy = sinon.spy(graphContainer, 'appendChild');
      const table = element.querySelector<AnomaliesTableSk>('#anomaly-table')!;

      await fetchMock.flush(true);

      await waitUntil(() => appendSpy.callCount === anomalies.length);
      mockExploreInstances.forEach((instance) =>
        instance.dispatchEvent(new CustomEvent('data-loaded'))
      );
      await waitUntil(() => element['_allGraphsLoaded']);

      assert.strictEqual(appendSpy.callCount, anomalies.length);
      assert.isTrue(
        (table.checkSelectedAnomalies as sinon.SinonStub).calledWith(anomalies),
        'checkSelectedAnomalies should be called with all anomalies'
      );
      assert.strictEqual(element['anomalyTracker'].getSelectedAnomalies().length, anomalies.length);

      window.history.replaceState({}, '', originalSearch);
    });
  });

  describe('Even X-Axis Spacing Synchronization', () => {
    beforeEach(async () => {
      // Setup with a few mock graphs
      const anomalies = Array.from({ length: 3 }, (_, i) => createMockAnomaly(i));
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

      await initializeElement();
      await fetchMock.flush(true);

      // Wait for all graphs to be appended
      const graphContainer = element.querySelector<HTMLDivElement>('#graph-container')!;
      await waitUntil(() => graphContainer.children.length === anomalies.length);

      // Simulate data-loaded for all graphs
      mockExploreInstances.forEach((instance) =>
        instance.dispatchEvent(new CustomEvent('data-loaded'))
      );
      await waitUntil(() => element['_allGraphsLoaded']);
    });

    it('syncs evenXAxisSpacing state to other graphs when event is received', async () => {
      const graph1 = mockExploreInstances[0];
      const graph2 = mockExploreInstances[1];
      const graph3 = mockExploreInstances[2];

      // Add setUseDiscreteAxis spies to the mock instances
      mockExploreInstances.forEach((instance) => {
        instance.setUseDiscreteAxis = sinon.spy();
      });

      // Simulate event from graph1
      const event = new CustomEvent('even-x-axis-spacing-changed', {
        detail: { value: true, graph_index: 0 },
        bubbles: true,
      });
      graph1.dispatchEvent(event);

      // Check that setUseDiscreteAxis was called on the other graphs
      assert.isTrue(graph2.setUseDiscreteAxis.calledOnceWith(true), 'graph2 should be updated');
      assert.isTrue(graph3.setUseDiscreteAxis.calledOnceWith(true), 'graph3 should be updated');

      // The graph originating the event should not have its method called by the handler
      assert.isTrue(graph1.setUseDiscreteAxis.notCalled, 'graph1 should not be updated by handler');
    });

    it('does not sync to the source graph', async () => {
      const graph1 = mockExploreInstances[0];
      const graph2 = mockExploreInstances[1];

      mockExploreInstances.forEach((instance) => {
        instance.setUseDiscreteAxis = sinon.spy();
      });

      // Simulate event from graph2
      const event = new CustomEvent('even-x-axis-spacing-changed', {
        detail: { value: false, graph_index: 1 },
        bubbles: true,
      });
      graph2.dispatchEvent(event);

      assert.isTrue(graph1.setUseDiscreteAxis.calledOnceWith(false), 'graph1 should be updated');
      assert.isTrue(graph2.setUseDiscreteAxis.notCalled, 'graph2 should not be updated by handler');
    });
  });

  describe('V2 Dashboard Integration', () => {
    it('renders explore-multi-v2-sk when the v2 url flag is present', async () => {
      const originalSearch = window.location.search;
      window.history.replaceState({}, '', '?sid=abc&v2=true');

      // Mock the Web Worker and its static assets to avoid real network fetch requests
      (window as any).WORKER_URL =
        'data:application/javascript,self.postMessage({ type: "LOADED" }); self.onmessage = (e) => { if (e.data.type === "INIT") { self.postMessage({ type: "READY" }); } };';
      fetchMock.get('glob:*/filter.worker.js', 'self.postMessage({ type: "LOADED" });');
      fetchMock.get('glob:*/meta.json', { version: 'test-version' });
      fetchMock.get('glob:*/filter.wasm', new ArrayBuffer(0));
      fetchMock.get('glob:*/params.json', {});
      fetchMock.get('glob:*/traces.json', new ArrayBuffer(0));

      const anomalies = [createMockAnomaly(0)];
      const timerangeMap = { '0': createMockTimerange() };

      // Stub the feature flag property to guarantee opt-in path activation in the test runner
      const isV2Stub = sinon.stub(ReportPageSk.prototype as any, 'isV2Enabled').get(() => true);

      fetchMock.post('/_/anomalies/group_report', {
        anomaly_list: anomalies,
        timerange_map: timerangeMap,
        selected_keys: ['0'],
      });

      element = factory();
      const table = element.querySelector<AnomaliesTableSk>('#anomaly-table')!;
      sinon.stub(table, 'populateTable').resolves();
      sinon.stub(table, 'checkSelectedAnomalies');

      await fetchMock.flush(true);
      // Wait for async fetch anomalies promise chain to resolve and render the child
      await waitUntil(() => element.querySelector('explore-multi-v2-sk') !== null, 5000);

      const multiExplore = element.querySelector('explore-multi-v2-sk');
      assert.isNotNull(multiExplore, 'explore-multi-v2-sk should be rendered');
      assert.isNull(element.querySelector('graph-list-sk'), 'graph-list-sk should NOT be rendered');

      window.history.replaceState({}, '', originalSearch);
      isV2Stub.restore();
    });

    describe('Split keys logic for V2 Dashboard', () => {
      beforeEach(async () => {
        fetchMock.post('/_/anomalies/group_report', {
          anomaly_list: [],
          timerange_map: {},
          selected_keys: [],
        });
        await initializeElement();
        await fetchMock.flush(true);

        sinon.stub(element as any, 'isV2Enabled').get(() => true);
      });

      it('splits only by specific subtest keys when they vary, and ignores base keys', () => {
        const anomaly1 = createMockAnomaly(0);
        anomaly1.test_path = ',master=m1,bot=b1,benchmark=bench1,test=t1,subtest_1=s1';
        const anomaly2 = createMockAnomaly(1);
        anomaly2.test_path = ',master=m2,bot=b1,benchmark=bench1,test=t1,subtest_1=s2';

        const timerangeMap = {
          '0': createMockTimerange(),
          '1': createMockTimerange(),
        };

        element['anomalyTracker'].load([anomaly1, anomaly2], timerangeMap, ['0', '1']);

        element['updateMultiExploreStateV2']();

        assert.deepEqual(element['_splitKeysV2'], new Set(['subtest_1']));
      });

      it('falls back to varying base keys when no subtest keys vary', () => {
        const anomaly1 = createMockAnomaly(0);
        anomaly1.test_path = ',master=m1,bot=b1,benchmark=bench1,test=t1';
        const anomaly2 = createMockAnomaly(1);
        anomaly2.test_path = ',master=m2,bot=b1,benchmark=bench1,test=t1';

        const timerangeMap = {
          '0': createMockTimerange(),
          '1': createMockTimerange(),
        };

        element['anomalyTracker'].load([anomaly1, anomaly2], timerangeMap, ['0', '1']);

        element['updateMultiExploreStateV2']();

        assert.deepEqual(element['_splitKeysV2'], new Set(['master']));
      });

      it('never splits by unit, stat, or improvement_dir', () => {
        const anomaly1 = createMockAnomaly(0);
        anomaly1.test_path = ',unit=u1,stat=s1,improvement_dir=d1,bot=b1';
        const anomaly2 = createMockAnomaly(1);
        anomaly2.test_path = ',unit=u2,stat=s2,improvement_dir=d2,bot=b1';

        const timerangeMap = {
          '0': createMockTimerange(),
          '1': createMockTimerange(),
        };

        element['anomalyTracker'].load([anomaly1, anomaly2], timerangeMap, ['0', '1']);

        element['updateMultiExploreStateV2']();

        assert.deepEqual(element['_splitKeysV2'], new Set<string>());
      });
    });

    describe('Bounding box and timestamp calculations for V2 Dashboard', () => {
      beforeEach(async () => {
        fetchMock.post('/_/anomalies/group_report', {
          anomaly_list: [],
          timerange_map: {},
          selected_keys: [],
        });
        await initializeElement();
        await fetchMock.flush(true);

        sinon.stub(element as any, 'isV2Enabled').get(() => true);
      });

      it('calculates the correct viewport and timestamp range bounds', () => {
        const anomaly1 = createMockAnomaly(0);
        anomaly1.start_revision = 1000;
        anomaly1.end_revision = 1100;
        anomaly1.test_path = ',master=m1,bot=b1,benchmark=bench1,test=t1';

        const anomaly2 = createMockAnomaly(1);
        anomaly2.start_revision = 950;
        anomaly2.end_revision = 1250;
        anomaly2.test_path = ',master=m1,bot=b1,benchmark=bench1,test=t1';

        const timerangeMap = {
          '0': { begin: 2000, end: 3000 },
          '1': { begin: 1500, end: 3500 },
        };

        element['anomalyTracker'].load([anomaly1, anomaly2], timerangeMap, ['0', '1']);

        element['updateMultiExploreStateV2']();

        // minCommit = 950, maxCommit = 1250
        // viewportMinX = Math.max(0, 950 - 100) = 850
        // viewportMaxX = 1250 + 100 = 1350
        assert.strictEqual(element['_viewportMinXV2'], 850);
        assert.strictEqual(element['_viewportMaxXV2'], 1350);

        // minBegin = 1500, maxEnd = 3500
        // beginV2 = 1500 - 7 * 24 * 60 * 60
        // endV2 = 3500 + 7 * 24 * 60 * 60
        assert.strictEqual(element['_beginV2'], 1500 - 7 * 24 * 60 * 60);
        assert.strictEqual(element['_endV2'], 3500 + 7 * 24 * 60 * 60);
      });
    });
  });
});
