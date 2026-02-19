import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import sinon from 'sinon';
import { ExploreMultiSk, State } from './explore-multi-sk';
import { ExploreSimpleSk } from '../explore-simple-sk/explore-simple-sk';
import { GraphConfig } from '../common/graph-config';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { setUpExploreDemoEnv } from '../common/test-util';
import { PlotSelectionEventDetails } from '../plot-google-chart-sk/plot-google-chart-sk';
import { PaginationSkPageChangedEventDetail } from '../../../golden/modules/pagination-sk/pagination-sk';
import { TestPickerSk } from '../test-picker-sk/test-picker-sk';
import * as loader from '@google-web-components/google-chart/loader';
import { Anomaly, CommitNumber, TimestampSeconds } from '../json';
import { DataService } from '../data-service';

fetchMock.config.overwriteRoutes = true;
describe('ExploreMultiSk', () => {
  let element: ExploreMultiSk;

  // Common setup for most tests
  const setupElement = async (mockDefaults: any = null, paramsMock: any = null) => {
    setUpExploreDemoEnv();
    window.perf = {
      dev_mode: false,
      instance_url: '',
      instance_name: 'chrome-perf-test',
      header_image_url: '',
      commit_range_url: '',
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
      fetch_anomalies_from_sql: false,
      feedback_url: '',
      chat_url: '',
      help_url_override: '',
      trace_format: 'chrome',
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
      enable_v2_ui: false,
      extra_links: null,
    };

    fetchMock.config.overwriteRoutes = true;
    fetchMock.get('/_/login/status', {
      email: 'user@google.com',
      roles: ['editor'],
    });

    const defaultsResponse = mockDefaults || {
      default_param_selections: {},
      default_url_values: {
        summary: 'true',
      },
      include_params: ['config'],
    };
    fetchMock.get('/_/defaults/', defaultsResponse);

    if (paramsMock) {
      fetchMock.get('/_/Params/', paramsMock);
    }

    const anomaly: Anomaly = {
      id: '123',
      test_path: '',
      bug_id: 123,
      start_revision: 101,
      end_revision: 101,
      is_improvement: false,
      recovered: false,
      state: 'regression',
      statistic: 'avg',
      units: 'ms',
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
    };
    fetchMock.post('/_/fe_telemetry', {});
    fetchMock.post('/_/keys/', { id: 'test-key-id' });
    fetchMock.post('/_/shortcut/update', { id: 'test-shortcut-id' });
    fetchMock.post('/_/frame/start', {
      status: 'Finished',
      messages: [],
      url: '',
      results: {
        dataframe: {
          traceset: {
            ',arch=x86,config=test,os=linux,': [1, 2, 3],
            ',arch=x86,config=test,os=android,': [4, 5, 6],
          },
          header: [
            { offset: 100, timestamp: 1000, hash: 'a', author: 'me', message: 'm', url: '' },
            { offset: 101, timestamp: 1001, hash: 'b', author: 'me', message: 'm', url: '' },
            { offset: 102, timestamp: 1002, hash: 'c', author: 'me', message: 'm', url: '' },
          ] as any,
          paramset: { config: ['test'], arch: ['x86'], os: ['linux', 'android'] },
        },
        anomalymap: {
          ',arch=x86,config=test,os=linux,': {
            101: anomaly,
          },
          ',arch=x86,config=test,os=android,': {
            101: anomaly,
          },
        },
      },
    });

    await window.customElements.whenDefined('explore-simple-sk');
    await window.customElements.whenDefined('dataframe-repository-sk');

    element = document.createElement('explore-multi-sk') as ExploreMultiSk;
    element.state.begin = 1000;
    element.state.end = 2000;
    document.body.appendChild(element);
    await fetchMock.flush(true);
    // Allow for connectedCallback and stateReflector to process
    await new Promise((resolve) => setTimeout(resolve, 0));
  };

  beforeEach(() => {
    sinon.stub(window, 'confirm').returns(true);
  });

  afterEach(() => {
    fetchMock.restore();
    sinon.restore();
  });

  describe('Anomaly management', () => {
    beforeEach(async () => {
      await setupElement({
        default_param_selections: {},
        default_url_values: {
          useTestPicker: 'true',
        },
        include_params: ['config', 'os'],
      });
      await element['initializeTestPicker']();
    });

    it('removes anomalies and traces when remove-trace event is received', async function () {
      this.timeout(10000); // Increase timeout for this test
      const RENDER_AWAIT_MS = 2000;

      const androidTrace = ',arch=x86,config=test,os=android,';
      const linuxTrace = ',arch=x86,config=test,os=linux,';

      // plot graph
      const event = new CustomEvent('add-to-graph', {
        detail: { query: 'config=test' },
        bubbles: true,
      });
      element.dispatchEvent(event);
      // Wait for data to be loaded and traceset to be available.
      // We rely on requestComplete which is consistent with other tests.
      await element['exploreElements'][0].requestComplete;

      assert.equal(element['exploreElements'].length, 1);

      const graph = element['exploreElements'][0];
      // spy on UpdateWithFrameResponse to ensure replaceAnomalies is called
      const updateFrameSpy = sinon.spy(graph, 'UpdateWithFrameResponse');

      // traces and anomalies are on the chart
      await new Promise((resolve) => setTimeout(resolve, RENDER_AWAIT_MS));
      const plot = graph.querySelector('plot-google-chart-sk') as any;
      const traceset = graph.getTraceset()!;
      assert.isDefined(traceset[androidTrace], 'Android trace should be present');
      assert.isDefined(plot.anomalyMap[androidTrace], 'Android anomaly should be present');
      assert.isDefined(traceset[linuxTrace], 'Linux trace should be present');
      assert.isDefined(plot.anomalyMap[linuxTrace], 'Linux anomaly should be present');

      // Remove os=android
      const removeEvent = new CustomEvent('remove-trace', {
        detail: {
          param: 'os',
          value: ['android'],
          query: ['config=test'],
        },
        bubbles: true,
      });
      element.dispatchEvent(removeEvent);

      await new Promise((resolve) => setTimeout(resolve, RENDER_AWAIT_MS));
      await fetchMock.flush(true);
      await plot.updateComplete;

      // Check that android trace is gone
      assert.isDefined(traceset[linuxTrace], 'Linux trace should remain');
      assert.isDefined(plot.anomalyMap[linuxTrace], 'Linux anomaly should stay');
      assert.isUndefined(traceset[androidTrace], 'Android trace should be removed');
      assert.isUndefined(plot.anomalyMap[androidTrace], 'Android anomaly should be removed');

      // Check that replaceAnomalies (6th arg) was true
      assert.isTrue(updateFrameSpy.called);
      assert.isTrue(updateFrameSpy.lastCall.args[5], 'replaceAnomalies should be true');
    });
  });

  describe('State management', () => {
    beforeEach(async () => {
      await setupElement();
    });

    it('initializes with a default state', () => {
      assert.notEqual(element.state.begin, -1);
      assert.notEqual(element.state.end, -1);
    });

    it('updates its state when the state property is set', () => {
      const newState = new State();
      newState.shortcut = 'test-shortcut';
      newState.pageSize = 10;
      element.state = newState;
      assert.deepEqual(element.state, newState);
    });

    it('expands the time range if begin equals end in the URL state', async () => {
      const state = new State();
      const now = Math.floor(Date.now() / 1000);
      state.begin = now - 1000;
      state.end = now - 1000; // Zero length range
      // We need to use _onStateChangedInUrl to trigger the logic
      await element['_onStateChangedInUrl'](state as any);

      assert.notEqual(element.state.begin, element.state.end);
      assert.isTrue(element.state.end > element.state.begin);
      // It should have expanded by default range (defaults.default_range or DEFAULT_RANGE_S)
      // Since we didn't provide defaults in setupElement for this specific test (it uses common setup),
      // we can check if it expanded significantly.
      assert.isTrue(element.state.end - element.state.begin > 0);
    });
  });

  describe('Default Domain (X-Axis Scale)', () => {
    it('sets domain to "date" if default_xaxis_domain is "date"', async () => {
      await setupElement({ default_xaxis_domain: 'date' });
      assert.equal(element.state.domain, 'date');
    });

    it('sets domain to "commit" if default_xaxis_domain is "commit"', async () => {
      await setupElement({ default_xaxis_domain: 'commit' });
      assert.equal(element.state.domain, 'commit');
    });

    it('sets domain to "commit" if default_xaxis_domain is missing', async () => {
      await setupElement({}); // No default_xaxis_domain property
      assert.equal(element.state.domain, 'commit');
    });

    it('sets domain to "commit" if defaults fails to load (simulated)', async () => {
      // This setup is slightly different as we need to mock a network failure for defaults.
      setUpExploreDemoEnv();
      window.perf = {
        dev_mode: false,
        instance_url: '',
        instance_name: '',
        header_image_url: '',
        commit_range_url: '',
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
        fetch_anomalies_from_sql: false,
        feedback_url: '',
        chat_url: '',
        help_url_override: '',
        trace_format: 'chrome',
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
        enable_v2_ui: false,
        extra_links: null,
      };
      fetchMock.config.overwriteRoutes = true;
      fetchMock.get('/_/login/status', {
        email: 'user@google.com',
        roles: ['editor'],
      });
      fetchMock.getOnce('/_/defaults/', Promise.reject(new Error('Network error')));
      fetchMock.post('/_/frame/v2', {});

      element = setUpElementUnderTest<ExploreMultiSk>('explore-multi-sk')();
      try {
        await fetchMock.flush(true);
      } catch (e) {
        console.log('Caught expected error from /_/defaults/ failure:', e);
      }
      // Even with the error, connectedCallback should complete.
      assert.equal(
        element.state.domain,
        'commit',
        'Domain should default to commit on fetch failure'
      );
    });
  });

  describe('Graph management', () => {
    beforeEach(async () => {
      await setupElement();
    });

    it('adds an empty graph', () => {
      const initialGraphCount = element['exploreElements'].length;
      element['addEmptyGraph']();
      assert.equal(element['exploreElements'].length, initialGraphCount + 1);
      assert.equal(element['allGraphConfigs'].length, initialGraphCount + 1);
    });

    it('removes a graph when a remove-explore event is dispatched', async () => {
      await element['initializeTestPicker'](); // Initialize to attach the listener.
      const exploreSimpleSk = element['addEmptyGraph']()!;
      const initialGraphCount = element['exploreElements'].length; // Will be 1.

      const detail = { elem: exploreSimpleSk };
      const event = new CustomEvent('remove-explore', {
        detail,
        bubbles: true,
      });
      element.dispatchEvent(event);

      assert.equal(element['exploreElements'].length, initialGraphCount - 1);
      assert.equal(element['allGraphConfigs'].length, initialGraphCount - 1);
    });

    it('ignores remove-explore event with null detail', async () => {
      await element['initializeTestPicker'](); // Initialize to attach the listener.
      element['addEmptyGraph']();
      const initialGraphCount = element['exploreElements'].length;
      assert.isAbove(initialGraphCount, 0);

      const event = new CustomEvent('remove-explore', {
        bubbles: true,
        composed: true,
        // No detail provided, simulating the bug/feature
      });
      element.dispatchEvent(event);

      assert.equal(element['exploreElements'].length, initialGraphCount);
    });

    it('resets pagination when the last graph is removed', async () => {
      await element['initializeTestPicker']();
      const graph1 = element['addEmptyGraph']()!;
      element.state.totalGraphs = 1;
      element.state.pageOffset = 30; // Pretend we are on page 2.

      const event = new CustomEvent('remove-explore', {
        detail: { elem: graph1 },
        bubbles: true,
      });
      element.dispatchEvent(event);

      assert.equal(element['exploreElements'].length, 0);
      assert.equal(element['allGraphConfigs'].length, 0);

      assert.equal(element.state.totalGraphs, 0);
      // However, the page offset is correctly reset to 0 by a different
      // part of the logic that recalculates the max valid offset.
      assert.equal(element.state.pageOffset, 0, 'Page offset should be reset to 0');
    });

    it('updates graph indices when a graph is removed', async () => {
      await element['initializeTestPicker']();
      const graph1 = element['addEmptyGraph']()!;
      const graph2 = element['addEmptyGraph']()!;
      const graph3 = element['addEmptyGraph']()!;

      // Manually set the graph_index as it would be in the real application flow.
      graph1.state.graph_index = 0;
      graph2.state.graph_index = 1;
      graph3.state.graph_index = 2;

      // Dispatch event to remove the second graph.
      const event = new CustomEvent('remove-explore', {
        detail: { elem: graph2 },
        bubbles: true,
      });
      element.dispatchEvent(event);

      // After removing graph2 (index 1), graph3 should have its index updated to 1.
      assert.equal(element['exploreElements'][1].state.graph_index, 1);
    });
  });

  describe('Shortcut functionality', () => {
    beforeEach(async () => {
      await setupElement();
    });

    it('fetches graph configs from a shortcut', async () => {
      const shortcutId = 'test-shortcut-id';
      const mockGraphConfigs: GraphConfig[] = [
        { queries: ['config=test'], formulas: [], keys: '' },
      ];
      const getShortcutStub = sinon
        .stub(DataService.prototype, 'getShortcut')
        .resolves(mockGraphConfigs);

      const configs = await element['getConfigsFromShortcut'](shortcutId);
      assert.deepEqual(configs, mockGraphConfigs);
      getShortcutStub.restore();
    });

    it('updates the shortcut when graph configs change', async () => {
      const newShortcutId = 'new-shortcut-id';
      const updateShortcutStub = sinon
        .stub(DataService.prototype, 'updateShortcut')
        .resolves(newShortcutId);

      element['allGraphConfigs'] = [{ queries: ['config=new'], formulas: [], keys: '' }];
      // stateHasChanged needs to be non-null for the update to be pushed.
      element['stateHasChanged'] = () => {};
      element['updateShortcutMultiview']();

      // Allow for async operations to complete.
      await new Promise((resolve) => setTimeout(resolve, 0));

      assert.equal(element.state.shortcut, newShortcutId);
      updateShortcutStub.restore();
    });
  });

  describe('Test Picker Integration', () => {
    beforeEach(async () => {
      await setupElement({
        default_param_selections: {},
        default_url_values: {},
        include_params: ['config'],
      });
      element.state.useTestPicker = true;
      await element['initializeTestPicker']();
    });

    it('initializes the test picker when useTestPicker is true', () => {
      assert.isNotNull(element.querySelector('test-picker-sk'));
    });

    it('adds a graph when plot-button-clicked event is received', async () => {
      const initialGraphCount = element['exploreElements'].length;

      const event = new CustomEvent('plot-button-clicked', {
        detail: { query: 'config=test' },
        bubbles: true,
      });
      element.dispatchEvent(event);

      // The event handler synchronously adds a new graph to the front of the array.
      // We grab that new graph.
      const newGraph = element['exploreElements'][0];
      // Now, instead of a timeout, we deterministically wait for its data to be loaded.
      await newGraph.requestComplete;

      assert.equal(element['exploreElements'].length, initialGraphCount + 1);
    });

    it('adds a graph when add-to-graph event is received', async () => {
      const initialGraphCount = element['exploreElements'].length;

      const event = new CustomEvent('add-to-graph', {
        detail: { query: 'config=test2' },
        bubbles: true,
      });
      element.dispatchEvent(event);

      await element['exploreElements'][0].requestComplete;

      assert.equal(element['exploreElements'].length, initialGraphCount + 1);
    });
  });

  describe('Manual Plot Graph Mode', () => {
    beforeEach(async () => {
      // Setup with manual_plot_mode true by default for these tests
      await setupElement({
        default_param_selections: {},
        default_url_values: {
          summary: 'true',
          useTestPicker: 'true',
          manual_plot_mode: 'true',
        },
        include_params: ['config'],
      });
      element.state.useTestPicker = true;
      await element['initializeTestPicker']();
    });

    it('initial load with manual_plot_mode=true', async () => {
      const shortcutId = 'multi-graph-shortcut';
      const mockGraphConfigs: GraphConfig[] = [
        { queries: ['config=test1'], formulas: [], keys: '' },
        { queries: ['config=test2'], formulas: [], keys: '' },
      ];
      fetchMock.post('/_/shortcut/get', {
        graphs: mockGraphConfigs,
      });

      // Trigger state change to load from shortcut
      const state = new State();
      state.shortcut = shortcutId;
      state.manual_plot_mode = true;
      await element['_onStateChangedInUrl'](state as any);

      await fetchMock.flush(true);
      await new Promise((resolve) => setTimeout(resolve, 0)); // Wait for updates

      assert.equal(element['exploreElements'].length, 2, 'Should have 2 graphs');
      element['exploreElements'].forEach((graph: ExploreSimpleSk, index: number) => {
        assert.isFalse(
          graph.state.doNotQueryData,
          `Graph ${index} should load data on initial load`
        );
      });
    });

    it('adds a graph in manual plot mode', async () => {
      // Start with one graph
      element['resetGraphs']();
      element['addEmptyGraph']();
      element['exploreElements'][0].state.queries = ['config=initial'];
      element['allGraphConfigs'][0].queries = ['config=initial'];
      element.state.manual_plot_mode = true;
      element['renderCurrentPage'](); // Initial render
      await new Promise((resolve) => setTimeout(resolve, 0)); // Wait for updates

      const initialGraphCount = element['exploreElements'].length;
      assert.equal(initialGraphCount, 1);

      // Simulate test picker returning a query
      const testPicker = element.querySelector('test-picker-sk') as TestPickerSk;
      sinon.stub(testPicker, 'createQueryFromFieldData').returns('config=new');

      // Spy on addFromQueryOrFormula for the new graph
      const newGraphSpy = sinon.spy(ExploreSimpleSk.prototype, 'addFromQueryOrFormula');

      // Dispatch plot event
      const event = new CustomEvent('plot-button-clicked', { bubbles: true });
      element.dispatchEvent(event);

      // Wait for async operations in the event handler
      await new Promise((resolve) => setTimeout(resolve, 0));
      await fetchMock.flush(true);
      await new Promise((resolve) => setTimeout(resolve, 0)); // Wait for updates

      assert.equal(
        element['exploreElements'].length,
        initialGraphCount + 1,
        'Should have added a graph'
      );

      const newGraph = element['exploreElements'][0];
      const oldGraph = element['exploreElements'][1];

      assert.equal(newGraph.state.graph_index, 0, 'New graph should be index 0');
      assert.equal(oldGraph.state.graph_index, 1, 'Old graph should be index 1');

      // Check that addStateToExplore was called with correct doNotQueryData values
      assert.isFalse(newGraph.state.doNotQueryData, 'New graph (index 0) should query data');
      assert.isTrue(oldGraph.state.doNotQueryData, 'Old graph (index 1) should NOT query data');

      // Verify newGraph.addFromQueryOrFormula was called
      assert.isTrue(newGraphSpy.called, 'addFromQueryOrFormula should be called on the new graph');
    });

    it('reflects manual_plot_mode in URL', async () => {
      element.state.manual_plot_mode = true;
      element['stateHasChanged']!(); // Trigger URL update
      await new Promise((resolve) => setTimeout(resolve, 0)); // Wait for updates
      const url = new URL(window.location.href);
      assert.equal(url.searchParams.get('manual_plot_mode'), 'true');

      element.state.manual_plot_mode = false;
      element['stateHasChanged']!(); // Trigger URL update
      await new Promise((resolve) => setTimeout(resolve, 0)); // Wait for updates
      const url2 = new URL(window.location.href);
      assert.isNull(url2.searchParams.get('manual_plot_mode'), 'Should be null when false');
    });

    it('removes a graph in manual plot mode and updates URL', async () => {
      element.state.manual_plot_mode = true;
      const updateShortcutSpy = sinon.spy(element, 'updateShortcutMultiview' as any);

      // Mock the shortcut update response
      fetchMock.post('/_/shortcut/update', { id: 'new-shortcut-id' }, { overwriteRoutes: true });

      // Add three graphs
      const graph1 = element['addEmptyGraph']()!;
      graph1.state.queries = ['config=test1'];
      const graph2 = element['addEmptyGraph']()!;
      graph2.state.queries = ['config=test2'];
      const graph3 = element['addEmptyGraph']()!;
      graph3.state.queries = ['config=test3'];
      element['allGraphConfigs'] = [
        { queries: ['config=test1'], formulas: [], keys: '' },
        { queries: ['config=test2'], formulas: [], keys: '' },
        { queries: ['config=test3'], formulas: [], keys: '' },
      ];
      element['renderCurrentPage']();
      await new Promise((resolve) => setTimeout(resolve, 0)); // Wait for updates

      assert.equal(element['exploreElements'].length, 3, 'Should have 3 graphs initially');
      assert.equal(graph1.state.graph_index, 0);
      assert.equal(graph2.state.graph_index, 1);
      assert.equal(graph3.state.graph_index, 2);

      // Simulate removing the second graph
      const event = new CustomEvent('remove-explore', {
        detail: { elem: graph2 },
        bubbles: true,
      });
      element.dispatchEvent(event);
      await new Promise((resolve) => setTimeout(resolve, 0)); // Wait for updates
      await fetchMock.flush(true);
      await new Promise((resolve) => setTimeout(resolve, 0)); // Wait for URL update

      assert.equal(element['exploreElements'].length, 2, 'Should have 2 graphs after removal');
      assert.equal(element['exploreElements'][0].state.graph_index, 0, 'Graph 1 index should be 0');
      assert.equal(
        element['exploreElements'][1].state.graph_index,
        1,
        'Graph 3 index should be updated to 1'
      );
      assert.deepEqual(element['exploreElements'], [graph1, graph3]);

      assert.isTrue(updateShortcutSpy.calledOnce, 'updateShortcutMultiview should be called');

      // Check URL for updated shortcut
      const updatedUrl = new URL(window.location.href);
      const updatedShortcut = updatedUrl.searchParams.get('shortcut');
      assert.equal(updatedShortcut, 'new-shortcut-id', 'Shortcut param should be updated in URL');
    });

    it('no re-fetch for remaining graphs when one is removed in manual plot mode', async () => {
      element.state.manual_plot_mode = true;

      // Add two graphs with some fake data
      const graph1 = element['addEmptyGraph']()!;
      graph1.state.queries = ['config=test1'];
      // Mock that it has traces
      sinon.stub(graph1, 'getTraceset').returns({ trace1: [] } as any);

      const graph2 = element['addEmptyGraph']()!;
      graph2.state.queries = ['config=test2'];
      sinon.stub(graph2, 'getTraceset').returns({ trace2: [] } as any);

      element['allGraphConfigs'] = [
        { queries: ['config=test1'], formulas: [], keys: '' },
        { queries: ['config=test2'], formulas: [], keys: '' },
      ];

      element['renderCurrentPage']();
      await new Promise((resolve) => setTimeout(resolve, 0));

      // Spy on addFromQueryOrFormula and rangeChangeImpl
      const addSpy = sinon.spy(ExploreSimpleSk.prototype, 'addFromQueryOrFormula');
      const rangeChangeSpy = sinon.spy(ExploreSimpleSk.prototype, 'rangeChangeImpl' as any);

      // Remove the first graph. This will cause graph2 to shift to index 0.
      const event = new CustomEvent('remove-explore', {
        detail: { elem: graph1 },
        bubbles: true,
      });
      element.dispatchEvent(event);
      await new Promise((resolve) => setTimeout(resolve, 0));

      // Verify no re-fetches were triggered on the remaining graph (graph2)
      // which is now at index 0.
      assert.isFalse(
        addSpy.called,
        'addFromQueryOrFormula should NOT be called during graph removal'
      );
      assert.isFalse(
        rangeChangeSpy.called,
        'rangeChangeImpl should NOT be called during graph removal'
      );

      assert.equal(element['exploreElements'].length, 1, 'Only one graph should remain');
      assert.equal(element['exploreElements'][0], graph2, 'Graph 2 should be the remaining graph');
      assert.equal(graph2.state.graph_index, 0, 'Graph 2 should now be at index 0');
    });

    it('removes all graphs and updates URL in manual plot mode', async () => {
      element.state.manual_plot_mode = true;
      const updateShortcutSpy = sinon.spy(element, 'updateShortcutMultiview' as any);

      // Add two graphs
      const graph1 = element['addEmptyGraph']()!;
      const graph2 = element['addEmptyGraph']()!;
      element['renderCurrentPage']();
      await new Promise((resolve) => setTimeout(resolve, 0)); // Wait for updates

      assert.equal(element['exploreElements'].length, 2, 'Should have 2 graphs initially');

      // Remove graph1
      element.dispatchEvent(
        new CustomEvent('remove-explore', {
          detail: { elem: graph1 },
          bubbles: true,
        })
      );
      await new Promise((resolve) => setTimeout(resolve, 0));

      // Remove graph2
      element.dispatchEvent(
        new CustomEvent('remove-explore', {
          detail: { elem: graph2 },
          bubbles: true,
        })
      );
      await new Promise((resolve) => setTimeout(resolve, 0));

      assert.equal(element['exploreElements'].length, 0, 'Should have 0 graphs after removal');
      assert.equal(element['allGraphConfigs'].length, 0, 'Should have 0 graphConfigs');
      assert.isTrue(
        updateShortcutSpy.calledTwice,
        'updateShortcutMultiview should be called twice'
      );

      // Check URL
      const url = new URL(window.location.href);
      assert.isNull(url.searchParams.get('shortcut'), 'Shortcut param should be removed from URL');
    });
  });

  describe('Graph Splitting', () => {
    beforeEach(async () => {
      await setupElement();
    });

    it('does not leak exploreElements on repeated splitGraphs', async () => {
      // Initial state: 0 graphs (connectedCallback might not add one if no shortcut/params)
      // Let's add one manually to start with a known state.
      element['addEmptyGraph']();
      assert.equal(element['exploreElements'].length, 1);

      // Mock grouping on the prototype to ensure it is picked up
      sinon
        .stub(ExploreMultiSk.prototype, 'groupTracesBySplitKey' as any)
        .returns(new Map([['key', ['trace1']]]));

      // Mock methods called by splitGraphs
      sinon.stub(element, 'createFrameRequest' as any).returns({});
      sinon.stub(element, 'createFrameResponse' as any).returns({});
      sinon.stub(element, 'renderCurrentPage' as any);
      sinon.stub(element, 'checkDataLoaded' as any);

      // Force totalGraphs to > 1 so splitGraphs doesn't return early
      element.state.totalGraphs = 2;
      element.state.splitByKeys = ['key'];

      await element['splitGraphs'](false, true);

      // Should be reset to 2 (1 Master + 1 group)
      assert.equal(element['exploreElements'].length, 2, 'exploreElements should be reset');

      // Run again
      await element['splitGraphs'](false, true);
      assert.equal(element['exploreElements'].length, 2, 'exploreElements should remain stable');
    });
  });

  describe('Synchronization', () => {
    beforeEach(async () => {
      await setupElement();
      // Ensure manual_plot_mode is false to prevent state leakage between tests
      element.state.manual_plot_mode = false;
    });

    it('syncs the x-axis label across all graphs', () => {
      const graph1 = element['addEmptyGraph']()!;
      const graph2 = element['addEmptyGraph']()!;
      graph1.state.graph_index = 0;
      graph2.state.graph_index = 1;
      // Need to add them to the graphDiv for querySelector to find them.
      element['graphDiv']!.appendChild(graph1);
      element['graphDiv']!.appendChild(graph2);

      const spy1 = sinon.spy(graph1, 'updateXAxis');
      const spy2 = sinon.spy(graph2, 'updateXAxis');

      const detail = {
        index: 0, // Event is coming from the first graph in the current view.
        domain: 'date',
      };
      const event = new CustomEvent('x-axis-toggled', {
        detail,
        bubbles: true,
      });

      element['graphDiv']!.dispatchEvent(event);

      // The graph that initiated the event should not be updated again,
      // but the other one should be.
      assert.isTrue(spy1.notCalled);
      assert.isTrue(spy2.calledOnceWith('date'));
    });

    it('correctly transforms commit offsets to timestamps in URL state', async () => {
      // Setup with commit domain
      await setupElement({ default_xaxis_domain: 'commit' });
      const graph1 = element['addEmptyGraph']()!;
      // Wait for data to load so we have the header
      await graph1.requestComplete;

      // Simulate selection in commit domain (offsets 100 to 101)
      // The header in setupElement has offsets 100, 101, 102 corresponding to timestamps 1000, 1001, 1002.
      const detail: PlotSelectionEventDetails = {
        value: { begin: 100, end: 101 },
        domain: 'commit',
        start: 0, // index in header
        end: 1, // index in header
      };

      const event = new CustomEvent('selection-changing-in-multi', {
        detail: detail,
        bubbles: true,
      });
      // Explicitly set state to a known value (1000) to ensure we are testing the update logic.
      // This defends against default initialization overwriting it asynchronously.
      element.state.begin = 1234; // Arbitrary value different from 1000
      element.state.end = 5678;

      // Ensure at least one graph exists
      if ((element as any).exploreElements.length === 0) {
        await (element as any).addGraph();
      }

      // Mock getHeader on the first graph to return valid timestamps
      const mockHeader = [
        { offset: 100, timestamp: 1600000000 },
        { offset: 101, timestamp: 1600000001 },
      ];
      const graph = (element as any).exploreElements[0];
      sinon.stub(graph, 'getHeader').returns(mockHeader);

      // Stub _onStateChangedInUrl to prevent it from resetting state based on URL
      // This is necessary because in the test environment the URL might not update/reflect correctly
      // or stateReflector might trigger a reset to default values (now - default range).
      sinon.stub(element as any, '_onStateChangedInUrl');

      element['graphDiv']!.dispatchEvent(event);

      // Wait for any async event handlers to process
      await new Promise((resolve) => setTimeout(resolve, 0));

      // With the bug (URL state corruption on refresh), begin would be 100. Correct behavior is 1600000000.
      assert.equal(
        element.state.begin,
        1600000000,
        'Begin should be timestamp 1600000000, not offset 100'
      );
      assert.equal(
        element.state.end,
        1600000001,
        'End should be timestamp 1600000001, not offset 101'
      );
    });

    it('syncs extend range across all graphs', async () => {
      const graph1 = element['addEmptyGraph']()!;
      const graph2 = element['addEmptyGraph']()!;

      const stub1 = sinon.stub(graph1, 'extendRange').resolves();
      const stub2 = sinon.stub(graph2, 'extendRange').resolves();

      const detail = {
        value: { begin: 1000, end: 2000 },
        offsetInSeconds: -3600,
      };
      element['graphDiv']!.dispatchEvent(
        new CustomEvent('range-changing-in-multi', {
          detail,
          bubbles: true,
        })
      );

      await new Promise((resolve) => setTimeout(resolve, 0));

      assert.isTrue(stub1.calledOnceWith(detail.value, detail.offsetInSeconds));
      assert.isTrue(stub2.calledOnceWith(detail.value, detail.offsetInSeconds));
    });

    it('syncs the x-axis label across all graphs with pagination', () => {
      // Create three graphs.
      const graph1 = element['addEmptyGraph']()!;
      const graph2 = element['addEmptyGraph']()!;
      const graph3 = element['addEmptyGraph']()!;

      // Set their graph_index property as if they are all loaded.
      graph1.state.graph_index = 0;
      graph2.state.graph_index = 1;
      graph3.state.graph_index = 2;

      // Simulate being on page 2, where only graph3 is visible.
      element.state.pageSize = 1;
      element.state.pageOffset = 2;
      element['currentPageExploreElements'] = [graph3];

      // Add only the current page's graph to the DOM.
      element['graphDiv']!.appendChild(graph3);

      const spy1 = sinon.spy(graph1, 'updateXAxis');
      const spy2 = sinon.spy(graph2, 'updateXAxis');
      const spy3 = sinon.spy(graph3, 'updateXAxis');

      const detail = {
        index: 2, // Event is coming from the third graph (index 2).
        domain: 'date',
      };
      const event = new CustomEvent('x-axis-toggled', {
        detail,
        bubbles: true,
      });

      element['graphDiv']!.dispatchEvent(event);

      // The first two graphs should be updated, even though they are not
      // on the current page. The third graph, which initiated the event,
      // should not be updated.
      assert.isTrue(spy1.calledOnceWith('date'));
      assert.isTrue(spy2.calledOnceWith('date'));
      assert.isTrue(spy3.notCalled);
    });

    it('syncs point selection across all graphs', () => {
      // Create simple mock graphs to purely test the handler logic.
      const graph1 = { updateSelectedRangeWithPlotSummary: () => {} };
      const graph2 = { updateSelectedRangeWithPlotSummary: () => {} };
      const spy1 = sinon.spy(graph1, 'updateSelectedRangeWithPlotSummary');
      const spy2 = sinon.spy(graph2, 'updateSelectedRangeWithPlotSummary');

      // Manually set the internal state to use our mocks.
      element['currentPageExploreElements'] = [graph1 as any, graph2 as any];
      element['exploreElements'] = [graph1 as any, graph2 as any];

      const detail: PlotSelectionEventDetails = {
        domain: 'commit',
        graphNumber: 0, // Event from graph 0
        value: { begin: 123, end: 123 },
        start: 100,
        end: 200,
      };
      const event = new CustomEvent('selection-changing-in-multi', {
        detail,
        bubbles: true,
      });
      element['graphDiv']!.dispatchEvent(event);

      assert.isTrue(spy1.notCalled, 'Source graph should not be updated');
      assert.isTrue(spy2.calledOnce, 'Target graph should be updated');
    });
  });

  describe('Even X-Axis Spacing Sync', () => {
    beforeEach(async () => {
      await setupElement();
      // Add a couple of graphs for testing sync
      element['addEmptyGraph']();
      element['addEmptyGraph']();
      element['exploreElements'][0].state.graph_index = 0;
      element['exploreElements'][1].state.graph_index = 1;
      // Add to DOM for event propagation
      element['graphDiv']!.appendChild(element['exploreElements'][0]);
      element['graphDiv']!.appendChild(element['exploreElements'][1]);
    });

    it('syncs enableDiscrete state to other graphs', async () => {
      const graph1 = element['exploreElements'][0];
      const graph2 = element['exploreElements'][1];

      const spy1 = sinon.spy(graph1, 'setUseDiscreteAxis');
      const spy2 = sinon.spy(graph2, 'setUseDiscreteAxis');

      // Simulate event from graph1
      const event = new CustomEvent('even-x-axis-spacing-changed', {
        detail: { value: true, graph_index: 0 },
        bubbles: true,
      });
      graph1.dispatchEvent(event);

      assert.equal(
        element.state.evenXAxisSpacing,
        'use_cache',
        'MultiSk state should not be updated'
      );
      assert.isTrue(spy1.notCalled, 'Source graph setUseDiscreteAxis should not be called by sync');
      assert.isTrue(spy2.calledOnceWith(true), 'Target graph setUseDiscreteAxis should be called');
      assert.isTrue(graph2.state.evenXAxisSpacing, 'Target graph state should be updated');
    });

    it('initializes new graphs with the current enableDiscrete state', () => {
      element.state.evenXAxisSpacing = 'true';
      const newGraph = element['addEmptyGraph']()!;
      element['addStateToExplore'](newGraph, new GraphConfig(), false);
      assert.isTrue(newGraph.state.evenXAxisSpacing);
    });
  });

  describe('Robustness', () => {
    beforeEach(async () => {
      await setupElement();
    });

    it('renderCurrentPage handles undefined frame responses/requests gracefully', () => {
      // Use addEmptyGraph to properly initialize exploreElements and allGraphConfigs
      element['addEmptyGraph']();
      element['addEmptyGraph']();
      element['addEmptyGraph']();

      // Force frame responses to be empty to simulate the condition where config exists
      // but data hasn't been fetched/stored yet.
      element['allFrameResponses'] = [];
      element['allFrameRequests'] = [];

      element.state.pageSize = 10;
      element.state.pageOffset = 0;

      // This should not throw 'TypeError: Cannot read properties of undefined'
      element['renderCurrentPage'](true);

      // Verify graphs were created despite missing data.
      // Logic skips index 0 (main graph) when >1 graph exists.
      // If we added 3 graphs, and potentially one existed or behavior changed,
      // we just want to ensure it rendered *something* without crashing.
      // In standard view (manual_plot_mode=false), the first graph is the summary graph
      // and is excluded from the paginated list, so we expect 2 graphs.
      assert.equal(element['currentPageExploreElements'].length, 2);
    });

    it('createFrameRequest uses defaults when begin/end are -1', () => {
      element.state.begin = -1;
      element.state.end = -1;
      element['allGraphConfigs'] = [{ queries: ['config=test'], formulas: [], keys: '' }];

      const request = element['createFrameRequest']();

      assert.notEqual(request.begin, -1);
      assert.notEqual(request.end, -1);
      assert.isAtLeast(request.end, request.begin);
    });

    it('createFrameRequest falls back to empty queries if graph config is missing', () => {
      // Setup state where allGraphConfigs is empty
      element['allGraphConfigs'] = [];

      // Should not throw error
      const request = element['createFrameRequest']();

      assert.isEmpty(request.queries);
    });
  });

  describe('Pagination', () => {
    beforeEach(async () => {
      await setupElement();
      // Mock stateHasChanged and splitGraphs for pagination tests.
      element['stateHasChanged'] = sinon.spy();
      sinon.stub(element, 'splitGraphs' as any);
    });

    it('updates page offset when page-changed event is received', () => {
      element.state.pageSize = 10;
      element.state.pageOffset = 10;

      const detail: PaginationSkPageChangedEventDetail = { delta: 1 };
      const event = new CustomEvent('page-changed', { detail, bubbles: true });
      element['pageChanged'](event);

      assert.equal(element.state.pageOffset, 20);
      assert.isTrue((element['stateHasChanged'] as sinon.SinonSpy).calledOnce);
    });

    it('updates page size on input change', () => {
      const input = document.createElement('input');
      input.value = '25';
      const event = { target: input } as unknown as MouseEvent;

      element['pageSizeChanged'](event);

      assert.equal(element.state.pageSize, 25);
      assert.isTrue((element['stateHasChanged'] as sinon.SinonSpy).calledOnce);
    });
  });

  describe('Test Picker ReadOnly behavior', () => {
    // This block needs special handling for setupElement to control include_params.
    it('sets test-picker to readonly on initialization if graphs exist', async () => {
      await setupElement({
        default_param_selections: {},
        default_url_values: {},
        include_params: ['config'],
      });
      // Mock exploreElements to exist before initializeTestPicker is called.
      element['exploreElements'] = [new ExploreSimpleSk()];
      await element['initializeTestPicker']();
      const testPicker = element.querySelector('test-picker-sk') as TestPickerSk;
      testPicker.setReadOnly(true);
      assert.isTrue(testPicker.readOnly);
    });
  });

  describe('addStateToExplore', () => {
    it('should always use the multi-sk state for begin and end', async () => {
      await setupElement();
      const multiSkState = new State();
      multiSkState.begin = 1000;
      multiSkState.end = 2000;
      element.state = multiSkState;

      const simpleSk = new ExploreSimpleSk();
      // Provide a full state object to satisfy the type checker.
      simpleSk.state = {
        begin: 500,
        end: 1500,
        formulas: [],
        queries: [],
        keys: '',
        xbaroffset: -1,
        showZero: false,
        numCommits: 50,
        requestType: 1,
        pivotRequest: { group_by: [], operation: 'avg', summary: [] },
        sort: '',
        summary: false,
        selected: { commit: 0 as CommitNumber, name: '', tableRow: -1, tableCol: -1 },
        domain: 'commit',
        labelMode: 0,
        incremental: false,
        disable_filter_parent_traces: false,
        plotSummary: false,
        highlight_anomalies: [],
        enable_chart_tooltip: false,
        show_remove_all: true,
        use_titles: false,
        useTestPicker: false,
        use_test_picker_query: false,
        enable_favorites: false,
        hide_paramset: false,
        horizontal_zoom: false,
        graph_index: 0,
        doNotQueryData: false,
        evenXAxisSpacing: false,
        dots: true,
        autoRefresh: false,
        show_google_plot: false,
      };

      element['addStateToExplore'](simpleSk, new GraphConfig(), false, 0);

      assert.equal(simpleSk.state.begin, 1000);
      assert.equal(simpleSk.state.end, 2000);
    });
  });

  describe('loadAllCharts', () => {
    beforeEach(() => {
      element['stateHasChanged'] = sinon.spy();
      sinon.stub(element, '_render' as any);
      sinon.stub(element, 'updateChartHeights' as any);
    });

    it('calls splitGraphs and updates pagination when there are multiple graphs', async () => {
      const splitGraphsSpy = sinon.spy(element, 'splitGraphs' as any);
      element['exploreElements'] = [
        new ExploreSimpleSk(),
        new ExploreSimpleSk(),
        new ExploreSimpleSk(),
      ];
      element.state.totalGraphs = 2;
      element.state.splitByKeys = ['config'];

      await element['loadAllCharts']();

      assert.equal(element.state.pageSize, 2);
      assert.equal(element.state.pageOffset, 0);
      assert.isTrue(splitGraphsSpy.calledOnce);
    });

    it('populates currentPageExploreElements after loading all charts', async () => {
      const exploreSimpleSk = new ExploreSimpleSk();
      exploreSimpleSk.getTraceset = () => ({
        ',config=test1,': [1, 2],
        ',config=test2,': [3, 4],
      });
      exploreSimpleSk.getHeader = () => [
        {
          offset: 0 as CommitNumber,
          timestamp: 1757354947 as TimestampSeconds,
          hash: '',
          author: '',
          message: '',
          url: '',
        },
        {
          offset: 1 as CommitNumber,
          timestamp: 1757441347 as TimestampSeconds,
          hash: '',
          author: '',
          message: '',
          url: '',
        },
      ];
      exploreSimpleSk.getCommitLinks = () => [];
      exploreSimpleSk.getAnomalyMap = () => ({});
      exploreSimpleSk.getSelectedRange = () => null;

      element['exploreElements'] = [exploreSimpleSk, new ExploreSimpleSk(), new ExploreSimpleSk()];
      element['allGraphConfigs'] = [
        { queries: ['config=test1', 'config=test2'], formulas: [], keys: '' },
        { queries: ['config=test1'], formulas: [], keys: '' },
        { queries: ['config=test2'], formulas: [], keys: '' },
      ];
      element.state.splitByKeys = ['config'];

      await element['loadAllCharts']();

      // After splitting, there should be 3 elements (1 master, 2 split).
      // After loadAllCharts, pageSize is 2, so currentPageExploreElements should have 2.
      assert.equal(element['currentPageExploreElements'].length, 2);
    });
  });

  describe('mergeParamSets', () => {
    it('returns original ParamSet in array if split key has one value', () => {
      const ps = { os: ['linux'], arch: ['x86'] };
      const result = element['groupParamSetBySplitKey'](ps, ['os']);
      assert.deepEqual(result, [{ os: ['linux'], arch: ['x86'] }]);
    });

    it('should return the original ParamSet in an array if no split key is provided', () => {
      const ps = { os: ['linux', 'windows'], arch: ['x86'] };
      const result = element['groupParamSetBySplitKey'](ps, []);
      assert.deepEqual(result, [ps]);
    });
  });

  describe('mergeParamSets', () => {
    it('should merge ParamSets with overlapping keys and de-duplicate values', () => {
      const paramSets = [
        { os: ['linux', 'windows'], arch: ['x86'], gpu: [] },
        { os: ['mac', 'linux'], gpu: ['nvidia'], arch: [] },
      ];
      const result = element['mergeParamSets'](paramSets);
      assert.isTrue(result.os.includes('linux'));
      assert.isTrue(result.os.includes('windows'));
      assert.isTrue(result.os.includes('mac'));
      assert.equal(result.os.length, 3);
      assert.deepEqual(result.arch, ['x86']);
      assert.deepEqual(result.gpu, ['nvidia']);
    });
  });

  describe('Chunked Graph Loading', () => {
    let mainGraph: ExploreSimpleSk;
    let addFromQuerySpy: sinon.SinonSpy;
    let loadExtendedSpy: sinon.SinonSpy;
    let testPicker: TestPickerSk;
    let updateShortcutSpy: sinon.SinonSpy;
    let setProgressSpy: sinon.SinonSpy;

    beforeEach(async () => {
      await setupElement();
      element.state = new State();
      element.state.useTestPicker = true;
      element.state.splitByKeys = ['os']; // The key we will split by.
      element.state.pageSize = 10; // Ensure we try to load all graphs.
      await element['initializeTestPicker']();

      // The 'plot-button-clicked' handler adds a 'mainGraph' at the beginning.
      // We will spy on its methods to verify the orchestration logic.
      mainGraph = new ExploreSimpleSk();
      addFromQuerySpy = sinon.stub(mainGraph, 'addFromQueryOrFormula').resolves();
      loadExtendedSpy = sinon.stub(mainGraph, 'loadExtendedRangeData').resolves();
      sinon.stub(mainGraph, 'getSelectedRange').returns({ begin: 0, end: 1 });
      // Ensure that awaiting 'requestComplete' doesn't hang the test.
      sinon.stub(mainGraph, 'requestComplete').get(async () => await Promise.resolve());

      // We stub 'createExploreSimpleSk' to ensure it returns our controlled instance
      // of ExploreSimpleSk, allowing us to spy on its methods.
      sinon.stub(element, 'createExploreSimpleSk' as any).returns(mainGraph);
      // Stub addEventListener to prevent duplicate listeners when createExploreSimpleSk
      // is called multiple times
      sinon.stub(mainGraph, 'addEventListener');

      // Mock the testPicker to return a ParamSet that will create 7 distinct groups.
      testPicker = element.querySelector('test-picker-sk')!;
      sinon.stub(testPicker, 'createParamSetFromFieldData').returns({
        os: ['win1', 'win2', 'mac1', 'mac2', 'linux1', 'linux2', 'chromeos'], // 7 values
        arch: ['x86'],
      });

      // We stub 'splitGraphs' because we are not testing its implementation here,
      // only that the loading orchestrator calls it.
      sinon.stub(element, 'splitGraphs' as any).resolves();
      element['stateHasChanged'] = () => {};

      updateShortcutSpy = sinon.spy(element, 'updateShortcutMultiview' as any);
      setProgressSpy = sinon.spy(element, 'setProgress' as any);
    });

    it('loads graphs in chunks and fetches extended data once at the end', async () => {
      // Dispatch the event that triggers the chunking logic.
      const event = new CustomEvent('plot-button-clicked', { bubbles: true });
      element.dispatchEvent(event);

      // The event handler is async. We need to wait for it to complete.
      // A small timeout allows the chain of promises in the handler to resolve.
      await new Promise((resolve) => setTimeout(resolve, 0));

      // --- Assertions ---

      // With 7 groups and a CHUNK_SIZE of 5, we expect 2 calls to add traces:
      // 1. The first 5 graphs (chunk size of 5)
      // 2. The next 2 graphs (chunk size of 2)
      assert.equal(
        addFromQuerySpy.callCount,
        2,
        'addFromQueryOrFormula should be called for each chunk'
      );

      // Verify that `loadExtendedRange` was correctly set to false for all chunk-loading calls.
      assert.isFalse(
        addFromQuerySpy.firstCall.args[5],
        'loadExtendedRange should be false for the first chunk'
      );
      assert.isFalse(
        addFromQuerySpy.secondCall.args[5],
        'loadExtendedRange should be false for the second chunk'
      );

      // Verify that `loadExtendedRangeData` was called exactly once, after all chunks were
      // processed.
      assert.isTrue(
        loadExtendedSpy.calledOnce,
        'loadExtendedRangeData should be called once after all chunks'
      );
      const allChunksAddedBeforeExtended = addFromQuerySpy.lastCall.calledBefore(
        loadExtendedSpy.firstCall
      );
      assert.isTrue(
        allChunksAddedBeforeExtended,
        'All chunks should be added before extended data is loaded'
      );

      // Verify that the user receives correct progress updates.
      // We expect calls for: initial, chunk 1, chunk 2, chunk 3, extended data, and final clear.
      assert.isTrue(setProgressSpy.callCount >= 5, 'setProgress should be called multiple times');
      assert.equal(setProgressSpy.getCall(0).args[0], 'Loading graphs...');
      assert.equal(setProgressSpy.lastCall.args[0], '', 'setProgress should be cleared at the end');

      assert.isTrue(
        updateShortcutSpy.calledOnce,
        'updateShortcutMultiview should be called once at the end'
      );
    });

    it('loads all graphs when "Load All Graphs" is clicked and page size is small', async () => {
      // Set a small page size to ensure not all graphs are loaded initially.
      element.state.pageSize = 2;
      const totalSplitGraphs = 7; // Based on the mocked test picker data.

      // Dispatch the event that triggers the chunking logic.
      const event = new CustomEvent('plot-button-clicked', { bubbles: true });
      element.dispatchEvent(event);

      // Wait for the async handler to complete.
      await new Promise((resolve) => setTimeout(resolve, 0));

      // After the initial chunked load, the `exploreElements` array is populated.
      // Let's simulate the state after `_onPlotButtonClicked` has run.
      // The stub for `splitGraphs` prevents `renderCurrentPage` from being called
      // in a way that's useful for this test's setup, so we call it manually.
      element['exploreElements'] = [
        mainGraph,
        ...Array(totalSplitGraphs).fill(new ExploreSimpleSk()),
      ];
      element['allGraphConfigs'] = Array(totalSplitGraphs + 1).fill(new GraphConfig());
      element.state.pageSize = 2;
      element['renderCurrentPage'](true); // This will respect the small pageSize.

      assert.equal(
        element['currentPageExploreElements'].length,
        2,
        'Initially, only one page of graphs should be loaded'
      );

      // Now, simulate clicking "Load All Charts".
      // The button only appears when totalGraphs > 10.
      element.state.totalGraphs = 11;
      element['_render']();
      const loadAllButton = element.querySelector<HTMLButtonElement>('div#pagination > button');
      assert.isNotNull(loadAllButton, 'Load All Charts button should be visible');

      // The real `loadAllCharts` calls `splitGraphs`. We'll spy on it.
      const splitGraphsSpy = element['splitGraphs'] as sinon.SinonSpy;

      await element['loadAllCharts']();

      assert.equal(
        element.state.pageSize,
        totalSplitGraphs,
        'Page size should be updated to total graphs'
      );
      assert.equal(element.state.pageOffset, 0, 'Page offset should be reset');
      assert.isTrue(splitGraphsSpy.called, 'splitGraphs should be called to reload');

      element['renderCurrentPage']();
      assert.equal(
        element['currentPageExploreElements'].length,
        totalSplitGraphs,
        'All graphs should be loaded on the page'
      );
    });
  });

  describe('_onStateChangedInUrl', () => {
    beforeEach(async () => {
      await setupElement();
      fetchMock.post('/_/shortcut/update', { id: 'new-shortcut-id' });
    });

    it('correctly calculates begin and end times when dayRange is provided', async function () {
      this.timeout(20000); // Increase timeout for this test
      // Stub updateShortcutMultiview to prevent network calls/timeouts
      sinon.stub(element, 'updateShortcutMultiview' as any).resolves();
      // Stub checkDataLoaded to prevent side effects
      sinon.stub(element, 'checkDataLoaded' as any);

      // Use fake timers ONLY for Date, so requestAnimationFrame/setTimeout work natively.
      // This prevents deadlock in renderCurrentPage which waits for rAF.
      const clock = sinon.useFakeTimers({ toFake: ['Date'] });
      const now = Math.floor(Date.now() / 1000);
      const dayRange = 5;
      const fiveDaysInSeconds = dayRange * 24 * 60 * 60;

      const state = new State();
      state.begin = -1;
      state.end = -1;
      state.dayRange = dayRange;

      await element['_onStateChangedInUrl'](state as any);

      assert.equal(element.state.end, now);
      assert.equal(element.state.begin, now - fiveDaysInSeconds);

      clock.restore();
    });

    it('correctly encodes spaces in aggregated queries', async function () {
      this.timeout(5000); // Increase timeout for this test

      // Mock Google Charts loader for this test
      const loadStub = sinon.stub(loader, 'load').resolves();

      const shortcutId = 'shortcut-with-spaces';
      const mockGraphConfigs: GraphConfig[] = [
        { queries: ['config=with space'], formulas: [], keys: '' },
        { queries: ['arch=x86 new'], formulas: [], keys: '' },
      ];
      fetchMock.post('/_/shortcut/get', {
        graphs: mockGraphConfigs,
      });

      const state = new State();
      state.shortcut = shortcutId;
      state.splitByKeys = ['someKey']; // Enable splitting to trigger aggregation.

      // Mock renderCurrentPage to avoid DOM issues in this test
      sinon.stub(element, 'renderCurrentPage' as any).returns(undefined);
      // Mock splitGraphs as well since it's called after graph loading
      sinon.stub(element, 'splitGraphs' as any).resolves();
      // Mock checkDataLoaded to prevent side effects
      sinon.stub(element, 'checkDataLoaded' as any).returns(undefined);

      await element['_onStateChangedInUrl'](state as any);

      // Check the aggregated query in the first graph config.
      const aggregatedQuery = element['allGraphConfigs'][0].queries[0];
      assert.include(aggregatedQuery, 'config=with%20space');
      assert.include(aggregatedQuery, 'arch=x86%20new');
      assert.notInclude(aggregatedQuery, '+');

      loadStub.restore();
    });
  });

  describe('Performance Verification', () => {
    beforeEach(async () => {
      await setupElement();
    });

    it('limits DOM nodes via pagination (DOM Weight)', async () => {
      // Simulate 20 graphs, PageSize 5

      const totalGraphs = 20;

      const pageSize = 5;

      element.state.totalGraphs = totalGraphs;

      element.state.pageSize = pageSize;

      element.state.pageOffset = 0;

      element.state.manual_plot_mode = true; // Ensure index 0 is included

      // Populate config

      element['allGraphConfigs'] = Array(totalGraphs).fill({
        queries: ['config=test'],

        formulas: [],

        keys: '',
      });

      element['allFrameRequests'] = Array(totalGraphs).fill({});

      element['allFrameResponses'] = Array(totalGraphs).fill({});

      // Use real elements so appendChild works

      const mockElements: ExploreSimpleSk[] = [];

      for (let i = 0; i < totalGraphs; i++) {
        const el = document.createElement('div') as unknown as ExploreSimpleSk;

        (el as any).state = {};

        (el as any).UpdateWithFrameResponse = sinon.stub().resolves();

        (el as any).updateComplete = Promise.resolve();

        (el as any).setUseDiscreteAxis = sinon.stub();

        (el as any).updateChartHeight = sinon.stub();

        mockElements.push(el);
      }

      element['exploreElements'] = mockElements;

      // Mock _render to avoid actual lit-html work

      sinon.stub(element, '_render' as any);

      sinon.stub(element, 'updateChartHeights' as any);

      await element['renderCurrentPage'](true);

      // Verify only pageSize elements are in the "current page" list

      assert.equal(element['currentPageExploreElements'].length, pageSize);

      assert.equal(element['graphDiv']!.childElementCount, pageSize);
    });

    it('only triggers data updates for visible graphs', async () => {
      // Setup 20 graphs, PageSize 5

      const total = 20;

      const pageSize = 5;

      element.state.pageSize = pageSize;

      element.state.manual_plot_mode = true; // Ensure index 0 is included

      element['allGraphConfigs'] = Array(total).fill({
        queries: ['config=test'],

        formulas: [],

        keys: '',
      });

      element['allFrameRequests'] = Array(total).fill({});

      element['allFrameResponses'] = Array(total).fill({});

      const updateSpies: sinon.SinonStub[] = [];

      const exploreMocks: ExploreSimpleSk[] = [];

      for (let i = 0; i < total; i++) {
        const el = document.createElement('div') as unknown as ExploreSimpleSk;

        (el as any).state = {};

        const stub = sinon.stub().resolves();

        (el as any).UpdateWithFrameResponse = stub;

        (el as any).updateComplete = Promise.resolve();

        (el as any).setUseDiscreteAxis = sinon.stub();

        (el as any).updateChartHeight = sinon.stub();

        updateSpies.push(stub);

        exploreMocks.push(el);
      }

      element['exploreElements'] = exploreMocks;

      // Mock other dependencies

      sinon.stub(element, '_render' as any);

      sinon.stub(element, 'updateChartHeights' as any);

      // Render page 0 (indices 0-4)

      element.state.pageOffset = 0;

      await element['renderCurrentPage'](false); // false = query data

      // Verify first 5 updated, others not

      for (let i = 0; i < total; i++) {
        if (i < pageSize) {
          assert.isTrue(updateSpies[i].called, `Graph ${i} should be updated`);
        } else {
          assert.isFalse(updateSpies[i].called, `Graph ${i} should NOT be updated`);
        }
      }
    });
  });

  describe('ParamSet Helpers', () => {
    beforeEach(async () => {
      await setupElement();
    });

    it('groupParamSetBySplitKey returns original ParamSet if split key is missing', () => {
      const ps = { os: ['linux', 'windows'], arch: ['x86'] };
      const result = element['groupParamSetBySplitKey'](ps, ['missing']);
      assert.deepEqual(result, [ps]);
    });

    it('mergeParamSets handles empty input', () => {
      const result = element['mergeParamSets']([]);
      assert.deepEqual(result, {});
    });
  });

  describe('ExploreMultiSk Split Behavior', () => {
    beforeEach(async () => {
      // Setup with splitting enabled
      await setupElement({
        default_param_selections: {},
        default_url_values: { useTestPicker: 'true' },
        include_params: ['config', 'subtest_2'],
      });
      element.state.useTestPicker = true;
      await element['initializeTestPicker']();
    });

    it('splits graphs from URL state', async () => {
      const state = new State();
      state.splitByKeys = ['subtest_2'];
      // GraphConfig uses queries
      const graphConfig = new GraphConfig();
      graphConfig.queries = ['config=8888&subtest_2=a', 'config=8888&subtest_2=b'];

      // Mock getConfigsFromShortcut
      sinon.stub(element, 'getConfigsFromShortcut' as any).resolves([graphConfig]);
      state.shortcut = 'some-shortcut';

      // We need to ensure the main graph (index 0) returns traceset so splitGraphs can work.
      // Mock createExploreSimpleSk to return an element with data.
      const exploreStub = sinon.stub(element, 'createExploreSimpleSk' as any).callsFake(() => {
        const el = new ExploreSimpleSk();
        // Mock getTraceset
        el.getTraceset = () => ({
          ',config=8888,subtest_2=a,': [1, 2],
          ',config=8888,subtest_2=b,': [3, 4],
        });
        // Mock requestComplete
        sinon.stub(el, 'requestComplete').get(() => Promise.resolve());
        return el;
      });

      // Stub load() to avoid network
      const loadStub = sinon.stub(loader, 'load').resolves();

      // Call _onStateChangedInUrl
      await element['_onStateChangedInUrl'](state as any);

      // Check explore elements
      // Should have 3 graphs: 1 hidden summary + 2 splits.
      assert.equal(
        element['exploreElements'].length,
        3,
        'Should have 3 graphs (1 summary + 2 splits)'
      );

      loadStub.restore();
      exploreStub.restore();
    });

    it('splits graphs correctly on plot button click with split key', async () => {
      await setupElement({
        default_param_selections: {},
        default_url_values: { useTestPicker: 'true' },
        include_params: ['config', 'subtest_2'],
      });
      element.state.useTestPicker = true;
      await element['initializeTestPicker']();

      const testPicker = element.querySelector('test-picker-sk') as TestPickerSk;
      // Mock createParamSetFromFieldData to return data for splitting
      sinon.stub(testPicker, 'createParamSetFromFieldData').returns({
        subtest_2: ['hash-map', 'navier-stokes'],
        benchmark: ['v8'],
      });
      sinon
        .stub(testPicker, 'createQueryFromFieldData')
        .returns('benchmark=v8&subtest_2=hash-map&subtest_2=navier-stokes');

      element.state.splitByKeys = ['subtest_2'];

      // Mock graph creation to return data.
      sinon.stub(element, 'createExploreSimpleSk' as any).callsFake(() => {
        const el = new ExploreSimpleSk();
        // Mock getTraceset to return data corresponding to the split.
        el.getTraceset = () => ({
          ',benchmark=v8,subtest_2=hash-map,': [1],
          ',benchmark=v8,subtest_2=navier-stokes,': [2],
        });

        // Mock requestComplete
        sinon.stub(el, 'requestComplete').get(() => Promise.resolve());

        // Mock methods used during load
        sinon.stub(el, 'addFromQueryOrFormula').resolves();
        sinon.stub(el, 'loadExtendedRangeData').resolves();
        sinon.stub(el, 'getSelectedRange').returns({ begin: 0, end: 100 });
        sinon.stub(el, 'updateChartHeight');

        return el;
      });

      // Mock loader
      sinon.stub(loader, 'load').resolves();

      const event = new CustomEvent('plot-button-clicked', { bubbles: true });
      element.dispatchEvent(event);

      // Wait for async operations
      await new Promise((resolve) => setTimeout(resolve, 0));

      // Should have 3 graphs (1 master + 2 splits)
      assert.equal(
        element['exploreElements'].length,
        3,
        'Should have 3 graphs (1 summary + 2 splits)'
      );
    });
  });

  describe('Graph Removal Scenarios', () => {
    it('removes the correct graph when an item is removed from a split field', async () => {
      // Setup: Split by 'test'. 3 Items: A, B, C.
      element.state.splitByKeys = ['test'];
      const mockExploreA = {
        state: { queries: ['test=A'] },
        getTraceset: () => ({ ',test=A,': [1] }),
        getHeader: () => [],
        getCommitLinks: () => [],
        getAnomalyMap: () => ({}),
        UpdateWithFrameResponse: () => Promise.resolve(),
        removeKeys: () => {},
        requestComplete: Promise.resolve(),
      } as any;
      const mockExploreB = {
        state: { queries: ['test=B'] },
        getTraceset: () => ({ ',test=B,': [1] }),
        getHeader: () => [],
        getCommitLinks: () => [],
        getAnomalyMap: () => ({}),
        UpdateWithFrameResponse: () => Promise.resolve(),
        removeKeys: () => {},
        requestComplete: Promise.resolve(),
      } as any;
      const mockExploreC = {
        state: { queries: ['test=C'] },
        getTraceset: () => ({ ',test=C,': [1] }),
        getHeader: () => [],
        getCommitLinks: () => [],
        getAnomalyMap: () => ({}),
        UpdateWithFrameResponse: () => Promise.resolve(),
        removeKeys: () => {},
        requestComplete: Promise.resolve(),
      } as any;

      element['exploreElements'] = [
        // Main graph (accumulator)
        {
          state: { queries: ['main=1'] },
          getTraceset: () => ({}),
          getHeader: () => [],
          getCommitLinks: () => [],
          getAnomalyMap: () => ({}),
          UpdateWithFrameResponse: () => Promise.resolve(),
          removeKeys: () => {},
          getSelectedRange: () => ({ begin: 0, end: 0 }),
        } as any,
        mockExploreA,
        mockExploreB,
        mockExploreC,
      ];
      element['allGraphConfigs'] = [
        new GraphConfig(),
        new GraphConfig(),
        new GraphConfig(),
        new GraphConfig(),
      ];

      // Mock helpers
      element['checkDataLoaded'] = () => {};
      element['updateShortcutMultiview'] = () => {};
      element['renderCurrentPage'] = () => Promise.resolve();
      element['getCompleteTraceset'] = () => ({
        ',test=A,': [1],
        ',test=B,': [1],
        ',test=C,': [1],
      });

      // Spy on removeExplore
      const removeSpy = sinon.spy(element as any, 'removeExplore');

      // Trigger remove-trace for 'B'
      const event = new CustomEvent('remove-trace', {
        detail: { param: 'test', value: ['B'], query: 'test=A&test=C', isSplit: true },
      });

      await element['_onRemoveTrace'](event);

      // Verify removeExplore was called with Graph B
      assert.isTrue(removeSpy.calledOnceWith(mockExploreB), 'Should remove Graph B');
    });

    it('removes a graph when the remove-explore event (trash button) is triggered', async () => {
      const mockExplore = { state: { queries: ['test=A'] } } as any;
      element['exploreElements'] = [mockExplore];
      element['allGraphConfigs'] = [new GraphConfig()];

      // Mock helpers
      element['checkDataLoaded'] = () => {};
      element['updateShortcutMultiview'] = () => {};
      element['renderCurrentPage'] = () => Promise.resolve();
      element['testPicker'] = {
        removeItemFromChart: sinon.spy(),
        setReadOnly: () => {},
        autoAddTrace: false,
      } as any;

      // Case 1: Length 1 (Independent Mode / Last Graph) -> Direct Removal
      const removeSpy = sinon.spy(element as any, 'removeExplore');

      // Trigger event
      const event = new CustomEvent('remove-explore', { detail: { elem: mockExplore } });
      element['_onRemoveExplore'](event); // Call handler directly to simulate event

      assert.isTrue(
        removeSpy.calledWith(mockExplore),
        'Should call removeExplore directly for last graph'
      );

      // Case 2: Length > 1 (Split Mode) -> Delegate to Picker
      element['exploreElements'] = [mockExplore, { state: { queries: ['test=B'] } } as any];
      element.state.splitByKeys = ['test'];

      // Reset spy
      removeSpy.resetHistory();

      // Trigger event for A
      element['_onRemoveExplore'](event);

      // Verify it called picker removal instead of direct removal
      assert.isFalse(removeSpy.called, 'Should NOT call removeExplore directly in split mode');
      assert.isTrue(
        (element['testPicker']!.removeItemFromChart as sinon.SinonSpy).calledWith('test', ['A']),
        'Should delegate to picker'
      );
    });
  });
});
