/**
 * @module module/explore-multi-sk
 * @description <h2><code>explore-multi-sk</code></h2>
 *
 * Page of Perf for exploring data in multiple graphs.
 *
 * User is able to add multiple ExploreSimpleSk instances. The state reflector will only
 * keep track of those properties necessary to add traces to each graph. All other settings,
 * such as the point selected or the beginning and ending range, will be the same
 * for all graphs. For example, passing ?dots=true as a URI parameter will enable
 * dots for all graphs to be loaded. This is to prevent the URL from becoming too long and
 * keeping the logic simple.
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
  Trace,
  TraceSet,
} from '../json';

import '../explore-simple-sk';
import '../favorites-dialog-sk';
import '../test-picker-sk';
import '../../../golden/modules/pagination-sk/pagination-sk';
import '../window/window';

import { $$ } from '../../../infra-sk/modules/dom';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { LoggedIn } from '../../../infra-sk/modules/alogin-sk/alogin-sk';
import { Status as LoginStatus } from '../../../infra-sk/modules/json';
import { FavoritesDialogSk } from '../favorites-dialog-sk/favorites-dialog-sk';
import { PaginationSkPageChangedEventDetail } from '../../../golden/modules/pagination-sk/pagination-sk';
import { PickerFieldSk } from '../picker-field-sk/picker-field-sk';
import { CommitLinks } from '../point-links-sk/point-links-sk';

class State {
  begin: number = Math.floor(Date.now() / 1000 - DEFAULT_RANGE_S);

  end: number = Math.floor(Date.now() / 1000);

  shortcut: string = '';

  showZero: boolean = false;

  dots: boolean = true;

  numCommits: number = 250;

  request_type: RequestType = 0;

  summary: boolean = false;

  pageSize: number = 10;

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
}

export class ExploreMultiSk extends ElementSk {
  private graphConfigs: GraphConfig[] = [];

  private exploreElements: ExploreSimpleSk[] = [];

  private currentPageExploreElements: ExploreSimpleSk[] = [];

  private currentPageGraphConfigs: GraphConfig[] = [];

  private stateHasChanged: (() => void) | null = null;

  private _state: State = new State();

  private addGraphButton: HTMLButtonElement | null = null;

  private graphDiv: Element | null = null;

  private useTestPicker: boolean = false;

  private testPicker: TestPickerSk | null = null;

  private defaults: QueryConfig | null = null;

  private userEmail: string = '';

  private splitByList: PickerFieldSk | null = null;

  private paramKeys: string[] = [];

  private refreshSplitList: boolean = true;

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

        const numElements = this.exploreElements.length;
        let graphConfigs: GraphConfig[] | undefined = [];
        if (state.shortcut !== '') {
          graphConfigs = await this.getConfigsFromShortcut(state.shortcut);
          if (graphConfigs === undefined) {
            graphConfigs = [];
          }
        }

        // This loop helps get rid of extra graphs that aren't part of the
        // current config. A scenario where this occurs is if we have 1 graph,
        // add another graph and then go back in the browser.
        while (this.exploreElements.length > graphConfigs.length) {
          this.exploreElements.pop();
          this.graphConfigs.pop();
          this.graphDiv!.removeChild(this.graphDiv!.lastChild!);
        }

        const validGraphs: GraphConfig[] | undefined = [];
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
        await load();

        google.charts.setOnLoadCallback(() => this.addGraphsToCurrentPage());

        document.addEventListener('keydown', (e) => {
          this.exploreElements.forEach((exp) => {
            exp.keyDown(e);
          });
        });

        // Update the split by dropdown list.
        this.updateSplitByKeys();
        // If a key is specified (eg: directly via url), perform the split
        if (this.state.splitByKeys.length > 0) {
          this.splitGraphs();
          this.refreshSplitList = true;
        }
        if (state.useTestPicker) {
          this.initializeTestPicker();
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

  private openAddFavoriteDialog = async () => {
    const d = $$<FavoritesDialogSk>('#fav-dialog', this) as FavoritesDialogSk;
    await d!.open();
  };

  private static template = (ele: ExploreMultiSk) => html`
    <div id="menu">
      <h1>MultiGraph Menu</h1>
      <test-picker-sk id="test-picker" class="hidden"></test-picker-sk>
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
        ? ``
        : html` <label>
              <span class="prefix">Charts per page</span>
              <input
                @change=${ele.pageSizeChanged}
                type="number"
                .value="${ele.state.pageSize.toString()}"
                min="1"
                max="50"
                title="The number of charts per page." />
            </label>
            <button @click=${ele.loadAllCharts}>Load All Charts</button>`}

      <div
        id="graphContainer"
        @x-axis-toggled=${ele.syncXAxisLabel}
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
    await fetch(`/_/defaults/`, {
      method: 'GET',
    })
      .then(jsonOrThrow)
      .then((json) => {
        this.defaults = json;
      });

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
   * updateSplitByKeys updates the split by dropdown. Also adds the count of splits next
   * to the respective key.
   */
  private updateSplitByKeys() {
    if (this.refreshSplitList) {
      const splitCounts = this.getSplitCountByParam();
      if (splitCounts.size > 0) {
        const splitList: string[] = [];
        this.paramKeys.forEach((paramKey) => {
          const optionText = paramKey + ' (' + splitCounts.get(paramKey) + ')';
          splitList.push(optionText);
        });
        this.splitByList!.options = splitList;
        this.refreshSplitList = false;
      }
    }
  }

  /**
   * This function initializes the split by list by populating
   * it with the available param keys in the dropdown.
   */
  // DEPRECATED: Replaced by splitting via checkboxes.
  private async initializeSplitByList(): Promise<void> {
    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
    await fetch(`/_/initpage/?tz=${tz}`, {
      method: 'GET',
    })
      .then(jsonOrThrow)
      .then((json) => {
        this.paramKeys = Object.keys(json.dataframe.paramset);
        this.splitByList = this.querySelector('#splitby-keys');
        this.splitByList!.label = 'Split By';
        this.splitByList!.options = this.paramKeys;
      });

    // Whenever the user selects a value from the split by list,
    // update the state to reflect it and then split the graphs.
    this.splitByList!.addEventListener('value-changed', (e) => {
      const selectedSplitKey = (e as CustomEvent).detail.value[0];
      // The selectedSplitkey string will contain the split count (eg: "bot (5)"),
      // so we need to extract that out.
      selectedSplitKey.trim();
      const splitByParamKey = selectedSplitKey.split('(')[0] ?? '';
      // Only split if the new selection is different.
      if (!this.state.splitByKeys.includes(splitByParamKey)) {
        this.state.splitByKeys.push(splitByParamKey);
        if (this.stateHasChanged) {
          this.stateHasChanged();
        }
        this.splitGraphs();
      }
    });
  }

  /**
   * getSplitCountByParam returns a map where the key is the param key
   * and the value is the number of split graphs based on the key.
   * @returns
   */
  private getSplitCountByParam(): Map<string, number> {
    const splitCountByParam = new Map<string, number>();
    const traceset: string[] = [];
    this.getTracesets().forEach((ts) => {
      traceset.push(...ts);
    });
    if (traceset.length > 0) {
      this.paramKeys.forEach((paramKey) => {
        const tracesGroupedForKey = this.groupTracesByParamKey(traceset, [paramKey]);
        splitCountByParam.set(paramKey, tracesGroupedForKey.size);
      });
    }

    return splitCountByParam;
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
    let existingGroup: string[] = [];
    if (traceset.length > 0) {
      traceset.forEach((traceId) => {
        const traceParams = new URLSearchParams(fromKey(traceId));
        // If there are no keys, then pass in empty string to group everything.
        if (keys.length === 0) {
          keys = [''];
        }
        keys.forEach((key) => {
          const splitValue = traceParams.get(key);
          existingGroup = groupedTraces.get(splitValue!) ?? [];
          existingGroup.push(traceId);
          groupedTraces.set(splitValue!, existingGroup);
        });
      });
    }
    return groupedTraces;
  }

  /**
   * Splits the graphs based on the split by dropdown selection.
   */
  private splitGraphs(): void {
    const groupedTraces = this.groupTracesBySplitKey();
    if (groupedTraces.size === 0) {
      return;
    }

    // It's the same graph, so let's simply return early.
    if (groupedTraces.size === 1 && this.state.totalGraphs === 1) {
      return;
    }
    const fullTraceSet = this.getCompleteTraceset();
    const header = this.getHeader();
    const commitLinks = this.getAllCommitLinks();
    const fullAnomalyMap = this.getFullAnomalyMap();
    const selectedRange = this.exploreElements[0].getSelectedRange();
    const frameRequests: FrameRequest[] = [];
    const frameResponses: FrameResponse[] = [];
    this.clearGraphs();
    // Create the graph configs for each group.
    groupedTraces.forEach((traces) => {
      this.addEmptyGraph();
      const queries: string[] = [];
      const traceSet: TraceSet = TraceSet({});
      const paramSet: ParamSet = ParamSet({});
      traces.forEach((trace) => {
        queries.push(queryFromKey(trace));
        traceSet[trace] = Trace(fullTraceSet[trace]);
        addParamsToParamSet(paramSet, fromKey(trace));
      });
      const exploreRequest: FrameRequest = {
        queries: queries,
        request_type: this.state.request_type,
        begin: this.state.begin,
        end: this.state.end,
        tz: '',
      };
      const exploreDataResponse: FrameResponse = {
        dataframe: {
          traceset: traceSet,
          header: header,
          paramset: ReadOnlyParamSet(paramSet),
          skip: 0,
          traceMetadata: ExploreSimpleSk.getTraceMetadataFromCommitLinks(traces, commitLinks),
        },
        anomalymap: this.getAnomalyMapForTraces(fullAnomalyMap, traces),
        display_mode: 'display_plot',
        msg: '',
        skps: [],
      };

      frameRequests.push(exploreRequest);
      frameResponses.push(exploreDataResponse);
      // TODO(ashwinpv): Support formulas?
    });

    // Upon the split action, we would want to move to the first page
    // of the split graph set.
    this.state.pageOffset = 0;

    // Now add the graphs that have been configured to the page.
    this.addGraphsToCurrentPage(true);
    for (let i = 0; i < this.exploreElements.length; i++) {
      this.exploreElements[i].UpdateWithFrameResponse(
        frameResponses[i],
        frameRequests[i],
        false,
        selectedRange
      );
    }
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

      this.testPicker!.initializeTestPicker(testPickerParams!, defaultParams);
      if (this.exploreElements.length > 0) {
        this.populateTestPicker(this.exploreElements[0].getParamSet());
      }
      // Event listener to remove the explore object from the list if the user
      // close it in a Multiview window.
      this.addEventListener('remove-explore', (e) => {
        const exploreElemToRemove = (e as CustomEvent).detail.elem as ExploreSimpleSk;
        const indexToRemove = this.exploreElements.findIndex(
          (elem) => elem === exploreElemToRemove
        );

        if (indexToRemove > -1) {
          this.exploreElements.splice(indexToRemove, 1);
          this.graphConfigs.splice(indexToRemove, 1);
          this.state.totalGraphs = this.exploreElements.length;

          // Adjust pagination: if there are no graphs left, reset page offset to 0.
          if (this.state.totalGraphs === 0) {
            this.state.pageOffset = 0;
            this.testPicker!.autoAddTrace = false;
          } else if (this.state.pageSize > 0) {
            // If graphs remain and pageSize is valid, calculate the maximum valid page offset.
            // This prevents being on a page that no longer exists
            // (e.g., if the last item on the last page was removed).
            const maxValidPageOffset = Math.max(
              0,
              (Math.ceil(this.state.totalGraphs / this.state.pageSize) - 1) * this.state.pageSize
            );
            this.state.pageOffset = Math.min(this.state.pageOffset, maxValidPageOffset);
          }

          this.updateShortcutMultiview();
          this.addGraphsToCurrentPage();
        } else {
          this.state.totalGraphs = this.exploreElements.length;
          if (this.stateHasChanged) this.stateHasChanged();
          this.addGraphsToCurrentPage();
        }
        e.stopPropagation();
      });

      // Event listener for when the Test Picker plot button is clicked.
      // This will create a new empty Graph at the top and plot it with the
      // selected test values.
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      this.addEventListener('plot-button-clicked', (e) => {
        const explore = this.addEmptyGraph(true);
        if (explore) {
          this.addGraphsToCurrentPage(true);
          const query = this.testPicker!.createQueryFromFieldData();
          explore.addFromQueryOrFormula(true, 'query', query, '');
          this.refreshSplitList = true;
          if (this.testPicker) {
            this.testPicker.autoAddTrace = true;
          }
        }
        this.updateSplitByKeys();
      });

      this.addEventListener('split-by-changed', (e) => {
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
        if (this.stateHasChanged) {
          this.stateHasChanged();
        }
        this.splitGraphs();
      });

      // Event listener for when the Test Picker plot button is clicked.
      // This will create a new empty Graph at the top and plot it with the
      // selected test values.
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      this.addEventListener('add-to-graph', async (e) => {
        const query = (e as CustomEvent).detail.query;
        let explore: ExploreSimpleSk;
        if (this.currentPageExploreElements.length === 0) {
          const newExplore = this.addEmptyGraph(true);
          if (newExplore) {
            this.addGraphsToCurrentPage(true); // Pass true to prevent immediate data query
            explore = newExplore;
          } else {
            return;
          }
        } else {
          explore = this.currentPageExploreElements[0];
          this.currentPageExploreElements.splice(1);
          this.currentPageGraphConfigs.splice(1);
          this.exploreElements.splice(1);
          this.graphConfigs.splice(1);
          this.state.totalGraphs = this.exploreElements.length;
        }
        await explore.addFromQueryOrFormula(true, 'query', query, '');
        this.splitGraphs();
        this.refreshSplitList = true;
      });

      // Event listener for when the "Query Highlighted" button is clicked.
      // It will populate the Test Picker with the keys from the highlighted
      // trace.
      this.addEventListener('populate-query', (e) => {
        this.populateTestPicker((e as CustomEvent).detail);
      });
    }
  }

  private async populateTestPicker(paramSet: { [key: string]: string[] }) {
    const paramSets: ParamSet = ParamSet({});

    const timeoutMs = 20000; // Timeout for waiting for non-empty tracesets.
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
    this.testPicker!.scrollIntoView();
  }

  private clearGraphs() {
    this.exploreElements = [];
    this.graphConfigs = [];
  }

  private emptyCurrentPage(): void {
    while (this.graphDiv!.hasChildNodes()) {
      this.graphDiv!.removeChild(this.graphDiv!.lastChild!);
    }
    this.currentPageExploreElements = [];
    this.currentPageGraphConfigs = [];
  }

  private addGraphsToCurrentPage(doNotQueryData: boolean = false): void {
    this.state.totalGraphs = this.exploreElements.length;
    this.emptyCurrentPage();
    const startIndex = this.state.pageOffset;
    let endIndex = startIndex + this.state.pageSize - 1;
    if (this.exploreElements.length <= endIndex) {
      endIndex = this.exploreElements.length - 1;
    }

    for (let i = startIndex; i <= endIndex; i++) {
      this.graphDiv!.appendChild(this.exploreElements[i]);
      this.currentPageExploreElements.push(this.exploreElements[i]);
      this.currentPageGraphConfigs.push(this.graphConfigs[i]);
    }

    this.currentPageExploreElements.forEach((elem, i) => {
      const graphConfig = this.currentPageGraphConfigs[i];
      this.addStateToExplore(elem, graphConfig, doNotQueryData);
    });

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

  private syncChartSelection(e: CustomEvent<PlotSelectionEventDetails>): void {
    const graphs = this.graphDiv!.querySelectorAll('explore-simple-sk');
    if (!e.detail.value) {
      return;
    }

    // Default behavior for non-split views or for pan/zoom actions.
    graphs.forEach((graph, i) => {
      // only update graph that isn't selected
      if (i !== e.detail.graphNumber && e.detail.offsetInSeconds === undefined) {
        (graph as ExploreSimpleSk).updateSelectedRangeWithPlotSummary(e.detail.value);
      }
      // Sync the selection by extending the range for other graphs.
      // If a sync is requested when only one graph then ignore.
      if (graphs.length > 1 && e.detail.offsetInSeconds !== undefined) {
        (graph as ExploreSimpleSk).extendRange(e.detail.value, e.detail.offsetInSeconds);
      }
    });
  }

  private syncXAxisLabel(e: CustomEvent): void {
    const graphs = this.graphDiv!.querySelectorAll('explore-simple-sk');
    graphs.forEach((graph) => {
      (graph as ExploreSimpleSk).switchXAxis(e.detail);
    });
  }

  private addStateToExplore(
    explore: ExploreSimpleSk,
    graphConfig: GraphConfig,
    doNotQueryData: boolean
  ) {
    const newState: ExploreState = {
      formulas: graphConfig.formulas || [],
      queries: graphConfig.queries || [],
      keys: graphConfig.keys || '',
      begin: explore.state.begin || this.state.begin,
      end: explore.state.end || this.state.end,
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
      _incremental: false,
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
      graph_index: this.exploreElements.indexOf(explore),
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

      graphConfig.formulas = elemState.formulas || [];

      graphConfig.queries = elemState.queries || [];

      graphConfig.keys = elemState.keys || '';

      this.updateShortcutMultiview();
    });
    explore.addEventListener('data-loaded', () => {
      this.refreshSplitList = true;
      this.updateSplitByKeys();
    });

    return explore;
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
      const exploreTraceSet = elem.getTraceset();
      if (!exploreTraceSet) {
        return;
      }
      Object.keys(exploreTraceSet!).forEach((key) => {
        fullTraceSet[key] = exploreTraceSet![key];
      });
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
  private getConfigsFromShortcut(shortcut: string): Promise<GraphConfig[]> | undefined {
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
    this.state.pageOffset = Math.max(
      0,
      this.state.pageOffset + e.detail.delta * this.state.pageSize
    );
    this.stateHasChanged!();
    this.addGraphsToCurrentPage();
  }

  private pageSizeChanged(e: MouseEvent) {
    this.state.pageSize = +(e.target! as HTMLInputElement).value;
    this.stateHasChanged!();
    this.addGraphsToCurrentPage();
  }

  private updatePageForNewExplore() {
    // Check if there is space left on the current page
    if (this.graphDiv!.childElementCount === this.state.pageSize) {
      // We will have to add another page since the current one is full.
      // Go to the next page.
      this.pageChanged(
        new CustomEvent<PaginationSkPageChangedEventDetail>('page-changed', {
          detail: {
            delta: 1,
          },
          bubbles: true,
        })
      );
    } else {
      // Re-render the page
      this.addGraphsToCurrentPage();
    }
  }

  private loadAllCharts() {
    if (
      window.confirm(
        'Loading all charts at once may cause performance issues or page crashes. Proceed?'
      )
    ) {
      this.state.pageSize = this.state.totalGraphs;
      this.state.pageOffset = 0;
      this.stateHasChanged!();
      this.addGraphsToCurrentPage();
    }
  }
}

define('explore-multi-sk', ExploreMultiSk);
