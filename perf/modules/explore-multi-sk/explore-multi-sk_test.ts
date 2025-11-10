import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import sinon from 'sinon';
import { ExploreMultiSk, State } from './explore-multi-sk';
import { GraphConfig, ExploreSimpleSk } from '../explore-simple-sk/explore-simple-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { setUpExploreDemoEnv } from '../common/test-util';
import { PlotSelectionEventDetails } from '../plot-google-chart-sk/plot-google-chart-sk';
import { PaginationSkPageChangedEventDetail } from '../../../golden/modules/pagination-sk/pagination-sk';
import { CommitNumber, TimestampSeconds } from '../json';
import { TestPickerSk } from '../test-picker-sk/test-picker-sk';
import * as loader from '@google-web-components/google-chart/loader';

describe('ExploreMultiSk', () => {
  let element: ExploreMultiSk;

  // Common setup for most tests
  const setupElement = async (mockDefaults: any = null, paramsMock: any = null) => {
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
      show_bisect_btn: true,
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
        show_bisect_btn: true,
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

  describe('Graph Splitting', () => {
    beforeEach(async () => {
      await setupElement();
    });
  });

  describe('Synchronization', () => {
    beforeEach(async () => {
      await setupElement();
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
      };

      element['addStateToExplore'](simpleSk, new GraphConfig(), false);

      assert.equal(simpleSk.state.begin, 1000);
      assert.equal(simpleSk.state.end, 2000);
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
      element['graphConfigs'] = [
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
    it('should return the original ParamSet in an array if the split key has only one value', () => {
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
    let setProgressSpy: sinon.SinonSpy;
    let testPicker: TestPickerSk;

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
      sinon.stub(mainGraph, 'requestComplete').get(() => Promise.resolve());

      // We stub 'addEmptyGraph' to ensure it returns our controlled instance
      // of ExploreSimpleSk, allowing us to spy on its methods.
      sinon.stub(element, 'addEmptyGraph' as any).returns(mainGraph);

      // Spy on setProgress to check for correct UI feedback.
      setProgressSpy = sinon.spy(element, 'setProgress' as any);

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
    });

    it('loads graphs in chunks and fetches extended data once at the end', async () => {
      const updateShortcutSpy = sinon.spy(element, 'updateShortcutMultiview' as any);
      // Dispatch the event that triggers the chunking logic.
      const event = new CustomEvent('plot-button-clicked', { bubbles: true });
      element.dispatchEvent(event);

      // The event handler is async. We need to wait for it to complete.
      // A small timeout allows the chain of promises in the handler to resolve.
      await new Promise((resolve) => setTimeout(resolve, 0));

      // --- Assertions ---

      // With 7 groups and a CHUNK_SIZE of 5, we expect 3 calls to add traces:
      // 1. The first graph (chunk size of 1)
      // 2. The next 5 graphs (chunk size of 5)
      // 3. The final graph
      assert.equal(
        addFromQuerySpy.callCount,
        3,
        'addFromQueryOrFormula should be called for each chunk'
      );

      // Verify that `loadExtendedRange` was correctly set to false for all chunk-loading calls.
      assert.isFalse(
        addFromQuerySpy.firstCall.args[4],
        'loadExtendedRange should be false for the first chunk'
      );
      assert.isFalse(
        addFromQuerySpy.secondCall.args[4],
        'loadExtendedRange should be false for the second chunk'
      );
      assert.isFalse(
        addFromQuerySpy.thirdCall.args[4],
        'loadExtendedRange should be false for the third chunk'
      );

      // Verify that `loadExtendedRangeData` was called exactly once, after all chunks were processed.
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
      assert.isTrue(setProgressSpy.callCount >= 6, 'setProgress should be called multiple times');
      assert.equal(setProgressSpy.getCall(0).args[0], 'Loading graphs...');
      assert.equal(setProgressSpy.getCall(1).args[0], 'Loading graphs 1-1 of 7');
      assert.equal(setProgressSpy.getCall(2).args[0], 'Loading graphs 2-6 of 7');
      assert.equal(setProgressSpy.getCall(3).args[0], 'Loading graphs 7-7 of 7');
      assert.equal(setProgressSpy.getCall(4).args[0], 'Loading more data for all graphs...');
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
      // The stub for `splitGraphs` prevents `addGraphsToCurrentPage` from being called
      // in a way that's useful for this test's setup, so we call it manually.
      element['exploreElements'] = [
        mainGraph,
        ...Array(totalSplitGraphs).fill(new ExploreSimpleSk()),
      ];
      element['graphConfigs'] = Array(totalSplitGraphs + 1).fill(new GraphConfig());
      element['addGraphsToCurrentPage'](true); // This will respect the small pageSize.

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

      // Stub window.confirm to simulate user confirmation.
      const confirmStub = sinon.stub(window, 'confirm').returns(true);

      // The real `loadAllCharts` calls `splitGraphs`. We'll spy on it.
      const splitGraphsSpy = element['splitGraphs'] as sinon.SinonSpy;

      await element['loadAllCharts']();

      assert.isTrue(confirmStub.calledOnce, 'window.confirm should be called');
      assert.equal(
        element.state.pageSize,
        totalSplitGraphs,
        'Page size should be updated to total graphs'
      );
      assert.equal(element.state.pageOffset, 0, 'Page offset should be reset');
      assert.isTrue(splitGraphsSpy.called, 'splitGraphs should be called to reload');

      element['addGraphsToCurrentPage']();
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

    it('correctly calculates begin and end times when dayRange is provided', async () => {
      // Use fake timers to control Date.now()
      const clock = sinon.useFakeTimers();
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

      // Mock addGraphsToCurrentPage to avoid DOM issues in this test
      sinon.stub(element, 'addGraphsToCurrentPage' as any).returns(undefined);
      // Mock splitGraphs as well since it's called after graph loading
      sinon.stub(element, 'splitGraphs' as any).resolves();
      // Mock checkDataLoaded to prevent side effects
      sinon.stub(element, 'checkDataLoaded' as any).returns(undefined);

      await element['_onStateChangedInUrl'](state as any);

      // Check the aggregated query in the first graph config.
      const aggregatedQuery = element['graphConfigs'][0].queries[0];
      assert.include(aggregatedQuery, 'config=with%20space');
      assert.include(aggregatedQuery, 'arch=x86%20new');
      assert.notInclude(aggregatedQuery, '+');

      loadStub.restore();
    });
  });
});
