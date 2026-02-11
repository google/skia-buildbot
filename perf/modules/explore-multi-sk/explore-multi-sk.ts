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
  LabelMode,
} from '../explore-simple-sk/explore-simple-sk';
import { GraphConfig, updateShortcut } from '../common/graph-config';
import { PlotSelectionEventDetails } from '../plot-google-chart-sk/plot-google-chart-sk';
import { load } from '@google-web-components/google-chart/loader';
import { TestPickerSk } from '../test-picker-sk/test-picker-sk';

import { addParamSet, addParamsToParamSet, fromKey, queryFromKey } from '../paramtools';
import {
  ParamSet as QueryParamSet,
  fromParamSet,
  toParamSet,
} from '../../../infra-sk/modules/query';
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
  TraceMetadata,
} from '../json';
import { CountMetric, SummaryMetric, telemetry } from '../telemetry/telemetry';

import '../../../elements-sk/modules/spinner-sk';
import '../explore-simple-sk';
import '../favorites-dialog-sk';
import '../test-picker-sk';
import '../../../golden/modules/pagination-sk/pagination-sk';
import '../window/window';

import { LoggedIn } from '../../../infra-sk/modules/alogin-sk/alogin-sk';
import { Status as LoginStatus } from '../../../infra-sk/modules/json';
import { PaginationSkPageChangedEventDetail } from '../../../golden/modules/pagination-sk/pagination-sk';
import { CommitLinks } from '../point-links-sk/point-links-sk';
import { MISSING_VALUE_SENTINEL } from '../const/const';
import { DataService, DataServiceError } from '../data-service';

const CACHE_KEY_EVEN_X_AXIS_SPACING = 'even_xaxis_spacing';

export class State {
  begin: number = -1;

  end: number = -1;

  shortcut: string = '';

  showZero: boolean = false;

  dots: boolean = true;

  autoRefresh: boolean = false;

  numCommits: number = 250;

  request_type: RequestType = 1;

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

  evenXAxisSpacing: 'true' | 'false' | 'use_cache' = 'use_cache';

  // TODO(eduardoyap): Handle browser history changes correctly in manual_plot_mode.
  // TODO(eduardoyap): Ensure new graphs in manual_plot_mode sync time ranges.
  manual_plot_mode: boolean = false;
}

export class ExploreMultiSk extends ElementSk {
  private allGraphConfigs: GraphConfig[] = [];

  private allFrameResponses: FrameResponse[] = [];

  private allFrameRequests: FrameRequest[] = [];

  private mainGraphSelectedRange: { begin: number; end: number } | null = null;

  private exploreElements: ExploreSimpleSk[] = [];

  private currentPageExploreElements: ExploreSimpleSk[] = [];

  private stateHasChanged: (() => void) | null = null;

  private _state: State = new State();

  private graphDiv: Element | null = null;

  private useTestPicker: boolean = false;

  private testPicker: TestPickerSk | null = null;

  private defaults: QueryConfig | null = null;

  private userEmail: string = '';

  private _dataLoading: boolean = false;

  private progress: string = '';

  private initialLoadStartTime: number = 0;

  private loadTrigger: string = '';

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

  private _onStateChangedInUrl = async (hintableState: HintableObject) => {
    this.initialLoadStartTime = performance.now();
    this.loadTrigger = 'direct_link';
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

    const validGraphs: GraphConfig[] = [];
    if (state.splitByKeys.length > 0 && graphConfigs.length > 0) {
      validGraphs.push(new GraphConfig());
      this.addEmptyGraph();
    }
    for (let i = 0; i < graphConfigs.length; i++) {
      if (
        graphConfigs[i].formulas.length > 0 ||
        graphConfigs[i].queries.length > 0 ||
        graphConfigs[i].keys !== ''
      ) {
        // Merge queries and formulas into the first graph if splitting.
        if (state.splitByKeys.length > 0) {
          // Ensure the master query exists and is a single string.
          if (validGraphs[0].queries.length === 0) {
            validGraphs[0].queries.push('');
          }
          const aggregatedParams = new URLSearchParams(validGraphs[0].queries[0]);

          graphConfigs[i].queries.forEach((q) => {
            const incomingParams = new URLSearchParams(q);
            incomingParams.forEach((value, key) => {
              // Check if this exact key-value pair already exists.
              const existingValues = aggregatedParams.getAll(key);
              if (!existingValues.includes(value)) {
                aggregatedParams.append(key, value);
              }
            });
          });
          // URLSearchParams.toString() encodes spaces as '+', but '%20' is generally preferred
          // and safer for consistent URL handling, so we replace them.
          validGraphs[0].queries[0] = aggregatedParams.toString().replace(/\+/g, '%20');

          // Handle formulas (simple duplicate check is fine here).
          graphConfigs[i].formulas.forEach((f) => {
            if (!validGraphs[0].formulas.includes(f)) {
              validGraphs[0].formulas.push(f);
            }
          });

          // Handle keys (simple duplicate check is fine here).
          if (graphConfigs[i].keys && !validGraphs[0].keys.includes(graphConfigs[i].keys)) {
            if (validGraphs[0].keys) {
              validGraphs[0].keys += ' ';
            }
            validGraphs[0].keys += graphConfigs[i].keys;
          }
        } else {
          if (i >= numElements) {
            this.addEmptyGraph();
          }
          validGraphs.push(graphConfigs[i]);
        }
      }
    }
    this.allGraphConfigs = validGraphs;

    // This loop removes graphs that are not in the current config.
    // This can happen if you add a graph and then use the browser's back button.
    while (this.exploreElements.length > this.allGraphConfigs.length) {
      this.exploreElements.pop();
      // Ensure graphDiv exists and has children before removing.
      if (this.graphDiv && this.graphDiv.lastChild) {
        this.graphDiv.removeChild(this.graphDiv.lastChild);
      }
    }

    this.state = state;
    if (state.useTestPicker) {
      await this.initializeTestPicker();
    }
    await load();

    await this.renderCurrentPage(false);
    // If a key is specified on initial load, we must wait for the
    // shortcut's graphs to load their data before we can split them.
    if (this.state.splitByKeys.length > 0 && this.exploreElements.length > 0) {
      this.setProgress('Loading graphs...');
      this._dataLoading = true;
      // Wait for the graph to start loading and then finish.
      // We use requestComplete which is more robust.
      await this.exploreElements[0].requestComplete;

      // Now that the data is loaded, we can split.
      await this.splitGraphs(false); // showErrorIfLoading = false
      this.setProgress('');
      this.checkDataLoaded();
    }

    document.addEventListener('keydown', (e) => {
      this.exploreElements.forEach((exp) => {
        exp.keyDown(e);
      });
    });
  };

  // Event listener to remove the explore object from the list if the user
  // close it in a Multiview window.
  private _onRemoveExplore = (e: Event) => {
    const detail = (e as CustomEvent).detail;
    if (!detail || !detail.elem) {
      console.warn('remove-explore event received without detail.elem; ignoring.');
      return;
    }
    const exploreElemToRemove = detail.elem as ExploreSimpleSk;

    if (this.state.manual_plot_mode) {
      this.removeExplore(exploreElemToRemove);
      e.stopPropagation();
      return;
    }

    this._dataLoading = true;
    this.testPicker?.setReadOnly(true);

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
  };

  // Event listener for when the Test Picker plot button is clicked.
  // This will create a new empty Graph at the top and plot it with the
  // selected test values.
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  private _onPlotButtonClicked = async (e: Event) => {
    this._dataLoading = true;
    this.testPicker?.setReadOnly(true);
    this.setProgress('Loading graphs...');
    try {
      this.initialLoadStartTime = performance.now();
      this.loadTrigger = 'plot_button_clicked';
      if (this.state.splitByKeys.length === 0) {
        // Just load single graph.
        // We add the graph to the config, but we don't use the returned element
        // because renderCurrentPage will create a new one.
        this.addEmptyGraph(true);
        if (this.allGraphConfigs.length === 0) {
          return;
        }
        if (this.exploreElements.length > 0 && this._dataLoading) {
          await new Promise<void>((resolve) => {
            const check = () => {
              if (!this.exploreElements[0].spinning) {
                resolve();
              } else {
                setTimeout(check, 100); // Poll every 100ms.
              }
            };
            check();
          });
        }
        const shouldPreserveExistingData = this.state.manual_plot_mode;
        this.renderCurrentPage(shouldPreserveExistingData);
        // Get the actual element that was just added to the DOM.
        // Since we unshifted (true), it should be at index 0 of the current page.
        const newExplore = this.currentPageExploreElements[0];
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
        this.addEmptyGraph(true);
        this.renderCurrentPage(false);
        const mainGraph = this.currentPageExploreElements[0];
        if (!mainGraph) {
          return;
        }
        await mainGraph.requestComplete;

        const CHUNK_SIZE = 5;
        const groupdToLoadInChunks = Math.min(this.state.pageSize, groups.length);
        for (let i = 0; i < groupdToLoadInChunks; ) {
          // The first chunk is always of size 1 - this is to avoid showing the primary
          // graph / "unsplit" mode.
          const chunkSize = CHUNK_SIZE;
          const endGroupIndex = Math.min(i + chunkSize, groupdToLoadInChunks);
          const chunk = groups.slice(i, endGroupIndex);
          if (chunk.length === 0) {
            break; // No more groups to process.
          }

          this.setProgress(`Loading graphs ${i + 1}-${endGroupIndex} of ${groups.length}`);
          await mainGraph.addFromQueryOrFormula(
            /*replace=*/ false,
            'query',
            fromParamSet(this.mergeParamSets(chunk)),
            '',
            '',
            // Important! Do not load extended range data. Otherwise it produces a lot of
            // queries fetching the same data + creates concurrency issues.
            /*loadExtendedRange=*/ false
          );
          await mainGraph.requestComplete;
          await this.splitGraphs(/*showErrorIfLoading=*/ false, /*splitIfOnlyOneGraph=*/ true);

          i = endGroupIndex;
        }

        // We were postponing loading more data until all the graphs are ready. Now it's time.
        this.setProgress(`Loading more data for all graphs...`);
        if (groups.length > groupdToLoadInChunks) {
          // Note that we load all the graphs, even if they don't fit in one page. It slows down
          // the initial load, but speeds up page navigation and "Load All Graphs".
          await mainGraph.addFromQueryOrFormula(
            /*replace=*/ true,
            'query',
            fromParamSet(this.mergeParamSets(groups)),
            '',
            '',
            /*loadExtendedRange=*/ true
          );
        } else {
          await mainGraph.loadExtendedRangeData(mainGraph.getSelectedRange()!);
        }
        await mainGraph.requestComplete;
        await this.splitGraphs();
      }
    } catch (err: unknown) {
      errorMessage(err as string);
    } finally {
      this.updateShortcutMultiview();
      this.setProgress('');
      this.checkDataLoaded();
    }

    // Prevent auto-add trace behavior in independent mode
    if (this.testPicker && !this.state.manual_plot_mode) {
      this.testPicker.autoAddTrace = true;
    }
  };

  // Event listener for when the Test Picker plot button is clicked.
  // This will create a new empty Graph at the top and plot it with the
  // selected test values.
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  private _onAddToGraph = async (e: Event) => {
    this.initialLoadStartTime = performance.now();
    this.loadTrigger = 'picker_add_trace';
    const query = (e as CustomEvent).detail.query;
    // Query is the same as the first graph, so do nothing.
    if (this.allGraphConfigs.length > 0 && query === this.allGraphConfigs[0].queries[0]) {
      return;
    }
    let explore: ExploreSimpleSk;
    this._dataLoading = true;
    this.testPicker?.setReadOnly(true);
    if (this.currentPageExploreElements.length === 0) {
      this.addEmptyGraph(true);
      // We don't use the returned element, but rely on renderCurrentPage to create
      // the element and put it in currentPageExploreElements.
      if (this.allGraphConfigs.length > 0) {
        if (this.exploreElements.length > 0 && this._dataLoading) {
          await this.exploreElements[0].requestComplete;
        }
        this.renderCurrentPage(true);
        // The newly added graph is at index 0.
        explore = this.currentPageExploreElements[0];
      } else {
        return;
      }
    } else {
      // Reset pagination and re-render to ensure we are targeting the main graph (index 0).
      // This collapses any existing split view back to a single master graph.
      this.state.pageOffset = 0;

      this.exploreElements.splice(1);
      this.allGraphConfigs.splice(1);
      this.allFrameRequests.splice(1);
      this.allFrameResponses.splice(1);
      this.state.totalGraphs = this.exploreElements.length;

      // Ensure the main graph is in the DOM so it can process data.
      await this.renderCurrentPage(false);
      explore = this.currentPageExploreElements[0];
      if (!explore) {
        return;
      }
      explore.state.doNotQueryData = false;
    }
    await explore.addFromQueryOrFormula(true, 'query', query, '', '', false);
    await this.splitGraphs();
  };

  private _onRemoveTrace = async (e: Event) => {
    try {
      const param = (e as CustomEvent).detail.param as string;
      const values = (e as CustomEvent).detail.value as string[];
      const query = (e as CustomEvent).detail.query as string[];

      if (values.length === 0) {
        this.resetGraphs();
        this.checkDataLoaded();
        return;
      }
      this._dataLoading = true;
      this.testPicker?.setReadOnly(true);

      // Capture the split state before yielding, as a subsequent split-by-changed event
      // (triggered by the same action) might update the state while we are yielded.
      const currentSplitByKeys = [...this.state.splitByKeys];

      // Yield to the browser to update the UI
      await new Promise((resolve) => setTimeout(resolve, 0));

      const traceSet = this.getCompleteTraceset();
      const tracesToRemove: string[] = [];
      const queriesToRemove: string[] = [];

      // Check through all existing TraceSets and find matches.
      Object.keys(traceSet).forEach((trace) => {
        const traceParams = fromKey(trace);
        const traceVal = traceParams[param] || MISSING_VALUE_SENTINEL;
        if (values.includes(traceVal)) {
          // Load remove array and delete from existing traceSet.
          tracesToRemove.push(trace);
          let query = queryFromKey(trace);
          if (this.defaults?.include_params) {
            const paramSet = toParamSet(query);
            const filteredParamSet = ParamSet({});
            const includeParams = this.defaults.include_params;
            for (const key in paramSet) {
              if (includeParams.includes(key)) {
                filteredParamSet[key] = paramSet[key];
              }
            }
            query = fromParamSet(filteredParamSet);
          }
          queriesToRemove.push(query);
          delete traceSet[trace];
        }
      });

      if (Object.keys(traceSet).length === 0) {
        this.emptyCurrentPage();
        await this.checkDataLoaded(); // Ensure readonly is reset
        return;
      }

      // Remove the traces from the current page explore elements.
      const elemsToRemove: ExploreSimpleSk[] = [];

      const updatePromises = this.exploreElements.map((elem, index) => {
        // If the graph has no queries AND no data, it is a zombie and should be removed.
        // We only remove split graphs (index > 0). The main graph (index 0) should persist.
        const hasTraces = elem.getTraceset() && Object.keys(elem.getTraceset()!).length > 0;
        if (index > 0 && (!elem.state.queries || elem.state.queries.length === 0) && !hasTraces) {
          elemsToRemove.push(elem);
          return Promise.resolve();
        }

        const hasQueryToRemove =
          elem.state.queries && queriesToRemove.some((qr) => elem.state.queries.includes(qr));

        const traceset = elem.getTraceset() as TraceSet;
        const hasTraceToRemove = traceset && tracesToRemove.some((key) => traceset[key]);

        // Only proceed with updates if the element is affected.
        if (
          hasQueryToRemove ||
          (elem.state.queries?.length && query !== undefined) ||
          hasTraceToRemove
        ) {
          elem.state.doNotQueryData = true;
          let keysWereRemoved = false;
          if (elem.state.queries.length > 0 && queriesToRemove.length > 0) {
            const queryCount = elem.state.queries.length;
            // Remove any queries that match queriesToRemove.
            elem.state.queries = elem.state.queries.filter((q) => !queriesToRemove.includes(q));
            // If queries were removed, we definitely need to remove keys.
            if (elem.state.queries.length !== queryCount) {
              keysWereRemoved = true;
            }
          }
          let shouldRemoveGraph = false;

          // When one query exists, check if param/value matches and replace with new query.
          if (elem.state.queries.length === 1 && !keysWereRemoved) {
            const queryToRemove = Array.isArray(query) ? query[0] : query;
            // If we are splitting by this param, we should remove the graph (clear queries)
            // instead of updating it to the new master query.
            // Prioritize the state passed from the event (source of truth).
            let isSplitParam = (e as CustomEvent).detail.isSplit;

            if (isSplitParam === undefined) {
              isSplitParam = currentSplitByKeys.includes(param);
            }

            values.forEach((v) => {
              const queryStr = elem.state.queries[0];
              let match = false;
              if (queryStr) {
                match = queryStr.includes(`${param}=${v}`);
                if (!match) {
                  const p = new URLSearchParams(queryStr);
                  if (p.get(param) === v) {
                    match = true;
                  }
                }
              }

              if (match || queryStr === queryToRemove || !queryStr) {
                if (isSplitParam) {
                  shouldRemoveGraph = true;

                  elem.state.queries = [];
                } else {
                  elem.state.queries = [queryToRemove];
                }
                keysWereRemoved = true;
              }
            });
          }

          if (
            !shouldRemoveGraph &&
            (keysWereRemoved || tracesToRemove.some((key) => traceset[key]))
          ) {
            elem.removeKeys(tracesToRemove, true);
            const newTraceset = elem.getTraceset();
            if (!newTraceset || Object.keys(newTraceset).length === 0) {
              elem.state.queries = [];
            }
          }
          if (index > 0 && (elem.state.queries.length === 0 || shouldRemoveGraph)) {
            elemsToRemove.push(elem);
            return Promise.resolve();
          }
          const elemTraceset = elem.getTraceset() as TraceSet;
          const elemHeader = elem.getHeader();
          // Default to filtering the global set by the element's existing keys (minus
          // removed ones). This ensures we preserve the split state of the graph while
          // getting fresh data.
          let updatedTraceset = TraceSet({});
          Object.keys(elemTraceset).forEach((key) => {
            if (
              traceSet[key] &&
              !tracesToRemove.includes(key) &&
              this.shouldKeepTrace(key, param, values)
            ) {
              updatedTraceset[key] = Trace(traceSet[key]);
            }
          });
          let headerToUse = this.getHeader();

          // We can compare the data length of a common trace to determine which set is "better".
          // We pick the first key from the local traceset that isn't being removed.
          const commonKey = Object.keys(elemTraceset).find((key) =>
            this.shouldKeepTrace(key, param, values)
          );

          if (commonKey && traceSet[commonKey] && elemTraceset[commonKey]) {
            if (elemTraceset[commonKey].length > traceSet[commonKey].length) {
              // Local data is better. Filter out the removed trace from the local set.
              const filteredLocalTraceset = TraceSet({});
              Object.keys(elemTraceset).forEach((key) => {
                if (this.shouldKeepTrace(key, param, values)) {
                  filteredLocalTraceset[key] = elemTraceset[key];
                }
              });
              updatedTraceset = filteredLocalTraceset;
              headerToUse = elemHeader;
            }
          }

          const params: ParamSet = ParamSet({});
          Object.keys(updatedTraceset).forEach((trace) => {
            addParamsToParamSet(params, fromKey(trace));
          });

          // If we filtered the traceset but didn't update the queries (because the
          // removal logic didn't match), we should update the queries to reflect the
          // remaining data. This prevents future fetches from bringing back the
          // removed trace and ensures consistency.
          // We only do this if we are not clearing the graph.
          if (Object.keys(updatedTraceset).length > 0) {
            // We construct a new query from the remaining params.
            // However, we should respect the include_params defaults if possible, to
            // avoid over-specifying. But here we want to be safe and match the data.
            // Let's use the params derived from the traceset.
            const newQuery = fromParamSet(params);
            elem.state.queries = [newQuery];
          }

          // Check if any remaining trace is missing the parameter or has an empty value.
          // If so, we need to disable parent trace filtering on the backend.
          const hasMissingParam = Object.keys(updatedTraceset).some((key) => {
            const p = fromKey(key);
            return !p[param] || p[param] === '';
          });

          // Update the graph with the new traceSet and params.
          const updatedRequest: FrameRequest = {
            queries: elem.state.queries,
            request_type: this.state.request_type,
            begin: this.state.begin,
            end: this.state.end,
            tz: '',
            disable_filter_parent_traces:
              elem.state.queries.some((q) => q.includes(MISSING_VALUE_SENTINEL)) || hasMissingParam,
          };
          const updatedResponse: FrameResponse = {
            dataframe: {
              traceset: updatedTraceset,
              header: headerToUse ? [...headerToUse] : null,
              paramset: ReadOnlyParamSet(params),
              skip: 0,
              traceMetadata: ExploreSimpleSk.getTraceMetadataFromCommitLinks(
                Object.keys(updatedTraceset),
                elem.getCommitLinks()
              ),
            },
            anomalymap: this.getAnomalyMapForTraces(
              this.getFullAnomalyMap(),
              Object.keys(updatedTraceset)
            ),
            display_mode: 'display_plot',
            msg: '',
            skps: [],
          };
          this.allFrameResponses[index] = updatedResponse;
          this.allFrameRequests[index] = updatedRequest;
          return elem.UpdateWithFrameResponse(
            updatedResponse,
            updatedRequest,
            true,
            this.exploreElements[0].getSelectedRange(),
            false,
            true
          );
        }
        return Promise.resolve();
      });

      await Promise.all(updatePromises);

      // This should be outside the map, so it always runs after promises.
      elemsToRemove.forEach((elem) => {
        this.removeExplore(elem);
      });

      this.exploreElements.forEach((elem, i) => {
        if (this.allGraphConfigs[i]) {
          // Add check to prevent error
          this.allGraphConfigs[i].queries = elem.state.queries ?? [];
        }
      });

      this.updateShortcutMultiview();
      await this.checkDataLoaded();
    } catch (error) {
      console.error(error);
      // Ensure we reset loading state on error
      this._dataLoading = false;
      this.testPicker?.setReadOnly(false);
    }
  };

  /**
   * Helper to check if a trace should be kept based on the removal criteria.
   * Returns true if the trace does NOT match the parameter and value being removed.
   */
  private shouldKeepTrace(key: string, param: string, values: string[]): boolean {
    const traceParams = fromKey(key);
    // For info on default key behavior, see: go/pdeio-perf-default-key-doc
    const traceVal = traceParams[param] || MISSING_VALUE_SENTINEL;
    return !values.includes(traceVal);
  }

  // Event listener for when the "Query Highlighted" button is clicked.
  // It will populate the Test Picker with the keys from the highlighted
  // trace.
  private _onPopulateQuery = async (e: Event) => {
    await this.populateTestPicker((e as CustomEvent).detail);
  };

  // Event listener for when the "Even X Axis Spacing" toggle is clicked.
  // It will sync the state across all the graphs.
  private _onEvenXAxisSpacingChanged = (e: Event) => {
    const detail = (e as CustomEvent).detail;
    localStorage.setItem(CACHE_KEY_EVEN_X_AXIS_SPACING, String(detail.value));

    this.exploreElements.forEach((elem) => {
      if (elem !== e.target) {
        elem.setUseDiscreteAxis(detail.value);
      }
    });
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
      this._onStateChangedInUrl
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
        @remove-explore=${ele.removeExploreEvent}
        @range-changing-in-multi=${ele.syncExtendRange}
        @selection-changing-in-multi=${ele.syncChartSelection}
        @x-axis-toggled=${ele.syncXAxisLabel}
        @even-x-axis-spacing-changed=${ele._onEvenXAxisSpacingChanged}></div>
      <pagination-sk
        offset=${ele.state.pageOffset}
        page_size=${ele.state.pageSize}
        total=${ele.state.totalGraphs}
        @page-changed=${ele.pageChanged}>
      </pagination-sk>
      <div id="bottom-spacer"></div>
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
      this.defaults = await DataService.getInstance().getDefaults();
    } catch (error: unknown) {
      console.error('Error fetching defaults:', error);
      const e = error as { message?: string };
      errorMessage(`Failed to load default configuration: ${e.message || error}`);
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
      // Use allGraphConfigs since it holds all graph configurations.
      // Index 0 corresponds to the main/primary graph.
      if (this.allGraphConfigs.length > 0 && this.allGraphConfigs[0].queries) {
        queries.push(...this.allGraphConfigs[0].queries);
      }
    }

    let begin = this.state.begin;
    let end = this.state.end;
    if (begin === -1 || end === -1) {
      const now = Math.floor(Date.now() / 1000);
      begin = now - DEFAULT_RANGE_S;
      end = now;
    }

    const request: FrameRequest = {
      queries: queries,
      request_type: this.state.request_type,
      begin: begin,
      end: end,
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

    let traceMetadata: TraceMetadata[];
    let anomalyMap: AnomalyMap;
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
    } else {
      // collect data for all traces.
      traceMetadata = ExploreSimpleSk.getTraceMetadataFromCommitLinks(
        Object.keys(fullTraceSet),
        commitLinks
      );
      anomalyMap = this.getAnomalyMapForTraces(fullAnomalyMap, Object.keys(fullTraceSet));
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
  private async splitGraphs(
    _showErrorIfLoading: boolean = true,
    splitIfOnlyOneGraph: boolean = false
  ): Promise<void> {
    const groupedTraces = this.groupTracesBySplitKey();
    // If no traces, we usually return. But if we are unsplitting (keys empty),
    // we must proceed to clear the split graphs and show the single main graph.
    if (groupedTraces.size === 0 && this.state.splitByKeys.length > 0) {
      this.checkDataLoaded();
      return;
    }

    if (this.state.totalGraphs === 1) {
      // If there is only one graph with no split or only one trace, then do nothing.
      const groupedLength = Array.from(groupedTraces.values()).reduce(
        (sum, v) => sum + v.length,
        0
      );
      // We removed 'this.state.splitByKeys.length === 0' because if we are transitioning
      // to Unsplit (length 0), we MUST proceed to clear the split graph (totalGraphs=1).
      if (!splitIfOnlyOneGraph && groupedLength === 1) {
        await this.checkDataLoaded();
        return;
      }
    }

    /* TODO(crbug/447196357): Remove or re-enable if unable to fix loading state bug.
    if (this.exploreElements.length > 0 && this._dataLoading === true) {
      if (showErrorIfLoading) {
        errorMessage('Data is still loading, please wait...', 3000);
      }
      await this.exploreElements[0].requestComplete;
    }
    */

    this.mainGraphSelectedRange = this.exploreElements[0].getSelectedRange();

    // Create the main graph config containing all trace data.
    const mainRequest: FrameRequest = this.createFrameRequest();
    const mainResponse: FrameResponse = this.createFrameResponse();

    // Pre-calculate requests and responses for all groups BEFORE clearing graphs.
    // clearGraphs() wipes this.exploreElements, which createFrameResponse depends on for data.
    const splitData =
      this.state.splitByKeys.length === 0
        ? []
        : Array.from(groupedTraces.values()).map((traces) => ({
            traces,
            request: this.createFrameRequest(traces),
            response: this.createFrameResponse(traces),
          }));

    this.allFrameRequests = [mainRequest];
    this.allFrameResponses = [mainResponse];

    this.clearGraphs();

    // Add the main graph back so it occupies index 0.
    this.addEmptyGraph();
    // Ensure the main graph config (index 0) has the queries for all data.
    if (this.allGraphConfigs.length > 0) {
      this.allGraphConfigs[0].queries = mainRequest.queries || [];
    }

    // Create the graph configs for each group using pre-calculated data.
    splitData.forEach(({ request, response }, i) => {
      this.addEmptyGraph();

      const graphConfig = new GraphConfig();
      graphConfig.queries = request.queries ?? [];
      // Main graph config is always at index 0.
      this.allGraphConfigs[i + 1] = graphConfig;

      this.allFrameRequests.push(request);
      this.allFrameResponses.push(response);
    });

    // Now add the graphs that have been configured to the page.
    this.renderCurrentPage();

    if (this.stateHasChanged) {
      this.stateHasChanged();
    }
    await this.checkDataLoaded();
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

      const readOnly = this.state.manual_plot_mode ? false : this.exploreElements.length > 0;

      // Pass the manual_plot_mode flag to force manual plotting if true.
      this.testPicker!.initializeTestPicker(
        testPickerParams!,
        defaultParams,
        readOnly,
        this.state.manual_plot_mode
      );
      this._render();
    }

    this.removeEventListener('remove-explore', this._onRemoveExplore);
    this.addEventListener('remove-explore', this._onRemoveExplore);

    this.removeEventListener('plot-button-clicked', this._onPlotButtonClicked);
    this.addEventListener('plot-button-clicked', this._onPlotButtonClicked);

    this.removeEventListener('split-by-changed', this._onSplitByChanged);
    this.addEventListener('split-by-changed', this._onSplitByChanged);

    this.removeEventListener('add-to-graph', this._onAddToGraph);
    this.addEventListener('add-to-graph', this._onAddToGraph);

    this.removeEventListener('remove-trace', this._onRemoveTrace);
    this.addEventListener('remove-trace', this._onRemoveTrace);

    this.removeEventListener('populate-query', this._onPopulateQuery);
    this.addEventListener('populate-query', this._onPopulateQuery);
  }

  private async populateTestPicker(paramSet: { [key: string]: string[] }) {
    const paramSets: ParamSet = ParamSet({});

    const timeoutMs = 50000; // Timeout for waiting for non-empty tracesets.
    const pollIntervalMs = 500; // Interval to re-check.

    const backfillMissingParams = (params: any, isParamSet: boolean = false) => {
      if (this.defaults?.include_params) {
        this.defaults.include_params.forEach((key) => {
          if (params[key] === undefined) {
            params[key] = isParamSet ? [MISSING_VALUE_SENTINEL] : MISSING_VALUE_SENTINEL;
          }
        });
      }
      return params;
    };

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

    if (this.state.manual_plot_mode) {
      addParamSet(paramSets, backfillMissingParams(paramSet, true));
    } else {
      let allTracesets: string[][];
      try {
        allTracesets = await Promise.race([tracesetsReadyPromise, timeoutPromise]);
      } catch (error: unknown) {
        const e = error as { message?: string };
        errorMessage(e.message || 'An unknown error occurred while getting tracesets.');
        return;
      }

      allTracesets.forEach((traceset) => {
        traceset.forEach((trace) => {
          const params = fromKey(trace);
          addParamsToParamSet(paramSets, backfillMissingParams(params));
        });
      });
    }

    await this.testPicker!.populateFieldDataFromParamSet(paramSets, paramSet);
    this.testPicker!.setReadOnly(false);
    this.currentPageExploreElements[0]?.useBrowserURL(false);
    this.testPicker!.scrollIntoView();
  }

  private removeExploreEvent = (e: Event) => {
    this.removeExplore((e as CustomEvent).detail as ExploreSimpleSk);
  };

  private removeExplore(elem: ExploreSimpleSk | null = null): void {
    const indexToRemove = this.exploreElements.findIndex((e) => e === elem);

    if (indexToRemove > -1) {
      this.exploreElements.splice(indexToRemove, 1);
      this.allGraphConfigs.splice(indexToRemove, 1);
      this.allFrameRequests.splice(indexToRemove, 1);
      this.allFrameResponses.splice(indexToRemove, 1);
      // Re-index the remaining graphs. This ensures that the graph_index property
      // in each element's state correctly reflects its position in the exploreElements array,
      // which is important for syncing actions between graphs.
      this._reindexGraphs();

      // Adjust pagination: if there are no graphs left, reset page offset to 0.
      if (this.exploreElements.length === 0) {
        this.state.pageOffset = 0;
        this.state.totalGraphs = 0;
        this.testPicker!.autoAddTrace = false;
        this.resetGraphs();
        this.emptyCurrentPage();
      } else if (this.state.pageSize > 0) {
        // If graphs remain and pageSize is valid, calculate the maximum valid page offset.
        // This prevents being on a page that no longer exists
        // (e.g., if the last item on the last page was removed).
        // Note: renderCurrentPage will recalculate state.totalGraphs.
        const effectiveTotalGraphs = this.state.manual_plot_mode
          ? this.exploreElements.length
          : this.exploreElements.length - 1;
        const numPages = Math.ceil(Math.max(0, effectiveTotalGraphs) / this.state.pageSize);
        const maxValidPageOffset = Math.max(0, (numPages - 1) * this.state.pageSize);
        this.state.pageOffset = Math.min(this.state.pageOffset, maxValidPageOffset);
        this.renderCurrentPage();
      }
      this.updateShortcutMultiview();
    } else {
      if (this.stateHasChanged) this.stateHasChanged();
      this.renderCurrentPage();
    }
  }

  private resetGraphs() {
    this.emptyCurrentPage();
    this.currentPageExploreElements = [];
    this.allGraphConfigs = [];
    this.exploreElements = [];
    this.allFrameRequests = [];
    this.allFrameResponses = [];
  }

  private clearGraphs() {
    this.allGraphConfigs = [];
    this.exploreElements = [];
  }

  private emptyCurrentPage(): void {
    this.graphDiv!.replaceChildren();
    this.currentPageExploreElements = [];
  }

  private async renderCurrentPage(doNotQueryData: boolean = true): Promise<void> {
    // Logic: In Standard Mode (not manual), if we have multiple graphs,
    // the first one (Index 0) is the "Summary" and is hidden from pagination.
    const isSummaryView = !this.state.manual_plot_mode && this.allGraphConfigs.length > 1;

    if (isSummaryView) {
      this.state.totalGraphs = this.allGraphConfigs.length - 1;
    } else {
      // In manual mode, or if there is only 1 graph, we count everything.
      this.state.totalGraphs = this.allGraphConfigs.length || 1;
    }

    this.emptyCurrentPage();
    const indexShift = isSummaryView ? 1 : 0;
    const startIndex = this.state.pageOffset + indexShift;

    let endIndex = startIndex + this.state.pageSize - 1;
    if (this.allGraphConfigs.length <= endIndex) {
      endIndex = this.allGraphConfigs.length - 1;
    }

    const fragment = document.createDocumentFragment();

    // Always render the main graph (index 0) if it exists, to ensure it's connected and can load
    // data.
    // In summary view (startIndex > 0), we hide it.
    if (this.allGraphConfigs.length > 0 && startIndex > 0) {
      let explore = this.exploreElements[0];
      if (!explore) {
        explore = this.createExploreSimpleSk();
        this.exploreElements[0] = explore;
      }
      explore.style.display = 'none';

      const graphConfig = this.allGraphConfigs[0];
      const hasResponse =
        !!this.allFrameResponses[0] ||
        (explore && Object.keys(explore.getTraceset() || {}).length > 0);
      // In summary view, manual_plot_mode is false, so we don't need the complex check.
      const shouldQueryDataForThisGraph = !doNotQueryData && !hasResponse;

      this.addStateToExplore(explore, graphConfig, !shouldQueryDataForThisGraph, 0);
      fragment.appendChild(explore);
    }

    for (let i = startIndex; i <= endIndex; i++) {
      const graphConfig = this.allGraphConfigs[i];
      if (!graphConfig) {
        continue;
      }
      // Ensure exploreElements has an entry for this index.
      let explore = this.exploreElements[i];
      if (!explore) {
        explore = this.createExploreSimpleSk();
        this.exploreElements[i] = explore;
      }
      explore.style.display = '';

      // If we already have a response for this graph, we don't need to query data.
      const hasResponse =
        !!this.allFrameResponses[i] ||
        (explore && Object.keys(explore.getTraceset() || {}).length > 0);

      // Logic:
      // 1. Default: Query if we are allowed to (doNotQueryData is false) AND we don't have a
      //    response yet.
      // 2. Manual Mode Override: If we are adding a graph (doNotQueryData is true), we DO want to
      //    query
      //    for the main graph (i === 0) because it's the new one and has no response yet.
      const shouldQueryDataForThisGraph =
        (!doNotQueryData && !hasResponse) ||
        (this.state.manual_plot_mode && doNotQueryData && i === 0 && !hasResponse);

      this.currentPageExploreElements.push(explore);
      this.addStateToExplore(explore, graphConfig, !shouldQueryDataForThisGraph, i);
      fragment.appendChild(explore);
    }

    this.graphDiv!.appendChild(fragment);
    this.updateChartHeights();
    this._render();

    if (this.state.manual_plot_mode && doNotQueryData) {
      return;
    }

    // Now that the elements are in the DOM, we can update them with data.
    // This avoids race conditions where the element is not yet connected/rendered
    // and thus the DataFrameRepository is not available.
    // We use requestAnimationFrame to ensure the lit-html render cycle has completed
    // and refs are populated.
    return await new Promise((resolve) => {
      window.requestAnimationFrame(async () => {
        try {
          const updatePromises: Promise<void>[] = [];

          // Update hidden accumulator if present (startIndex > 0)
          if (this.allGraphConfigs.length > 0 && startIndex > 0) {
            const i = 0;
            const explore = this.exploreElements[0];
            if (explore) {
              const isDefaultResponse = !this.allFrameResponses[i];
              const frameResponse = this.allFrameResponses[i] || this.createFrameResponse();
              const frameRequest = this.allFrameRequests[i] || this.createFrameRequest();

              if ((explore as any).updateComplete) {
                await (explore as any).updateComplete;
              }
              updatePromises.push(
                explore.UpdateWithFrameResponse(
                  frameResponse,
                  frameRequest,
                  false,
                  this.mainGraphSelectedRange,
                  false,
                  !isDefaultResponse
                )
              );
            }
          }

          for (let i = startIndex; i <= endIndex; i++) {
            // Calculate the index in currentPageExploreElements (0-based for the current page)
            const pageIndex = i - startIndex;
            const explore = this.currentPageExploreElements[pageIndex];
            if (!explore) {
              continue;
            }
            const isDefaultResponse = !this.allFrameResponses[i];
            // If we don't have a cached response and the graph is already fetching
            // its own data, skip this update to avoid triggering "No data found" errors.
            if (isDefaultResponse && !explore.state.doNotQueryData) {
              continue;
            }
            const frameResponse = this.allFrameResponses[i] || this.createFrameResponse();
            const frameRequest = this.allFrameRequests[i] || this.createFrameRequest();

            // Wait for the element to be ready.
            if ((explore as any).updateComplete) {
              await (explore as any).updateComplete;
            }

            updatePromises.push(
              explore.UpdateWithFrameResponse(
                frameResponse,
                frameRequest,
                false,
                this.mainGraphSelectedRange,
                false,
                !isDefaultResponse
              )
            );
          }
          await Promise.all(updatePromises);
        } catch (error) {
          console.error('Error updating charts:', error);
        } finally {
          resolve();
        }
      });
    });
  }

  private updateChartHeights(): void {
    window.requestAnimationFrame(() => {
      if (!this.isConnected || !this.graphDiv) {
        return;
      }
      const graphs = this.graphDiv.querySelectorAll('explore-simple-sk');
      const visibleGraphCount = Array.from(graphs).filter(
        (g) => (g as HTMLElement).style.display !== 'none'
      ).length;
      const height = visibleGraphCount === 1 ? '500px' : '250px';

      graphs.forEach((graph) => {
        (graph as ExploreSimpleSk).updateChartHeight(height);
      });
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
          selectedRange,
          true,
          false
        );
        graph.dataLoading = false;
      }
    });
  }

  private async syncExtendRange(e: CustomEvent<PlotSelectionEventDetails>): Promise<void> {
    if (this._dataLoading) {
      return;
    }
    const graphs = this.exploreElements;
    const offset = e.detail.offsetInSeconds;
    const range = e.detail.value;

    if (!offset) {
      return;
    }

    const promises = graphs.map(async (graph) => {
      try {
        await graph.extendRange(range, offset);
      } catch (error) {
        errorMessage(`Error in graph.extendRange: ${error}`);
      }
    });
    await Promise.allSettled(promises);

    if (this.stateHasChanged) {
      this.stateHasChanged();
    }
  }

  private async syncChartSelection(e: CustomEvent<PlotSelectionEventDetails>): Promise<void> {
    if (this._dataLoading) {
      return;
    }
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
    } else if (!this.state.manual_plot_mode) {
      // For single graph (not in manual mode), we need to ensure local state respects the selection
      // so that it doesn't revert the URL when stateHasChanged is called later.
      if (e.detail.value.begin !== this.state.begin) {
        this.state.begin = e.detail.value.begin;
      }
      if (e.detail.value.end !== this.state.end) {
        this.state.end = e.detail.value.end;
      }
      if (this.stateHasChanged) {
        this.stateHasChanged();
      }
    }
  }

  private syncXAxisLabel(e: CustomEvent): void {
    this.exploreElements.forEach((graph) => {
      // Skip graph that sent the event.
      if (graph.state.graph_index !== e.detail.index) {
        graph.updateXAxis(e.detail.domain);
      }
    });
  }

  private addStateToExplore(
    explore: ExploreSimpleSk,
    graphConfig: GraphConfig,
    doNotQueryData: boolean,
    index: number
  ) {
    const newState: ExploreState = {
      formulas: graphConfig.formulas || [],
      queries: graphConfig.queries || [],
      keys: graphConfig.keys || '',
      begin: this.state.begin,
      end: this.state.end,
      showZero: this.state.showZero,
      numCommits: this.state.numCommits,
      summary: this.state.summary,
      xbaroffset: explore.state.xbaroffset,
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
      enable_favorites: this.canAddFav(),
      hide_paramset: true,
      graph_index: index,
      doNotQueryData: doNotQueryData,
      evenXAxisSpacing:
        this.state.evenXAxisSpacing !== 'use_cache'
          ? this.state.evenXAxisSpacing === 'true'
          : localStorage.getItem(CACHE_KEY_EVEN_X_AXIS_SPACING) === 'true',
      dots: this.state.dots,
      autoRefresh: this.state.autoRefresh,
      show_google_plot: this.state.show_google_plot,
    };
    explore.state = newState;
  }

  private _reindexGraphs(): void {
    this.exploreElements.forEach((elem, index) => {
      elem.state.graph_index = index;
    });
  }

  private addEmptyGraph(unshift?: boolean): ExploreSimpleSk {
    const explore = this.createExploreSimpleSk();
    const graphConfig = new GraphConfig();
    if (unshift) {
      this.exploreElements.unshift(explore);
      this.allGraphConfigs.unshift(graphConfig);
    } else {
      this.exploreElements.push(explore);
      this.allGraphConfigs.push(graphConfig);
    }
    this._reindexGraphs();
    return explore;
  }

  private createExploreSimpleSk(): ExploreSimpleSk {
    const explore: ExploreSimpleSk = new ExploreSimpleSk(this.useTestPicker);
    explore.defaults = this.defaults;
    explore.openQueryByDefault = false;
    explore.navOpen = false;
    // If multi chart has user email, set it for the explore.
    if (this.userEmail) {
      explore.user = this.userEmail;
    }
    explore.addEventListener('state_changed', () => {
      const elemState = explore.state;
      if (!this.allGraphConfigs[elemState.graph_index]) {
        return;
      }
      let stateChanged = false;
      if (this.allGraphConfigs[elemState.graph_index].formulas !== elemState.formulas) {
        this.allGraphConfigs[elemState.graph_index].formulas = elemState.formulas || [];
        stateChanged = true;
      }

      if (this.allGraphConfigs[elemState.graph_index].queries[0] !== elemState.queries[0]) {
        this.allGraphConfigs[elemState.graph_index].queries = elemState.queries || [];
        stateChanged = true;
      }

      if (this.allGraphConfigs[elemState.graph_index].keys !== elemState.keys) {
        this.allGraphConfigs[elemState.graph_index].keys = elemState.keys || '';
        stateChanged = true;
      }

      if (stateChanged) {
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
      // CHANGE: Only sync graph state back to picker if NOT manual_plot_mode
      if (!this.state.manual_plot_mode) {
        const triggeredByPicker =
          this.loadTrigger === 'plot_button_clicked' || this.loadTrigger === 'picker_add_trace';
        if (!this.testPicker.isLoaded() && this.exploreElements.length > 0 && !triggeredByPicker) {
          this.populateTestPicker(this.exploreElements[0].getParamSet());
        }
      }

      if (this.exploreElements.length === 0) {
        this._dataLoading = false;
      }
      if (this.exploreElements.some((e) => e.dataLoading)) {
        this._dataLoading = true;
        this.testPicker.setReadOnly(true);
      } else {
        // Only record telemetry if a load was explicitly triggered (direct link or plot button).
        // Resetting initialLoadStartTime and loadTrigger prevents duplicate telemetry reports.
        if (this.initialLoadStartTime > 0 && this.loadTrigger) {
          telemetry.recordSummary(
            SummaryMetric.MultiGraphDataLoadTime,
            (performance.now() - this.initialLoadStartTime) / 1000,
            { url: window.location.href }
          );
          telemetry.increaseCounter(CountMetric.MultiGraphVisit, {
            trigger: this.loadTrigger,
            total_graphs: this.state.totalGraphs.toString(),
          });
          this.initialLoadStartTime = 0;
          this.loadTrigger = '';
        }
        this._dataLoading = false;
        this.testPicker.setReadOnly(false);
      }
    }
    if (this.stateHasChanged) {
      this.stateHasChanged();
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
    if (this.exploreElements.length === 0) {
      return null;
    }
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
  private async getConfigsFromShortcut(shortcut: string): Promise<GraphConfig[]> {
    return (await DataService.getInstance()
      .getShortcut(shortcut)
      .catch((msg) => {
        errorMessage(msg);
        return [];
      })) as unknown as GraphConfig[];
  }

  /**
   * Creates a shortcut ID for the current Graph Configs and updates the state.
   *
   */
  private updateShortcutMultiview() {
    updateShortcut(this.allGraphConfigs)
      .then((shortcut: string) => {
        if (shortcut === '') {
          this.state.shortcut = '';
          this.stateHasChanged!();
          return;
        }
        this.state.shortcut = shortcut;
        this.stateHasChanged!();
      })
      .catch((err: unknown) => {
        if (err instanceof DataServiceError) {
          errorMessage(err.message);
        } else {
          errorMessage(err as string);
        }
      });
  }

  private pageChanged(e: CustomEvent<PaginationSkPageChangedEventDetail>) {
    this._dataLoading = true;
    this.testPicker?.setReadOnly(true);
    this.state.pageOffset = Math.max(
      0,
      this.state.pageOffset + e.detail.delta * this.state.pageSize
    );
    this.stateHasChanged!();
    this.renderCurrentPage();
  }

  private pageSizeChanged(e: MouseEvent) {
    this._dataLoading = true;
    this.testPicker?.setReadOnly(true);
    this.state.pageSize = +(e.target! as HTMLInputElement).value;
    this.stateHasChanged!();
    this.renderCurrentPage();
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
