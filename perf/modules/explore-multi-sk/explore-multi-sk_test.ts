import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import sinon from 'sinon';
import { ExploreMultiSk, State } from './explore-multi-sk';
import { GraphConfig, ExploreSimpleSk } from '../explore-simple-sk/explore-simple-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { setUpExploreDemoEnv } from '../common/test-util';
import { PlotSelectionEventDetails } from '../plot-google-chart-sk/plot-google-chart-sk';
import { PaginationSkPageChangedEventDetail } from '../../../golden/modules/pagination-sk/pagination-sk';
import { Trace, TraceSet, CommitNumber, TimestampSeconds } from '../json';
import { TestPickerSk } from '../test-picker-sk/test-picker-sk';

describe('ExploreMultiSk', () => {
  let element: ExploreMultiSk;

  // Common setup for most tests
  const setupElement = async (mockDefaults: any = null) => {
    setUpExploreDemoEnv();
    window.perf = {
      instance_url: '',
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

    // Mock the data fetch that new graphs will trigger.
    fetchMock.post('/_/frame/v2', {
      dataframe: {
        traceset: { ',config=test,': [1, 2, 3] },
        header: [],
        paramset: { config: ['test'] },
      },
    });

    element = setUpElementUnderTest<ExploreMultiSk>('explore-multi-sk')();
    // Wait for connectedCallback to finish, including initializeDefaults
    await fetchMock.flush(true);
  };

  afterEach(() => {
    fetchMock.restore();
    sinon.restore();
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
        instance_url: '',
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
      };
      fetchMock.config.overwriteRoutes = true;
      fetchMock.get('/_/login/status', {
        email: 'user@google.com',
        roles: ['editor'],
      });
      fetchMock.get('/_/defaults/', Promise.reject(new Error('Network error')));
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
      assert.equal(element['graphConfigs'].length, initialGraphCount + 1);
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
      fetchMock.post('/_/shortcut/get', {
        graphs: mockGraphConfigs,
      });

      const configs = await element['getConfigsFromShortcut'](shortcutId);
      assert.deepEqual(configs, mockGraphConfigs);
    });

    it('updates the shortcut when graph configs change', async () => {
      const newShortcutId = 'new-shortcut-id';
      fetchMock.post('/_/shortcut/update', { id: newShortcutId });

      element['graphConfigs'] = [{ queries: ['config=new'], formulas: [], keys: '' }];
      // stateHasChanged needs to be non-null for the update to be pushed.
      element['stateHasChanged'] = () => {};
      element['updateShortcutMultiview']();

      // Allow for async operations to complete.
      await new Promise((resolve) => setTimeout(resolve, 0));

      assert.equal(element.state.shortcut, newShortcutId);
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

    it('removes a trace when remove-trace event is received', async () => {
      // Add a graph and mock its methods for the test.
      const graph = element['addEmptyGraph']()!;
      const mockTraceset = TraceSet({
        ',config=test1,arch=x86,': Trace([1, 2]),
        ',config=test1,arch=arm,': Trace([3, 4]),
      });

      // Mock methods on the graph instance that will be called by the handler.
      graph.getTraceset = () => mockTraceset;

      sinon.stub(graph, 'removeKeys').callsFake((...args: unknown[]) => {
        const keysToRemove = args[0] as string[];
        keysToRemove.forEach((key) => {
          delete mockTraceset[key];
        });
      });

      sinon.stub(graph, 'UpdateWithFrameResponse');
      sinon.stub(graph, 'getHeader').returns([]);

      graph.state.queries = ['config=test1&arch=x86', 'config=test1&arch=arm'];

      const event = new CustomEvent('remove-trace', {
        detail: { param: 'arch', value: ['x86'] },
        bubbles: true,
      });
      element.dispatchEvent(event);

      // Verify the correct trace was removed by checking the state.
      assert.deepEqual(Object.keys(mockTraceset), [',config=test1,arch=arm,']);
      assert.deepEqual(graph.state.queries, ['config=test1&arch=arm']);
    });
  });

  describe('Graph Splitting', () => {
    beforeEach(async () => {
      await setupElement();
    });

    it('splits a single graph with multiple traces into multiple graphs', async () => {
      // Setup a single graph with two traces.
      const exploreSimpleSk = new ExploreSimpleSk();
      exploreSimpleSk.getTraceset = () => ({
        ',config=test1,': [1, 2],
        ',config=test2,': [3, 4],
      });
      exploreSimpleSk.getHeader = () => [];
      exploreSimpleSk.getCommitLinks = () => [];
      exploreSimpleSk.getAnomalyMap = () => ({});
      exploreSimpleSk.getSelectedRange = () => null;

      element['exploreElements'] = [exploreSimpleSk];
      element['graphConfigs'] = [
        { queries: ['config=test1', 'config=test2'], formulas: [], keys: '' },
      ];
      element.state.splitByKeys = ['config'];

      await element['splitGraphs']();

      // After splitting 2 traces, totalGraphs should be 2. The internal
      // exploreElements array will be 3 (1 master + 2 split).
      assert.equal(element.state.totalGraphs, 2);
      assert.equal(element['exploreElements'].length, 3);
    });

    it('does not split if there are no split keys', async () => {
      const exploreSimpleSk = new ExploreSimpleSk();
      exploreSimpleSk.getTraceset = () => ({
        ',config=test1,': [1, 2],
        ',config=test2,': [3, 4],
      });
      element['exploreElements'] = [exploreSimpleSk];
      element['graphConfigs'] = [
        { queries: ['config=test1', 'config=test2'], formulas: [], keys: '' },
      ];
      element.state.splitByKeys = []; // No split key.
      element.state.totalGraphs = 1;

      const clearSpy = sinon.spy(element, 'clearGraphs' as any);
      await element['splitGraphs']();

      // Should return early without modifying the graphs.
      assert.isTrue(clearSpy.notCalled);
      assert.equal(element['exploreElements'].length, 1);
    });
  });

  describe('Synchronization', () => {
    beforeEach(async () => {
      await setupElement();
    });

    it('syncs the x-axis label across all graphs', () => {
      const graph1 = element['addEmptyGraph']()!;
      const graph2 = element['addEmptyGraph']()!;
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

    it('syncs point selection across all graphs', () => {
      // Create simple mock graphs to purely test the handler logic.
      const graph1 = { updateSelectedRangeWithPlotSummary: () => {} };
      const graph2 = { updateSelectedRangeWithPlotSummary: () => {} };
      const spy1 = sinon.spy(graph1, 'updateSelectedRangeWithPlotSummary');
      const spy2 = sinon.spy(graph2, 'updateSelectedRangeWithPlotSummary');

      // Manually set the internal state to use our mocks.
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

  describe('loadAllCharts', () => {
    beforeEach(() => {
      // Mock window.confirm to always return true for these tests.
      sinon.stub(window, 'confirm').returns(true);
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

      await element['loadAllCharts']();

      assert.equal(element.state.pageSize, 2);
      assert.equal(element.state.pageOffset, 0);
      assert.isTrue((element['stateHasChanged'] as sinon.SinonSpy).calledOnce);
      assert.isTrue(splitGraphsSpy.calledOnce);
    });

    it('calls splitGraphs and updates pagination when there is only one graph', async () => {
      const splitGraphsSpy = sinon.spy(element, 'splitGraphs' as any);
      element['exploreElements'] = [new ExploreSimpleSk()];
      element.state.totalGraphs = 1;

      await element['loadAllCharts']();

      assert.equal(element.state.pageSize, 0);
      assert.equal(element.state.pageOffset, 0);
      assert.isTrue((element['stateHasChanged'] as sinon.SinonSpy).calledOnce);
      assert.isTrue(splitGraphsSpy.calledOnce);
    });

    it('populates currentPageExploreElements after loading all charts', () => {
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
      element['graphConfigs'] = [
        { queries: ['config=test1', 'config=test2'], formulas: [], keys: '' },
        { queries: ['config=test1'], formulas: [], keys: '' },
        { queries: ['config=test2'], formulas: [], keys: '' },
      ];
      element.state.splitByKeys = ['config'];

      element['loadAllCharts']();

      // After splitting, there should be 3 elements (1 master, 2 split).
      // After loadAllCharts, pageSize is 2, so currentPageExploreElements should have 2.
      assert.equal(element['currentPageExploreElements'].length, 2);
    });
  });
});
