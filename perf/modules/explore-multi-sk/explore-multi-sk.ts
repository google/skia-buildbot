/**
 * @module module/explore-multi-sk
 * @description <h2><code>explore-multi-sk</code></h2>
 *
 * Page of Perf for exploring data in multiple graphs.
 *
 * User is able to add multiple ExploreSimpleSk instances. The state reflector will
 * only keep track of those properties necessary to add traces to each graph. All
 * other settings, such as the point selected or the beginning and ending range,
 * will be the same for all graphs. For example, passing ?dots=true as a URI
 * parameter will enable dots for all graphs to be loaded. This is to prevent the
 * URL from becoming too long and keeping the logic simple.
 *
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import {
  DEFAULT_RANGE_S,
  ExploreSimpleSk,
  State as ExploreState,
  GraphConfig,
  LabelMode,
  updateShortcut,
} from '../explore-simple-sk/explore-simple-sk';
import { PlotSelectionEventDetails } from '../plot-google-chart-sk/plot-google-chart-sk';
import { load } from '@google-web-components/google-chart/loader';
import { TestPickerSk } from '../test-picker-sk/test-picker-sk';

import { addParamsToParamSet, fromKey, queryFromKey } from '../paramtools';
import { ParamSet as QueryParamSet, fromParamSet } from '../../../infra-sk/modules/query';
import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { HintableObject } from '../../../infra-sk/modules/hintable';
import { errorMessage } from '../errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
  AnomalyMap,
  ColumnHeader,
  FrameRequest,
  FrameResponse,
  ParamSet,
  QueryConfig,
  ReadOnlyParamSet,
  RequestType,
  TraceSet,
  Trace,
} from '../json';

import '../../../elements-sk/modules/spinner-sk';
import '../explore-simple-sk';
import '../favorites-dialog-sk';
import '../test-picker-sk';
import '../../../golden/modules/pagination-sk/pagination-sk';
import '../window/window';

import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { LoggedIn } from '../../../infra-sk/modules/alogin-sk/alogin-sk';
import { Status as LoginStatus } from '../../../infra-sk/modules/json';
import { PaginationSkPageChangedEventDetail } from '../../../golden/modules/pagination-sk/pagination-sk';
import { CommitLinks } from '../point-links-sk/point-links-sk';

export class State {
  begin: number = -1;

  end: number = -1;

  shortcut: string = '';

  showZero: boolean = false;

  dots: boolean = true;

  numCommits: number = 250;

  request_type: RequestType = 0;

  domain: 'commit' | 'date' = 'commit'; // The domain of the x-axis, either commit or date.

  summary: boolean = false;

  pageSize: number = 30;

  pageOffset: number = 0;

  totalGraphs: number = 0;

  plotSummary: boolean = false;

  useTestPicker: boolean = false;

  highlight_anomalies: string[] = [];

  enable_chart_tooltip: boolean = false;

  show_remove_all: boolean = true;

  use_titles: boolean = false;

  show_google_plot = false;

  xbaroffset: number = -1;

  splitByKeys: string[] = [];

  dayRange: number = -1;

  dateAxis: boolean = false;
}

export class ExploreMultiSk extends ElementSk {
  private graphConfigs: GraphConfig[] = [];

  private exploreElements: ExploreSimpleSk[] = [];

  private currentPageExploreElements: ExploreSimpleSk[] = [];

  private currentPageGraphConfigs: GraphConfig[] = [];

  private stateHasChanged: (() => void) | null = null;

  private _state: State = new State();

  private graphDiv: Element | null = null;

  private useTestPicker: boolean = false;

  private testPicker: TestPickerSk | null = null;

  private defaults: QueryConfig | null = null;

  private userEmail: string = '';

  private _dataLoading: boolean = false;

  private progress: string = '';

  private setProgress(value: string) {
    this.progress = value;
    this._render();
  }

  private _onSplitByChanged = async (e: Event) => {
    this._dataLoading = true;
    this.testPicker?.setReadOnly(true);
    const splitByParamKey: string = (e as CustomEvent).detail.param;
    const split = (e as CustomEvent).detail.split;
    if (!split) {
      // No longer split so remove selected param from keys.
      this.state.splitByKeys = this.state.splitByKeys.filter((key) => key !== splitByParamKey);
    } else {
      // Split by only a single key
      // TODO(seawardt): Enable multiple splits
      this.state.splitByKeys = [splitByParamKey];
    }
    this.splitGraphs();
  };

  constructor() {
    super(ExploreMultiSk.template);
  }

  async connectedCallback() {
    super.connectedCallback();

    this._render();

    this.graphDiv = this.querySelector('#graphContainer');
    this.testPicker = this.querySelector('#test-picker');

    await this.initializeDefaults();
    this.stateHasChanged = stateReflector(
      () => this.state as unknown as HintableObject,
      async (hintableState) => {
        const state = hintableState as unknown as State;

        // -- Domain Logic --
        const useDateAxis = state.dateAxis
          ? state.dateAxis
          : this.defaults?.default_xaxis_domain === 'date';
        state.domain = useDateAxis ? 'date' : 'commit';

        // -- Time Range Logic --
        // Precedence: explicit begin/end > dayRange > component defaults.
        const beginProvided = state.begin !== -1;
        const endProvided = state.end !== -1;
        const dayRangeProvided = state.dayRange !== -1;

        const now = Math.floor(Date.now() / 1000);
        const defaultRangeS = this.defaults?.default_range || DEFAULT_RANGE_S;

        if (beginProvided || endProvided) {
          // Scenario 1: begin and/or end are provided in the URL.
          if (!beginProvided) {
            state.begin = state.end - defaultRangeS;
          } else if (!endProvided) {
            state.end = state.begin + defaultRangeS;
            if (state.end > now) state.end = now;
          }
        } else if (dayRangeProvided) {
          // Scenario 2: dayRange is provided, begin/end are NOT.
          state.end = now;
          state.begin = now - state.dayRange * 24 * 60 * 60;
          this.state = state;
        } else {
          // Scenario 3: No time parameters in URL, use component defaults.
          state.begin = now - defaultRangeS;
          state.end = now;
        }
        const numElements = this.exploreElements.length;
        let graphConfigs: GraphConfig[] = [];
        if (state.shortcut !== '') {
          const shortcutConfigs = (await this.getConfigsFromShortcut(state.shortcut)) ?? [];
          graphConfigs = shortcutConfigs.map((c) => Object.assign(new GraphConfig(), c));
        }

        // This loop removes graphs that are not in the current config.
        // This can happen if you add a graph and then use the browser's back button.
        while (this.exploreElements.length > graphConfigs.length) {
          this.exploreElements.pop();
          this.graphConfigs.pop();
          // Ensure graphDiv exists and has children before removing.
          if (this.graphDiv && this.graphDiv.lastChild) {
            this.graphDiv.removeChild(this.graphDiv.lastChild);
          }
        }

        const validGraphs: GraphConfig[] = [];
        for (let i = 0; i < graphConfigs.length; i++) {
          if (
            graphConfigs[i].formulas.length > 0 ||
            graphConfigs[i].queries.length > 0 ||
            graphConfigs[i].keys !== ''
          ) {
            if (i >= numElements) {
              this.addEmptyGraph();
            }
            validGraphs.push(graphConfigs[i]);
          }
        }
        this.graphConfigs = validGraphs;

        this.state = state;
        if (state.useTestPicker) {
          this.initializeTestPicker();
        }
        await load();

        google.charts.setOnLoadCallback(() => {
          this.addGraphsToCurrentPage();
        });

        document.addEventListener('keydown', (e) => {
          this.exploreElements.forEach((exp) => {
            exp.keyDown(e);
          });
        });

        // If a key is specified (eg: directly via url), perform the split
        if (this.state.splitByKeys.length > 0) {
          this._dataLoading = true;
          this.testPicker?.setReadOnly(true);
          this.splitGraphs();
        }
      }
    );

    LoggedIn()
      .then((status: LoginStatus) => {
        this.userEmail = status.email;
        this._render();
      })
      .catch(errorMessage);
  }

  private canAddFav(): boolean {
    return this.userEmail !== null && this.userEmail !== '';
  }

  private static template = (ele: ExploreMultiSk) => html`
    <div id="menu">
      <h1>MultiGraph Menu</h1>
      <spinner-sk id="spinner"></spinner-sk>
      <test-picker-sk id="test-picker" class="hidden"></test-picker-sk>
      ${ele.progress
        ? html`
            <div class="progress-container">
              <spinner-sk id="spinner" active></spinner-sk>
              <span class="progress">${ele.progress}</span>
            </div>
          `
        : ''}
    </div>
    <hr />

    <div id="pagination">
      <pagination-sk
        offset=${ele.state.pageOffset}
        page_size=${ele.state.pageSize}
        total=${ele.state.totalGraphs}
        @page-changed=${ele.pageChanged}>
      </pagination-sk>

      ${ele.state.totalGraphs < 10
        ? ''
        : html`
            <label>
              <span class="prefix">Charts per page</span>
              <input
                @change=${ele.pageSizeChanged}
                type="number"
                .value="${ele.state.pageSize.toString()}"
                min="1"
                max="50"
                title="The number of charts per page." />
            </label>
            <button @click=${ele.loadAllCharts}>Load All Charts</button>
          `}

      <div
        id="graphContainer"
        @x-axis-toggled=${ele.syncXAxisLabel}
        @range-changing-in-multi=${ele.syncRange}
        @selection-changing-in-multi=${ele.syncChartSelection}></div>
      <pagination-sk
        offset=${ele.state.pageOffset}
        page_size=${ele.state.pageSize}
        total=${ele.state.totalGraphs}
        @page-changed=${ele.pageChanged}>
      </pagination-sk>
    </div>
  `;

  /**
   * Fetch defaults from backend.
   *
   * Defaults are used in multiple ways by downstream elements:
   * - TestPickerSk uses include_params to initialize only the fields
   *   specified and in the given order.
   * - ExploreSimpleSk and TestPickerSk use default_param_selections to
   *   apply default param values to queries before making backend
   *   requests.
   */
  private async initializeDefaults() {
    try {
      const response = await fetch(`/_/defaults/`, {
        method: 'GET',
      });
      const json = await jsonOrThrow(response);
      this.defaults = json;
    } catch (error: any) {
      console.error('Error fetching defaults:', error);
      errorMessage(`Failed to load default configuration: ${error.message || error}`);
      this.defaults = null;
    }

    if (this.defaults !== null) {
      if (
        this.defaults.default_url_values !== undefined &&
        this.defaults.default_url_values !== null
      ) {
        const defaultKeys = Object.keys(this.defaults.default_url_values);
        if (
          defaultKeys !== null &&
          defaultKeys !== undefined &&
          defaultKeys.indexOf('useTestPicker') > -1
        ) {
          const stringToBool = function (str: string): boolean {
            return str.toLowerCase() === 'true';
          };
          this.state.useTestPicker = stringToBool(this.defaults!.default_url_values.useTestPicker);
        }
      }
    }
  }

  /**
   * Splits a ParamSet into multiple ParamSets based on the values of a given key.
   * This is analogous to groupTracesByParamKey, but operates on a ParamSet
   * instead of a list of trace IDs.
   *
   * For example, given a ParamSet:
   * {
   * "a": ["x", "y"],
   * "b": ["z"],
   * }
   * and a split key of "a", the function will return an array of two ParamSets:
   * [
   * { "a": ["x"], "b": ["z"] },
   * { "a": ["y"], "b": ["z"] }
   * ]
   *
   * @param paramSet The QueryParamSet to split.
   * @param splitByKeys The key(s) to split by. Currently, only the first key is used.
   * @returns An array of ParamSets, each representing a group.
   */
  private groupParamSetBySplitKey(paramSet: QueryParamSet, splitByKeys: string[]): QueryParamSet[] {
    if (splitByKeys.length === 0 || Object.keys(paramSet).length === 0) {
      return [paramSet];
    }

    // Only handle the first split key for now.
    const splitKey = splitByKeys[0];
    const splitValues = paramSet[splitKey];
    if (!splitValues || splitValues.length <= 1) {
      return [paramSet];
    }

    const groups: ParamSet[] = [];
    splitValues.forEach((value) => {
      const newGroup: ParamSet = ParamSet({ ...paramSet });
      // Override the split key to have only the current single value.
      newGroup[splitKey] = [value];
      groups.push(newGroup);
    });
    return groups;
  }

  private mergeParamSets(paramSets: QueryParamSet[]): QueryParamSet {
    const merged: QueryParamSet = {};
    for (const currentObject of paramSets) {
      for (const key in currentObject) {
        if (merged[key]) {
          merged[key] = [...new Set([...merged[key], ...currentObject[key]])];
        } else {
          merged[key] = currentObject[key];
        }
      }
    }
    return merged;
  }

  /**
   * Groups all the traces on the current multi graph view based on the
   * key selected in the SplitBy dropdown.
   * @returns A map where the key is the value of the split by param and
   * the value is a list of traceIds grouped by that value.
   */
  private groupTracesBySplitKey(): Map<string, string[]> {
    const splitKeys = this.state.splitByKeys;
    const traceset: string[] = [];
    this.getTracesets().forEach((ts) => {
      traceset.push(...ts);
    });

    return this.groupTracesByParamKey(traceset, splitKeys);
  }

  /**
   * groupTracesByParamKey returns a map where the key is the paramValue (for the given param key)
   * and the value is the group of traces matching that param value.
   * @param traceset Set of traces to split.
   * @param key Param key to base the split on.
   */
  private groupTracesByParamKey(traceset: string[], keys: string[]): Map<string, string[]> {
    const groupedTraces = new Map<string, string[]>();
    if (traceset.length > 0) {
      // If there are no keys, then pass in empty string to group everything.
      const keysToUse = keys.length === 0 ? [''] : keys;
      traceset.forEach((traceId) => {
        const traceParams = new URLSearchParams(fromKey(traceId));
        keysToUse.forEach((key) => {
          const splitValue = traceParams.get(key);
          const existingGroup = groupedTraces.get(splitValue!) ?? [];
          if (!existingGroup.includes(traceId)) {
            existingGroup.push(traceId);
          }
          groupedTraces.set(splitValue!, existingGroup);
        });
      });
    }
    return groupedTraces;
  }

  /**
   * Creates a FrameRequest object.
   *
   * @param traces - An optional array of trace IDs. If provided,
   *                 queries will be generated from these traces.
   *                 Otherwise, the queries from the first graph configuration will be used.
   * @returns A FrameRequest object.
   */
  private createFrameRequest(traces?: string[]): FrameRequest {
    const queries: string[] = [];
    if (traces) {
      traces.forEach((trace) => {
        queries.push(queryFromKey(trace));
      });
    } else {
      queries.push(...this.graphConfigs[0].queries);
    }
    const request: FrameRequest = {
      queries: queries,
      request_type: this.state.request_type,
      begin: this.state.begin,
      end: this.state.end,
      tz: '',
    };
    return request;
  }

  /**
   * Creates a FrameResponse object.
   *
   * @param traces - An optional array of trace IDs to filter the response.
   *                 If not provided, the response
   *                 will include all traces from all graphs.
   * @returns A FrameResponse object.
   */
  private createFrameResponse(traces?: string[]): FrameResponse {
    const fullTraceSet = this.getCompleteTraceset();
    const header = this.getHeader();
    const mainParams: ParamSet = ParamSet({});
    const fullAnomalyMap = this.getFullAnomalyMap();

    Object.keys(fullTraceSet).forEach((trace) => {
      addParamsToParamSet(mainParams, fromKey(trace));
    });

    // Use primary explore element for main chart.
    let traceset = fullTraceSet as TraceSet;
    let paramset = mainParams;
    const commitLinks = this.exploreElements[0].getCommitLinks();
    let traceMetadata = ExploreSimpleSk.getTraceMetadataFromCommitLinks(
      Object.keys(fullTraceSet),
      commitLinks
    );
    let anomalyMap = this.getAnomalyMapForTraces(fullAnomalyMap, Object.keys(fullTraceSet));

    // If passing in traces, then create child specific requests per trace.
    if (traces) {
      const traceSet: TraceSet = TraceSet({});
      const paramSet: ParamSet = ParamSet({});
      traces.forEach((trace) => {
        traceSet[trace] = Trace(fullTraceSet[trace]);
        addParamsToParamSet(paramSet, fromKey(trace));
      });
      traceset = traceSet;
      paramset = paramSet;
      traceMetadata = ExploreSimpleSk.getTraceMetadataFromCommitLinks(traces, commitLinks);
      anomalyMap = this.getAnomalyMapForTraces(fullAnomalyMap, traces);
    }

    const response: FrameResponse = {
      dataframe: {
        traceset: traceset,
        header: header,
        paramset: ReadOnlyParamSet(paramset),
        skip: 0,
        traceMetadata: traceMetadata,
      },
      anomalymap: anomalyMap,
      display_mode: 'display_plot',
      msg: '',
      skps: [],
    };
    return response;
  }

  /**
   * Splits the graphs based on the split by dropdown selection.
   */
  private async splitGraphs(): Promise<void> {
    const groupedTraces = this.groupTracesBySplitKey();
    if (groupedTraces.size === 0) {
      this.checkDataLoaded();
      return;
    }

    const groupedLength = Array.from(groupedTraces.values()).reduce((sum, v) => sum + v.length, 0);
    // It's the same graph, so let's simply return early.
    if (groupedLength === 1 && this.state.totalGraphs === 1) {
      this.checkDataLoaded();
      return;
    }

    if (this.exploreElements.length > 0 && this._dataLoading === true) {
      errorMessage('Data is still loading, please wait...', 3000);
      await this.exploreElements[0].requestComplete;
    }

    const selectedRange = this.exploreElements[0].getSelectedRange();

    // Create the main graph config containing all trace data.
    const mainRequest: FrameRequest = this.createFrameRequest();
    const mainResponse: FrameResponse = this.createFrameResponse();

    const frameRequests: FrameRequest[] = [mainRequest];
    const frameResponses: FrameResponse[] = [mainResponse];

    this.clearGraphs();
    // Create the graph configs for each group.
    Array.from(groupedTraces.values()).forEach((traces, i) => {
      this.addEmptyGraph();
      const exploreRequest = this.createFrameRequest(traces);
      const exploreResponse = this.createFrameResponse(traces);

      const graphConfig = new GraphConfig();
      graphConfig.queries = exploreRequest.queries ?? [];
      // Main graph config is always at index 0.
      this.graphConfigs[i + 1] = graphConfig;

      frameRequests.push(exploreRequest);
      frameResponses.push(exploreResponse);
    });

    // Now add the graphs that have been configured to the page.
    this.addGraphsToCurrentPage(true);

    const isSplitChart: boolean = this.exploreElements.length > 1;
    // Limit page size to the number of graphs available.
    const limit = Math.min(
      this.state.pageSize + this.state.pageOffset + 1,
      this.exploreElements.length
    );

    // If graph is being split, skip the primary graph (index 0), as it contains all the data.
    // This is to avoid displaying the primary graph in the pagination view.
    const offset = isSplitChart ? this.state.pageOffset + 1 : 0;
    for (let i = offset; i < limit; i++) {
      this.exploreElements[i].UpdateWithFrameResponse(
        frameResponses[i],
        frameRequests[i],
        false,
        selectedRange
      );
    }
    if (this.stateHasChanged) {
      this.stateHasChanged();
    }
    this.checkDataLoaded();
  }

  /**
   * Initialize TestPickerSk only if include_params has been specified.
   *
   * If so, hide the default "Add Graph" button and display the Test Picker.
   */
  private async initializeTestPicker() {
    const testPickerParams = this.defaults?.include_params ?? null;
    if (testPickerParams !== null) {
      this.useTestPicker = true;
      this.testPicker!.classList.remove('hidden');
      let defaultParams = this.defaults?.default_param_selections ?? {};
      if (window.perf.remove_default_stat_value) {
        defaultParams = {};
      }

      const readOnly = this.exploreElements.length > 0;
      this.testPicker!.initializeTestPicker(testPickerParams!, defaultParams, readOnly);
      this._render();
    }
    // Event listener to remove the explore object from the list if the user
    // close it in a Multiview window.
    this.addEventListener('remove-explore', (e) => {
      const exploreElemToRemove = (e as CustomEvent).detail.elem as ExploreSimpleSk;
      if (this.exploreElements.length === 1) {
        this.removeExplore(exploreElemToRemove);
        e.stopPropagation();
      } else {
        const param = this.state.splitByKeys[0];
        if (exploreElemToRemove.state.queries.length > 0) {
          const query = exploreElemToRemove.state.queries[0];
          const valueToRemove = new URLSearchParams(query).get(param);
          if (valueToRemove) {
            this.testPicker?.removeItemFromChart(param, [valueToRemove]);
          }
        }
      }
    });

    // Event listener for when the Test Picker plot button is clicked.
    // This will create a new empty Graph at the top and plot it with the
    // selected test values.
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    this.addEventListener('plot-button-clicked', async (e) => {
      this._dataLoading = true;
      this.testPicker?.setReadOnly(true);
      this.setProgress('Loading graphs...');
      try {
        if (this.state.splitByKeys.length === 0) {
          // Just load single graph.
          const newExplore = this.addEmptyGraph(true);
          if (!newExplore) {
            return;
          }
          if (this.exploreElements.length > 0 && this._dataLoading) {
            await this.exploreElements[0].requestComplete;
          }
          this.addGraphsToCurrentPage(true);
          const query = this.testPicker!.createQueryFromFieldData();
          await newExplore.addFromQueryOrFormula(true, 'query', query, '');
        } else {
          // Load multiple graphs, split by the selected split key.
          // To improve UX, allow some interactivity before all the data is loaded.
          // To achieve this, load everything in 2 steps:
          // 1. Load all graphs, but only the selected range. This stage can be chunked,
          //    updates are incremental.
          // 2. Load extended range data for all graphs. This is one huge request, but it
          //    allows to avoid any troubles with concurrency and merging.

          // Split the graphs before loading, so we can load each group separately.
          const paramSet = this.testPicker!.createParamSetFromFieldData();
          const groups = this.groupParamSetBySplitKey(paramSet, this.state.splitByKeys);

          if (groups.length === 0) {
            return;
          }

          // The mainGraph (exploreElements[0]) will act as an accumulator for all queries.
          // splitGraphs will then use its accumulated traceset to create the individual
          // split graphs.
          const mainGraph = this.addEmptyGraph(true);
          if (!mainGraph) {
            return;
          }
          await mainGraph.requestComplete;
          this.addGraphsToCurrentPage(false);

          const CHUNK_SIZE = 5;
          const totalGroupsToLoad = Math.min(this.state.pageSize, groups.length);

          for (let i = 0; i < totalGroupsToLoad; ) {
            // The first chunk is always of size 1 - this is to avoid showing the primary
            // graph / "unsplit" mode.
            const chunkSize = i === 0 ? 1 : CHUNK_SIZE;
            const endGroupIndex = Math.min(i + chunkSize, totalGroupsToLoad);
            const chunk = groups.slice(i, endGroupIndex);
            if (chunk.length === 0) {
              break; // No more groups to process.
            }

            this.setProgress(`Loading graphs ${i + 1}-${endGroupIndex} of ${totalGroupsToLoad}`);
            await mainGraph.addFromQueryOrFormula(
              /*replace=*/ false,
              'query',
              fromParamSet(this.mergeParamSets(chunk)),
              '',
              // Important! Do not load extended range data. Otherwise it produces a lot of
              // queries fetching the same data + creates concurrency issues.
              /*loadExtendedRange=*/ false
            );
            await mainGraph.requestComplete;
            await this.splitGraphs();

            i = endGroupIndex;
          }

          // We were postponing loading more data until all the graphs are ready. Now it's time.
          this.setProgress(`Loading more data for all graphs...`);
          await mainGraph.loadExtendedRangeData(mainGraph.getSelectedRange()!);
          await mainGraph.requestComplete;
          await this.splitGraphs();
        }
      } finally {
        this.setProgress('');
        this.checkDataLoaded();
      }
      if (this.testPicker) {
        this.testPicker.autoAddTrace = true;
      }
    });

    this.removeEventListener('split-by-changed', this._onSplitByChanged);
    this.addEventListener('split-by-changed', this._onSplitByChanged);
    // Event listener for when the Test Picker plot button is clicked.
    // This will create a new empty Graph at the top and plot it with the
    // selected test values.
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    this.addEventListener('add-to-graph', async (e) => {
      const query = (e as CustomEvent).detail.query;
      // Query is the same as the first graph, so do nothing.
      if (query === this.graphConfigs[0].queries[0]) {
        return;
      }
      let explore: ExploreSimpleSk;
      this._dataLoading = true;
      this.testPicker?.setReadOnly(true);
      if (this.currentPageExploreElements.length === 0) {
        const newExplore = this.addEmptyGraph(true);
        if (newExplore) {
          if (this.exploreElements.length > 0 && this._dataLoading) {
            await this.exploreElements[0].requestComplete;
          }
          this.addGraphsToCurrentPage(true);
          explore = newExplore;
        } else {
          return;
        }
      } else {
        explore = this.exploreElements[0];
        this.currentPageExploreElements.splice(1);
        this.currentPageGraphConfigs.splice(1);
        this.exploreElements.splice(1);
        this.graphConfigs.splice(1);
        this.state.totalGraphs = this.exploreElements.length;
        explore.state.doNotQueryData = false;
      }
      await explore.addFromQueryOrFormula(true, 'query', query, '');
      await this.splitGraphs();
    });

    this.addEventListener('remove-trace', async (e) => {
      const param = (e as CustomEvent).detail.param as string;
      const values = (e as CustomEvent).detail.value as string[];
      const query = (e as CustomEvent).detail.query as string[];

      if (values.length === 0) {
        this.resetGraphs();
        return;
      }
      this._dataLoading = true;
      this.testPicker?.setReadOnly(true);
      const traceSet = this.getCompleteTraceset();
      const tracesToRemove: string[] = [];
      const queriesToRemove: string[] = [];

      // Check through all existing TraceSets and find matches.
      Object.keys(traceSet).forEach((trace) => {
        const traceParams = fromKey(trace);
        if (traceParams[param] && values.includes(traceParams[param])) {
          // Load remove array and delete from existing traceSet.
          tracesToRemove.push(trace);
          queriesToRemove.push(queryFromKey(trace));
          delete traceSet[trace];
        }
      });

      if (Object.keys(traceSet).length === 0) {
        this.emptyCurrentPage();
        return;
      }

      // Remove the traces from the current page explore elements.
      const elemsToRemove: ExploreSimpleSk[] = [];
      const updatePromises = this.exploreElements.map((elem) => {
        if (!elem.state.queries?.length) {
          return Promise.resolve();
        }

        elem.state.doNotQueryData = true;
        const traceset = elem.getTraceset() as TraceSet;
        elem.state.queries = elem.state.queries.filter(
          (q) =>
            !q.includes(`${(e as CustomEvent).detail.param}=${(e as CustomEvent).detail.value}`)
        );
        elem.removeKeys(tracesToRemove, true);
        if (elem.state.queries.length === 1) {
          // Only one query, so update it with the new query based on params.
          elem.state.queries = query;
        } else {
          // Multiple queries, so remove the ones that match the deleted traces.
          elem.state.queries = elem.state.queries.filter((q) => !queriesToRemove.includes(q));
        }
        if (elem.state.queries.length === 0) {
          elemsToRemove.push(elem);
          return Promise.resolve();
        }
        const params: ParamSet = ParamSet({});
        Object.keys(traceset).forEach((trace) => {
          addParamsToParamSet(params, fromKey(trace));
        });

        // Update the graph with the new traceSet and params.
        const updatedRequest: FrameRequest = {
          queries: elem.state.queries,
          request_type: this.state.request_type,
          begin: this.state.begin,
          end: this.state.end,
          tz: '',
        };
        const updatedResponse: FrameResponse = {
          dataframe: {
            traceset: traceset,
            header: this.getHeader(),
            paramset: ReadOnlyParamSet(params),
            skip: 0,
            traceMetadata: ExploreSimpleSk.getTraceMetadataFromCommitLinks(
              Object.keys(traceset),
              elem.getCommitLinks()
            ),
          },
          anomalymap: this.getAnomalyMapForTraces(this.getFullAnomalyMap(), Object.keys(traceset)),
          display_mode: 'display_plot',
          msg: '',
          skps: [],
        };
        return elem.UpdateWithFrameResponse(
          updatedResponse,
          updatedRequest,
          true,
          this.exploreElements[0].getSelectedRange()
        );
      });

      await Promise.all(updatePromises);

      elemsToRemove.forEach((elem) => {
        this.removeExplore(elem);
      });

      if (this.stateHasChanged) {
        this.stateHasChanged();
        this.checkDataLoaded();
      }
    });
    // Event listener for when the "Query Highlighted" button is clicked.
    // It will populate the Test Picker with the keys from the highlighted
    // trace.
    this.addEventListener('populate-query', (e) => {
      this.populateTestPicker((e as CustomEvent).detail);
    });
  }

  private async populateTestPicker(paramSet: { [key: string]: string[] }) {
    const paramSets: ParamSet = ParamSet({});

    const timeoutMs = 50000; // Timeout for waiting for non-empty tracesets.
    const pollIntervalMs = 500; // Interval to re-check.

    // Create a promise that resolves when the tracesets are ready.
    // This checks if all explore elements have reported a traceset,
    // and at least one of those tracesets contains actual trace strings.
    const tracesetsReadyPromise = new Promise<string[][]>((resolve) => {
      const checkTracesets = () => {
        const currentTracesets = this.getTracesets();
        if (
          currentTracesets.length === this.exploreElements.length &&
          currentTracesets.some((ts) => ts.length > 0)
        ) {
          resolve(currentTracesets);
        } else {
          setTimeout(checkTracesets, pollIntervalMs);
        }
      };
      checkTracesets(); // Start checking immediately
    });

    // Create a timeout promise.
    const timeoutPromise = new Promise<string[][]>((_, reject) => {
      setTimeout(() => reject(new Error('Getting Tracesets timed out.')), timeoutMs);
    });

    let allTracesets: string[][];
    try {
      allTracesets = await Promise.race([tracesetsReadyPromise, timeoutPromise]);
    } catch (error: any) {
      errorMessage(error.message || 'An unknown error occurred while getting tracesets.');
      return;
    }

    allTracesets.forEach((traceset) => {
      traceset.forEach((trace) => {
        addParamsToParamSet(paramSets, fromKey(trace));
      });
    });

    this.testPicker!.populateFieldDataFromParamSet(paramSets, paramSet);
    this.testPicker!.setReadOnly(false);
    this.exploreElements[0].useBrowserURL(false);
    this.testPicker!.scrollIntoView();
  }

  private removeExplore(elem: ExploreSimpleSk | null = null): void {
    const indexToRemove = this.exploreElements.findIndex((e) => e === elem);

    if (indexToRemove > -1) {
      this.exploreElements.splice(indexToRemove, 1);
      this.graphConfigs.splice(indexToRemove, 1);
      // Re-index the remaining graphs. This ensures that the graph_index property
      // in each element's state correctly reflects its position in the exploreElements array,
      // which is important for syncing actions between graphs.
      this.exploreElements.forEach((elem, index) => {
        elem.state.graph_index = index;
      });
      const numElements = this.exploreElements.length;
      this.state.totalGraphs = numElements > 1 ? numElements - 1 : 0;

      // Adjust pagination: if there are no graphs left, reset page offset to 0.
      if (this.state.totalGraphs === 0) {
        this.state.pageOffset = 0;
        this.testPicker!.autoAddTrace = false;
        this.resetGraphs();
        this.emptyCurrentPage();
      } else if (this.state.pageSize > 0) {
        // If graphs remain and pageSize is valid, calculate the maximum valid page offset.
        // This prevents being on a page that no longer exists
        // (e.g., if the last item on the last page was removed).
        const numPages = Math.ceil(this.state.totalGraphs / this.state.pageSize);
        const maxValidPageOffset = Math.max(0, (numPages - 1) * this.state.pageSize);
        this.state.pageOffset = Math.min(this.state.pageOffset, maxValidPageOffset);
        this.addGraphsToCurrentPage(true);
      }
      this.updateShortcutMultiview();
    } else {
      const numElements = this.exploreElements.length;
      this.state.totalGraphs = numElements > 1 ? numElements - 1 : 1;
      if (this.stateHasChanged) this.stateHasChanged();
      this.addGraphsToCurrentPage(true);
    }
  }

  private resetGraphs() {
    this.emptyCurrentPage();
    this.exploreElements = [];
    this.graphConfigs = [];
  }

  private clearGraphs() {
    this.exploreElements.splice(1);
    this.graphConfigs.splice(1);
  }

  private emptyCurrentPage(): void {
    this.graphDiv!.replaceChildren();
    this.currentPageExploreElements = [];
    this.currentPageGraphConfigs = [];
  }

  private addGraphsToCurrentPage(doNotQueryData: boolean = false): void {
    this.state.totalGraphs = this.exploreElements.length > 1 ? this.exploreElements.length - 1 : 1;
    this.emptyCurrentPage();
    let startIndex = this.exploreElements.length > 1 ? this.state.pageOffset : 0;

    if (this.exploreElements.length > 1) {
      startIndex++;
    }
    let endIndex = startIndex + this.state.pageSize - 1;
    if (this.exploreElements.length <= endIndex) {
      endIndex = this.exploreElements.length - 1;
    }

    for (let i = startIndex; i <= endIndex; i++) {
      this.currentPageExploreElements.push(this.exploreElements[i]);
      this.currentPageGraphConfigs.push(this.graphConfigs[i]);
    }

    const elementsToAdd: ExploreSimpleSk[] = [];
    this.currentPageExploreElements.forEach((elem, i) => {
      elementsToAdd.push(elem);
      const graphConfig = this.currentPageGraphConfigs[i];
      this.addStateToExplore(elem, graphConfig, doNotQueryData);
    });
    this.graphDiv!.append(...elementsToAdd);
    this.updateChartHeights();
    this._render();
  }

  private updateChartHeights(): void {
    const graphs = this.graphDiv!.querySelectorAll('explore-simple-sk');
    graphs.forEach((graph) => {
      const height = graphs.length === 1 ? '500px' : '250px';
      (graph as ExploreSimpleSk).updateChartHeight(height);
    });
  }

  private async syncRange(e: CustomEvent<PlotSelectionEventDetails>): Promise<void> {
    const graphs = this.exploreElements;
    const offset = e.detail.offsetInSeconds;
    const range = e.detail.value;
    // It is possible when loading split graphs on start that the first element
    // hasnt selected a range yet.
    const selectedRange = this.exploreElements.map((e) => e.getSelectedRange()).find((r) => !!r);

    // Sets dataLoading state across all graphs since the main graph is only one doing work.
    graphs.forEach((graph, i) => {
      // Skip main graph as its loading state will be handled by extendRange.
      if (i > 0) {
        graph.dataLoading = true;
      }
    });

    // Extend range of primary graph first, so that the other graphs can use
    // the updated range when they are updated.
    await this.exploreElements[0].extendRange(range, offset);

    // Once extended, then update each split graph.
    graphs.forEach((graph, i) => {
      if (i > 0) {
        const traces = graph.getTraceset();
        const traceKeys = traces ? Object.keys(traces) : undefined;
        if (traceKeys === undefined) {
          return;
        }
        const frameRequest = this.createFrameRequest(traceKeys);
        const frameResponse = this.createFrameResponse(traceKeys);

        (graph as ExploreSimpleSk).UpdateWithFrameResponse(
          frameResponse,
          frameRequest,
          true,
          selectedRange
        );
        graph.dataLoading = false;
      }
    });
  }

  private async syncChartSelection(e: CustomEvent<PlotSelectionEventDetails>): Promise<void> {
    const graphs = this.exploreElements;
    if (!e.detail.value) {
      return;
    }

    if (graphs.length > 1 && e.detail.offsetInSeconds !== undefined) {
      await graphs[0].extendRange(e.detail.value, e.detail.offsetInSeconds);
    }
    // Default behavior for non-split views or for pan/zoom actions.
    graphs.forEach((graph, i) => {
      // only update graph that isn't selected
      if (i !== e.detail.graphNumber && e.detail.offsetInSeconds === undefined) {
        (graph as ExploreSimpleSk).updateSelectedRangeWithPlotSummary(
          e.detail.value,
          e.detail.start ?? 0,
          e.detail.end ?? 0
        );
      }
    });
    //If in multigraph view, sync the plotSummary dfRepo on other graphs.
    graphs.forEach(async (graph, i) => {
      if (i !== e.detail.graphNumber && e.detail.offsetInSeconds !== undefined)
        await graph.requestComplete; // Wait for load then update
    });

    // Ensure that the multichart state is updated when multiple charts are available.
    if (graphs.length > 1) {
      const currentUrl = new URL(window.location.href);
      const begin = currentUrl.searchParams.get('begin');
      if (begin !== null && Number(begin) !== this.state.begin) {
        this.state.begin = Number(begin);
      }
      const end = currentUrl.searchParams.get('end');
      if (end !== null && Number(end) !== this.state.end) {
        this.state.end = Number(end);
      }
      if (this.stateHasChanged) {
        this.stateHasChanged();
      }
    }
  }

  private syncXAxisLabel(e: CustomEvent): void {
    const graphs = this.graphDiv!.querySelectorAll('explore-simple-sk');
    graphs.forEach((graph, i) => {
      // Skip graph that sent the event.
      if (i !== e.detail.index) {
        (graph as ExploreSimpleSk).updateXAxis(e.detail.domain);
      }
    });
  }

  private addStateToExplore(
    explore: ExploreSimpleSk,
    graphConfig: GraphConfig,
    doNotQueryData: boolean
  ) {
    const index = this.exploreElements.indexOf(explore);
    const newState: ExploreState = {
      formulas: graphConfig.formulas || [],
      queries: graphConfig.queries || [],
      keys: graphConfig.keys || '',
      begin: this.state.begin,
      end: this.state.end,
      showZero: this.state.showZero,
      dots: this.state.dots,
      numCommits: this.state.numCommits,
      summary: this.state.summary,
      xbaroffset: explore.state.xbaroffset,
      autoRefresh: explore.state.autoRefresh,
      requestType: this.state.request_type,
      pivotRequest: explore.state.pivotRequest,
      sort: explore.state.sort,
      selected: explore.state.selected,
      horizontal_zoom: explore.state.horizontal_zoom,
      incremental: false,
      domain: this.state.domain, // Always use the domain from ExploreMultiSk's state
      labelMode: LabelMode.Date,
      disable_filter_parent_traces: explore.state.disable_filter_parent_traces,
      plotSummary: this.state.plotSummary,
      highlight_anomalies: this.state.highlight_anomalies,
      enable_chart_tooltip: this.state.enable_chart_tooltip,
      show_remove_all: this.state.show_remove_all,
      use_titles: this.state.use_titles,
      useTestPicker: this.state.useTestPicker,
      use_test_picker_query: false,
      show_google_plot: this.state.show_google_plot,
      enable_favorites: this.canAddFav(),
      hide_paramset: true,
      graph_index: index,
      doNotQueryData: doNotQueryData,
    };
    explore.state = newState;
  }

  private addEmptyGraph(unshift?: boolean): ExploreSimpleSk | null {
    const explore: ExploreSimpleSk = new ExploreSimpleSk(this.useTestPicker);
    const graphConfig = new GraphConfig();
    explore.defaults = this.defaults;
    explore.openQueryByDefault = false;
    explore.navOpen = false;
    // If multi chart has user email, set it for the explore.
    if (this.userEmail) {
      explore.user = this.userEmail;
    }
    if (unshift) {
      this.exploreElements.unshift(explore);
      this.graphConfigs.unshift(graphConfig);
    } else {
      this.exploreElements.push(explore);
      this.graphConfigs.push(graphConfig);
    }
    explore.addEventListener('state_changed', () => {
      const elemState = explore.state;
      let stateChanged = false;
      if (this.graphConfigs[elemState.graph_index].formulas !== elemState.formulas) {
        graphConfig.formulas = elemState.formulas || [];
        stateChanged = true;
      }

      if (this.graphConfigs[elemState.graph_index].queries[0] !== elemState.queries[0]) {
        graphConfig.queries = elemState.queries || [];
        stateChanged = true;
      }

      if (this.graphConfigs[elemState.graph_index].keys !== elemState.keys) {
        graphConfig.keys = elemState.keys || '';
        stateChanged = true;
      }

      if (stateChanged) {
        this.graphConfigs[elemState.graph_index] = graphConfig;
        this.updateShortcutMultiview();
      }
    });
    explore.addEventListener('data-loaded', () => {
      this.checkDataLoaded();
    });

    explore.addEventListener('data-loading', () => {
      this._dataLoading = true;
      this.testPicker?.setReadOnly(true);
    });

    return explore;
  }

  private checkDataLoaded(): void {
    if (this.progress) {
      return;
    }
    if (this.testPicker) {
      if (!this.testPicker.isLoaded() && this.exploreElements.length > 0) {
        this.populateTestPicker(this.exploreElements[0].getParamSet());
      }
      if (this.exploreElements.every((e) => !e.dataLoading)) {
        this.testPicker.setReadOnly(false);
        this._dataLoading = false;
        if (this.stateHasChanged) {
          this.stateHasChanged();
        }
      } else {
        this._dataLoading = true;
        this.testPicker.setReadOnly(true);
        if (this.stateHasChanged) {
          this.stateHasChanged();
        }
      }
    }
  }

  public get state(): State {
    return this._state;
  }

  public set state(v: State) {
    this._state = v;
  }

  /**
   * Get the trace keys for each graph formatted in a 2D string array.
   *
   * In the case of formulas, we extract the base key, since all the operations
   * we will do on these keys (adding to shortcut store, merging into single graph, etc.)
   * are not applicable to function strings. Currently does not support query-based formulas
   * (e.g. count(filter("a=b"))).
   *
   * TODO(@eduardoyap): add support for query-based formulas.
   *
   * Example output given that we're displaying 2 graphs:
   *
   * [
   *  [
   *    ",a=1,b=2,c=3,",
   *    ",a=1,b=2,c=4,"
   *  ],
   *  [
   *    ",a=1,b=2,c=5,"
   *  ]
   * ]
   *
   * @returns {string[][]} - Trace keys.
   */
  private getTracesets(): string[][] {
    const tracesets: string[][] = [];

    this.exploreElements.forEach((elem) => {
      const traceset: string[] = [];

      const formula_regex = new RegExp(/\((,[^)]+,)\)/);
      // Tracesets include traces from Queries and Keys. Traces
      // from formulas are wrapped around a formula string.
      const elemTraceSet = elem.getTraceset(); // This returns { [key: string]: number[] } | null
      if (elemTraceSet) {
        Object.keys(elemTraceSet).forEach((key) => {
          if (key[0] === ',') {
            traceset.push(key);
          } else {
            const match = formula_regex.exec(key);
            if (match) {
              // If it's a formula, extract the base key
              traceset.push(match[1]);
            }
          }
        });
      }
      // Always push the traceset for this element, even if it's empty.
      // This ensures that the length of 'tracesets' matches the number of 'exploreElements'
      // once all elements have reported their tracesets (even if empty).
      tracesets.push(traceset);
    });
    return tracesets;
  }

  /**
   * getHeader returns the columnheader header of the first explore element.
   * @returns
   */
  private getHeader(): (ColumnHeader | null)[] | null {
    return this.exploreElements[0].getHeader();
  }

  /**
   * getCompleteTraceset returns the full traceset consisting of all the tracesets
   * in all explore elements in the current page.
   * @returns
   */
  private getCompleteTraceset(): { [key: string]: number[] } {
    const fullTraceSet: { [key: string]: number[] } = {};

    this.exploreElements.forEach((elem) => {
      const headerLength = elem.getHeader()?.length;
      // Check that header lengths are the same, otherwise ignore.
      if (headerLength === this.getHeader()?.length) {
        const exploreTraceSet = elem.getTraceset();
        if (!exploreTraceSet) {
          return;
        }
        for (const [key, trace] of Object.entries(exploreTraceSet)) {
          fullTraceSet[key] = trace;
        }
      }
    });

    return fullTraceSet;
  }

  /**
   * getAllCommitLinks returns all commit links across all explore elements.
   * @returns
   */
  private getAllCommitLinks(): (CommitLinks | null)[] {
    const commitLinks: (CommitLinks | null)[] = [];
    this.exploreElements.forEach((elem) => {
      const elemLinks = elem.getCommitLinks();
      if (elemLinks.length > 0) {
        commitLinks.push(...elemLinks);
      }
    });

    return commitLinks;
  }

  private getFullAnomalyMap(): AnomalyMap {
    const anomalyMap: AnomalyMap = {};
    this.exploreElements.forEach((elem) => {
      const anomalies = elem.getAnomalyMap();
      Object.keys(anomalies!).forEach((traceId) => {
        const existingEntry = anomalyMap[traceId];
        if (!existingEntry) {
          anomalyMap[traceId] = {};
        }

        const commitMap = anomalies![traceId];
        Object.keys(commitMap!).forEach((commitnumber) => {
          anomalyMap[traceId]![parseInt(commitnumber!)] = commitMap![parseInt(commitnumber!)];
        });
      });
    });
    return anomalyMap;
  }

  private getAnomalyMapForTraces(fullAnomalyMap: AnomalyMap, traces: string[]): AnomalyMap {
    const anomalyMap: AnomalyMap = {};
    traces.forEach((trace) => {
      anomalyMap[trace] = fullAnomalyMap![trace];
    });

    return anomalyMap;
  }

  /**
   * Fetches the Graph Configs in the DB for a given shortcut ID.
   *
   * @param {string} shortcut - shortcut ID to look for in the GraphsShortcut table.
   * @returns - List of Graph Configs matching the shortcut ID in the GraphsShortcut table
   * or undefined if the ID doesn't exist.
   */
  private getConfigsFromShortcut(shortcut: string): Promise<GraphConfig[]> | GraphConfig[] {
    const body = {
      ID: shortcut,
    };

    return fetch('/_/shortcut/get', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((json) => json.graphs)
      .catch(errorMessage);
  }

  /**
   * Creates a shortcut ID for the current Graph Configs and updates the state.
   *
   */
  private updateShortcutMultiview() {
    updateShortcut(this.graphConfigs)
      .then((shortcut) => {
        if (shortcut === '') {
          this.state.shortcut = '';
          this.stateHasChanged!();
          return;
        }
        this.state.shortcut = shortcut;
        this.stateHasChanged!();
      })
      .catch(errorMessage);
  }

  private pageChanged(e: CustomEvent<PaginationSkPageChangedEventDetail>) {
    this._dataLoading = true;
    this.testPicker?.setReadOnly(true);
    this.state.pageOffset = Math.max(
      0,
      this.state.pageOffset + e.detail.delta * this.state.pageSize
    );
    this.stateHasChanged!();
    this.splitGraphs();
  }

  private pageSizeChanged(e: MouseEvent) {
    this._dataLoading = true;
    this.testPicker?.setReadOnly(true);
    this.state.pageSize = +(e.target! as HTMLInputElement).value;
    this.stateHasChanged!();
    this.splitGraphs();
  }

  private async loadAllCharts() {
    if (
      window.confirm(
        'Loading all charts at once may cause performance issues or page crashes. Proceed?'
      )
    ) {
      const pageSize = this.exploreElements.length > 0 ? this.exploreElements.length - 1 : 1;
      this.state.pageSize = pageSize;
      this.state.pageOffset = 0;
      this.stateHasChanged!();
      await this.splitGraphs();
    }
  }
}

define('explore-multi-sk', ExploreMultiSk);
