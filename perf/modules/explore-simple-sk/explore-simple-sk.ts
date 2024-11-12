/**
 * @module module/explore-simple-sk
 * @description <h2><code>explore-simple-sk</code></h2>
 *
 * Element for exploring data.
 */
import { html } from 'lit/html.js';
import { when } from 'lit/directives/when.js';
import { ref, createRef, Ref } from 'lit/directives/ref.js';
import { MdDialog } from '@material/web/dialog/dialog.js';
import { MdSwitch } from '@material/web/switch/switch.js';
import { define } from '../../../elements-sk/modules/define';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';

import { deepCopy } from '../../../infra-sk/modules/object';
import { toParamSet, fromParamSet } from '../../../infra-sk/modules/query';
import { TabsSk } from '../../../elements-sk/modules/tabs-sk/tabs-sk';
import { ToastSk } from '../../../elements-sk/modules/toast-sk/toast-sk';
import { ParamSet as CommonSkParamSet } from '../../../infra-sk/modules/query';
import { SpinnerSk } from '../../../elements-sk/modules/spinner-sk/spinner-sk';
import { errorMessage } from '../errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { escapeAndLinkifyToString } from '../../../infra-sk/modules/linkify';

import '@material/web/button/outlined-button.js';
import '@material/web/icon/icon.js';
import '@material/web/iconbutton/outlined-icon-button.js';
import '@material/web/switch/switch.js';
import '@material/web/dialog/dialog.js';

import '../../../elements-sk/modules/checkbox-sk';
import '../../../elements-sk/modules/collapse-sk';
import '../../../elements-sk/modules/icons/expand-less-icon-sk';
import '../../../elements-sk/modules/icons/expand-more-icon-sk';
import '../../../elements-sk/modules/icons/help-icon-sk';
import '../../../elements-sk/modules/icons/close-icon-sk';
import '../../../elements-sk/modules/spinner-sk';
import '../../../elements-sk/modules/tabs-panel-sk';
import '../../../elements-sk/modules/tabs-sk';
import '../../../elements-sk/modules/toast-sk';

import '../../../infra-sk/modules/query-sk';
import '../../../infra-sk/modules/paramset-sk';

import '../anomaly-sk';
import '../commit-detail-panel-sk';
import '../commit-range-sk';
import '../domain-picker-sk';
import '../json-source-sk';
import '../ingest-file-links-sk';
import '../picker-field-sk';
import '../pivot-query-sk';
import '../pivot-table-sk';
import '../plot-google-chart-sk';
import '../plot-simple-sk';
import '../plot-summary-sk';
import '../point-links-sk';
import '../split-chart-menu-sk';
import '../query-count-sk';
import '../graph-title-sk';
import '../new-bug-dialog-sk';
import '../existing-bug-dialog-sk';
import '../window/window';

import {
  CommitNumber,
  CreateBisectRequest,
  CreateBisectResponse,
  Anomaly,
  DataFrame,
  RequestType,
  ParamSet,
  FrameRequest,
  FrameResponse,
  ShiftRequest,
  ShiftResponse,
  progress,
  pivot,
  FrameResponseDisplayMode,
  ColumnHeader,
  CIDHandlerResponse,
  QueryConfig,
  TraceSet,
  Commit,
  Trace,
  ReadOnlyParamSet,
} from '../json';
import {
  AnomalyData,
  PlotSimpleSk,
  PlotSimpleSkTraceEventDetails,
} from '../plot-simple-sk/plot-simple-sk';
import { CommitDetailPanelSk } from '../commit-detail-panel-sk/commit-detail-panel-sk';
import { JSONSourceSk } from '../json-source-sk/json-source-sk';
import {
  ParamSetSk,
  ParamSetSkCheckboxClickEventDetail,
  ParamSetSkClickEventDetail,
  ParamSetSkPlusClickEventDetail,
  ParamSetSkRemoveClickEventDetail,
} from '../../../infra-sk/modules/paramset-sk/paramset-sk';
import { AnomalySk, getAnomalyDataMap } from '../anomaly-sk/anomaly-sk';
import {
  QuerySk,
  QuerySkQueryChangeEventDetail,
} from '../../../infra-sk/modules/query-sk/query-sk';
import { QueryCountSk } from '../query-count-sk/query-count-sk';
import { DomainPickerSk } from '../domain-picker-sk/domain-picker-sk';
import {
  messageByName,
  messagesToErrorString,
  messagesToPreString,
  startRequest,
} from '../progress/progress';
import { IngestFileLinksSk } from '../ingest-file-links-sk/ingest-file-links-sk';
import { validatePivotRequest } from '../pivotutil';
import { PivotQueryChangedEventDetail, PivotQuerySk } from '../pivot-query-sk/pivot-query-sk';
import { PivotTableSk, PivotTableSkChangeEventDetail } from '../pivot-table-sk/pivot-table-sk';
import { fromKey, paramsToParamSet, validKey } from '../paramtools';
import { dataFrameToCSV } from '../csv';
import { CommitRangeSk } from '../commit-range-sk/commit-range-sk';
import { MISSING_DATA_SENTINEL } from '../const/const';
import { LoggedIn } from '../../../infra-sk/modules/alogin-sk/alogin-sk';
import { Status as LoginStatus } from '../../../infra-sk/modules/json';
import { findAnomalyInRange, findSubDataframe, range } from '../dataframe';
import { TraceFormatter, GetTraceFormatter } from '../trace-details-formatter/traceformatter';
import { fixTicksLength, tick, ticks } from '../plot-simple-sk/ticks';
import {
  PlotSummarySk,
  PlotSummarySkSelectionEventDetails,
} from '../plot-summary-sk/plot-summary-sk';
import { PickerFieldSk } from '../picker-field-sk/picker-field-sk';
import '../chart-tooltip-sk/chart-tooltip-sk';
import '../dataframe/dataframe_context';
import { ChartTooltipSk } from '../chart-tooltip-sk/chart-tooltip-sk';
import { NudgeEntry } from '../triage-menu-sk/triage-menu-sk';
import { $$ } from '../../../infra-sk/modules/dom';
import { PointLinksSk } from '../point-links-sk/point-links-sk';
import { GraphTitleSk } from '../graph-title-sk/graph-title-sk';
import { NewBugDialogSk } from '../new-bug-dialog-sk/new-bug-dialog-sk';
import {
  PlotGoogleChartSk,
  PlotSelectionEventDetails,
  PlotShowTooltipEventDetails,
} from '../plot-google-chart-sk/plot-google-chart-sk';
import { DataFrameRepository } from '../dataframe/dataframe_context';
import { ExistingBugDialogSk } from '../existing-bug-dialog-sk/existing-bug-dialog-sk';
import { generateSubDataframe } from '../dataframe/index';
import { SplitChartSelectionEventDetails } from '../split-chart-menu-sk/split-chart-menu-sk';

/** The type of trace we are adding to a plot. */
type addPlotType = 'query' | 'formula' | 'pivot';

// The trace id of the zero line, a trace of all zeros.
const ZERO_NAME = 'special_zero';

// A list of all special trace names.
const SPECIAL_TRACE_NAMES = [ZERO_NAME];

// How often to refresh if the auto-refresh checkmark is checked.
const REFRESH_TIMEOUT = 30 * 1000; // milliseconds

// The default query range in seconds.
export const DEFAULT_RANGE_S = 24 * 60 * 60; // 2 days in seconds.

// The index of the params tab.
const PARAMS_TAB_INDEX = 0;

// The index of the commit detail info tab.
const COMMIT_TAB_INDEX = 1;

// The percentage of the current zoom window to pan or zoom on a keypress.
const ZOOM_JUMP_PERCENT = CommitNumber(0.1);

// When we are zooming around and bump into the edges of the graph, how much
// should we widen the range of commits, as a percentage of the currently
// displayed commit.
const RANGE_CHANGE_ON_ZOOM_PERCENT = 0.5;

// The minimum length [right - left] of a zoom range.
const MIN_ZOOM_RANGE = 0.1;

// The max number of points a user can nudge an anomaly by.
const NUDGE_RANGE = 2;

const STATISTIC_VALUES = ['avg', 'count', 'max', 'min', 'std', 'sum'];

const monthInSec = 30 * 24 * 60 * 60;

// max number of charts to show on a page
const chartsPerPage = 11;

type RequestFrameCallback = (frameResponse: FrameResponse) => void;

export interface ZoomWithDelta {
  zoom: CommitRange;
  delta: CommitNumber;
}

// Even though pivot.Request sent to the server can be null, we don't want to
// put use a null in state, as that won't let stateReflector figure out the
// right types inside pivot.Request, so we default to an invalid value here.
const defaultPivotRequest = (): pivot.Request => ({
  group_by: [],
  operation: 'avg',
  summary: [],
});

export type CommitRange = [CommitNumber, CommitNumber];

// Stores the trace name and commit number of a single point on a trace.
export interface PointSelected {
  commit: CommitNumber;
  name: string;
}

export enum LabelMode {
  Date = 0,
  CommitPosition = 1,
}

/** Returns true if the PointSelected is valid. */
export const isValidSelection = (p: PointSelected): boolean => p.name !== '';

/** Converts a PointSelected into a CustomEvent<PlotSimpleSkTraceEventDetails>,
 * so that it can be passed into traceSelected().
 *
 * Note that we need the _dataframe.header to convert the commit back into an
 * offset. Also note that might fail, in which case the 'x' value will be set to
 * -1.
 */
export const selectionToEvent = (
  p: PointSelected,
  header: (ColumnHeader | null)[] | null
): CustomEvent<PlotSimpleSkTraceEventDetails> => {
  let x = -1;
  if (header !== null) {
    // Find the index of the ColumnHeader that matches the commit.
    x = header.findIndex((h: ColumnHeader | null) => {
      if (h === null) {
        return false;
      }
      return h.offset === p.commit;
    });
  }
  return new CustomEvent<PlotSimpleSkTraceEventDetails>('', {
    detail: {
      x: x,
      y: 0,
      name: p.name,
    },
  });
};

/** Returns a default value for PointSelected. */
export const defaultPointSelected = (): PointSelected => ({
  commit: CommitNumber(0),
  name: '',
});

export class GraphConfig {
  formulas: string[] = []; // Formulas

  queries: string[] = []; // Queries

  keys: string = ''; // Keys
}

/**
 * Creates a shortcut ID for the given Graph Configs.
 *
 */
export const updateShortcut = async (graphConfigs: GraphConfig[]): Promise<string> => {
  if (graphConfigs.length === 0) {
    return '';
  }

  const body = {
    graphs: graphConfigs,
  };

  return fetch('/_/shortcut/update', {
    method: 'POST',
    body: JSON.stringify(body),
    headers: {
      'Content-Type': 'application/json',
    },
  })
    .then(jsonOrThrow)
    .then((json) => json.id)
    .catch(errorMessage);
};

// State is reflected to the URL via stateReflector.
export class State {
  begin: number = Math.floor(Date.now() / 1000 - DEFAULT_RANGE_S);

  end: number = Math.floor(Date.now() / 1000);

  formulas: string[] = [];

  queries: string[] = [];

  keys: string = ''; // The id of the shortcut to a list of trace keys.

  xbaroffset: number = -1; // The offset of the commit in the repo.

  showZero: boolean = true;

  dots: boolean = true; // Whether to show dots when plotting traces.

  autoRefresh: boolean = false;

  numCommits: number = 250;

  requestType: RequestType = 1; // TODO(jcgregorio) Use constants in domain-picker-sk.

  pivotRequest: pivot.Request = defaultPivotRequest();

  sort: string = ''; // Pivot table sort order.

  summary: boolean = false; // Whether to show the zoom/summary area.

  selected: PointSelected = defaultPointSelected(); // The point on a trace that was clicked on.

  labelMode: LabelMode = LabelMode.Date;

  _incremental: boolean = false; // Enables a data fetching optimization.

  disable_filter_parent_traces: boolean = false;

  plotSummary: boolean = false;

  disableMaterial?: boolean = false;

  highlight_anomalies: string[] = [];

  enable_chart_tooltip: boolean = false;

  show_remove_all: boolean = true;

  use_titles: boolean = false;

  useTestPicker: boolean = false;

  use_test_picker_query: boolean = false;

  show_google_plot = false;
}

// TODO(jcgregorio) Move to a 'key' module.
// Returns true if paramName=paramValue appears in the given structured key.
function _matches(key: string, paramName: string, paramValue: string): boolean {
  return key.indexOf(`,${paramName}=${paramValue},`) >= 0;
}

interface RangeChange {
  /**
   * If true then do a range change with the provided offsets, otherwise just
   * do a zoom.
   */
  rangeChange: boolean;

  newOffsets?: [CommitNumber, CommitNumber];
}

// clamp ensures a number is not negative.
function clampToNonNegative(x: number): number {
  if (x < 0) {
    return 0;
  }
  return x;
}

/**
 * Determines if a range change is needed based on a zoom request, and if so
 * calculates what the new range change should be.
 *
 * @param zoom is the requested zoom.
 * @param clampedZoom is the requested zoom clamped to the current dataframe.
 * @param offsets are the commit offset of the first and last value in the
 * dataframe.
 */
export function calculateRangeChange(
  zoom: CommitRange,
  clampedZoom: CommitRange,
  offsets: [CommitNumber, CommitNumber]
): RangeChange {
  // How much we will change the offset if we zoom beyond an edge.
  const offsetDelta = Math.floor(
    (offsets[1] - offsets[0]) * RANGE_CHANGE_ON_ZOOM_PERCENT
  ) as CommitNumber;
  const exceedsLeftEdge = zoom[0] !== clampedZoom[0];
  const exceedsRightEdge = zoom[1] !== clampedZoom[1];
  if (exceedsLeftEdge && exceedsRightEdge) {
    // shift both
    return {
      rangeChange: true,
      newOffsets: [
        clampToNonNegative(offsets[0] - offsetDelta) as CommitNumber,
        (offsets[1] + offsetDelta) as CommitNumber,
      ],
    };
  }
  if (exceedsLeftEdge) {
    // shift left
    return {
      rangeChange: true,
      newOffsets: [clampToNonNegative(offsets[0] - offsetDelta) as CommitNumber, offsets[1]],
    };
  }
  if (exceedsRightEdge) {
    // shift right
    return {
      rangeChange: true,
      newOffsets: [offsets[0], (offsets[1] + offsetDelta) as CommitNumber],
    };
  }
  return {
    rangeChange: false,
  };
}

export class ExploreSimpleSk extends ElementSk {
  private _dataframe: DataFrame = {
    traceset: TraceSet({}),
    header: [],
    paramset: ReadOnlyParamSet({}),
    skip: 0,
  };

  // The state that does into the URL.
  private _state: State = new State();

  // Set of customization params that have been explicitly specified
  // by the user.
  private _userSpecifiedCustomizationParams: Set<string> = new Set();

  // Controls the mode of the display. See FrameResponseDisplayMode.
  private displayMode: FrameResponseDisplayMode = 'display_query_only';

  // Are we waiting on data from the server.
  private _spinning: boolean = false;

  private _dialogOn: boolean = false;

  // The id of the current frame request. Will be the empty string if there
  // is no pending request.
  private _requestId = '';

  // The id of the interval timer if we are refreshing.
  private _refreshId = -1;

  // All the data converted into a CVS blob to download.
  private _csvBlobURL: string = '';

  private fromParamsKey: string = '';

  private testPath: string = '';

  private startCommit: string = '';

  private endCommit: string = '';

  private story: string = '';

  private bugId: string = '';

  private jobUrl: string = '';

  private jobId: string = '';

  private user: string = '';

  private _defaults: QueryConfig | null = null;

  private _initialized: boolean = false;

  private anomalyTable: AnomalySk | null = null;

  private commits: CommitDetailPanelSk | null = null;

  private commitsTab: HTMLButtonElement | null = null;

  private detailTab: TabsSk | null = null;

  private formula: HTMLTextAreaElement | null = null;

  private jsonsource: JSONSourceSk | null = null;

  private ingestFileLinks: IngestFileLinksSk | null = null;

  private pointLinks: PointLinksSk | null = null;

  private logEntry: HTMLPreElement | null = null;

  private paramset: ParamSetSk | null = null;

  private percent: HTMLSpanElement | null = null;

  private plotSimple = createRef<PlotSimpleSk>();

  private googleChartPlot = createRef<PlotGoogleChartSk>();

  private plotSummary = createRef<PlotSummarySk>();

  private query: QuerySk | null = null;

  private fromParamsQuery: QuerySk | null = null;

  private fromParamsQueryCount: QueryCountSk | null = null;

  private queryCount: QueryCountSk | null = null;

  private range: DomainPickerSk | null = null;

  private simpleParamset: ParamSetSk | null = null;

  private spinner: SpinnerSk | null = null;

  private summary: ParamSetSk | null = null;

  private commitTime: HTMLSpanElement | null = null;

  private csvDownload: HTMLAnchorElement | null = null;

  private pivotControl: PivotQuerySk | null = null;

  private pivotTable: PivotTableSk | null = null;

  private pivotDisplayButton: HTMLButtonElement | null = null;

  private queryDialog: HTMLDialogElement | null = null;

  private bisectDialog: HTMLDialogElement | null = null;

  private fromParamsQueryDialog: HTMLDialogElement | null = null;

  private helpDialog: HTMLDialogElement | null = null;

  private commitRangeSk: CommitRangeSk | null = null;

  // TODO(b/372694234): consolidate the pinpoint and triage toasts.
  private pinpointJobToast: ToastSk | null = null;

  private triageResultToast: ToastSk | null = null;

  private closePinpointToastButton: HTMLButtonElement | null = null;

  private closeTriageToastButton: HTMLButtonElement | null = null;

  private bisectButton: HTMLButtonElement | null = null;

  private collapseButton: HTMLButtonElement | null = null;

  private traceDetails: HTMLSpanElement | null = null;

  private traceDetailsCopy: HTMLDivElement | null = null;

  private traceFormatter: TraceFormatter | null = null;

  private originalTraceSet: TraceSet = TraceSet({});

  private scrollable: boolean = false;

  private traceKeyForSummary: string = '';

  chartTooltip: ChartTooltipSk | null = null;

  useTestPicker: boolean = false;

  private summaryOptionsField: Ref<PickerFieldSk> = createRef();

  private traceNameIdMap = new Map();

  private pointToCommitDetailMap = new Map();

  // tooltipSelected tracks whether someone has turned on the tooltip
  // by selecting a data point. A new tooltip will not be created on
  // mouseover unless the current selected tooltip is closed.
  // true - the tooltip is selected
  // false - the tooltip is not selected but could be on via mouse hover
  private tooltipSelected = false;

  private graphTitle: GraphTitleSk | null = null;

  private showRemoveAll = true;

  private tracesRendered = false;

  // material UI
  private settingsDialog: MdDialog | null = null;

  private dfRepo = createRef<DataFrameRepository>();

  constructor(scrollable: boolean, useTestPicker?: boolean) {
    super(ExploreSimpleSk.template);
    this.scrollable = scrollable;
    this.traceFormatter = GetTraceFormatter();
    this.useTestPicker = useTestPicker ?? false;
  }

  private static template = (ele: ExploreSimpleSk) => html`
  <dataframe-repository-sk ${ref(ele.dfRepo)}>
  <div id=explore class=${ele.displayMode}>
    <div id=buttons>
      <button
        id=open_query_dialog
        ?hidden=${ele.useTestPicker}
        @click=${ele.openQuery}>
        Query
      </button>
      <div id=traceButtons class="hide_on_query_only hide_on_pivot_table hide_on_spinner">
        <split-chart-menu-sk
          @split-chart-selection=${ele.splitByAttribute}>
        </split-chart-menu-sk>
        <md-outlined-button
          ?disabled=${!(
            ele.plotSimple.value &&
            ele.plotSimple.value!.highlight.length &&
            ele.useTestPicker
          )}
          @click=${ele.queryHighlighted}>
          Query Highlighted
        </md-outlined-button>
        <md-outlined-button
          @click=${ele.removeHighlighted}
          ?disabled=${!(ele.plotSimple.value && ele.plotSimple.value!.highlight.length)}>
          Remove Highlighted
        </md-outlined-button>
        <md-outlined-button
          @click=${ele.highlightedOnly}
          ?disabled=${!(ele.plotSimple.value && ele.plotSimple.value!.highlight.length)}>
          Highlighted Only
        </md-outlined-button>

        <span
          title='Number of commits skipped between each point displayed.'
          ?hidden=${ele.isZero(ele._dataframe.skip)}
          id=skip>
            ${ele._dataframe.skip}
        </span>

        <md-outlined-button
          ?hidden=${!ele.showRemoveAll}
          @click=${() => ele.removeAll(false)}>
          Remove All
        </md-outlined-button>
        <div
          id=calcButtons
          class="hide_on_query_only">
          <md-outlined-button
            @click=${() => ele.applyFuncToTraces('norm')}>
            Normalize
          </md-outlined-button>
          <md-outlined-button
            @click=${() => ele.applyFuncToTraces('scale_by_avg')}>
            Scale By Avg
          </md-outlined-button>
          <md-outlined-button
            @click=${() => {
              ele.applyFuncToTraces('iqrr');
            }}>
            Remove outliers
          </md-outlined-button>
          <md-outlined-button
            @click=${ele.csv}>
            CSV
          </md-outlined-button>
          <a href='' target=_blank download='traces.csv' id=csv_download></a>
          <md-outlined-button
            ?hidden=${!window.perf.fetch_chrome_perf_anomalies}
            @click=${ele.openBisect}>
            Bisect
          </md-outlined-button>
          <div id="zoomPan" ?hidden=${ele.plotSummary}>
            <h3>Zoom/Pan:</h3>
            <div id="btnContainer">
              <button
                class=navigation
                @click=${ele.zoomInKey}
                title='Zoom In'>
                <span class=icon-sk>add</span>
              </button>
              <button
                class=navigation
                @click=${ele.zoomOutKey}
                title='Zoom Out'>
                <span class=icon-sk>remove</span>
              </button>
              <button
                class=navigation
                @click=${ele.zoomLeftKey}
                title='Pan Left'>
                <span class=icon-sk>arrow_back</span>
              </button>
              <button
                class=navigation
                @click=${ele.zoomRightKey}
                title='Pan Right'>
                <span class=icon-sk>arrow_forward</span>
              </button>
            </div>
          </div>
        </div>
      </div>
      <md-outlined-icon-button
        class="hide_on_query_only hide_on_pivot_table hide_on_spinner"
        @click=${ele.showSettingsDialog}>
          <md-icon id="settings-button">settings</md-icon>
      </md-outlined-icon-button>
      <md-dialog
        aria-label='Settings dialog'
        id='settings-dialog'>
        <form id="form" slot="content" method="dialog">
        </form>
        <div slot="actions">
          <ul style="list-style-type:none; padding-left: 0;">
            <li>
              <label>
                <md-switch
                  form="form"
                  id="commit-switch"
                  ?selected=${ele._state.labelMode === LabelMode.CommitPosition}
                  @change=${(e: InputEvent) => ele.switchXAxis(e.target as MdSwitch)}></md-switch>
                X-Axis as Commit Positions
              </label>
            <li>
            <li>
              <label>
                <md-switch
                  form="form"
                  id="dots-switch"
                  ?selected=${ele._state!.dots}
                  @change=${() => ele.toggleDotsHandler()}></md-switch>
                Dots on graph
              </label>
            </li>
            <li>
              <label>
                <md-switch
                  form="form"
                  id="zero-switch"
                  ?selected=${ele._state.showZero}
                  @change=${(e: InputEvent) =>
                    ele.zeroChangeHandler(e.target as MdSwitch)}></md-switch>
                Draw against "zero" line
              </label>
            </li>
            <li>
              <label>
                <md-switch
                  form="form"
                  id="auto-refresh-switch"
                  ?selected=${ele._state.autoRefresh}
                  @change=${(e: InputEvent) =>
                    ele.autoRefreshHandler(e.target as MdSwitch)}></md-switch>
                Auto refresh data
              </label>
            </li>
            <li ?hidden=${ele._state.plotSummary}>
              <label>
                <md-switch
                  form="form"
                  id="summary-bar-switch"
                  ?selected=${ele._state.summary}
                  @change=${(e: InputEvent) =>
                    ele.summaryChangeHandler(e.target as MdSwitch)}></md-switch>
                Summary Bar at Top of Plot
              </label>
            </li>
          </ul>
        </div>
      </md-dialog>
      <button
        class="hide_on_query_only hide_on_pivot_table hide_on_spinner"
        id="removeAll"
        @click=${() => ele.removeAll(false)}
        title='Remove all the traces.'>
        <close-icon-sk></close-icon-sk>
      </button>
    </div>

    <graph-title-sk id=graphTitle></graph-title-sk>

    <div id=spin-overlay @mouseleave=${ele.mouseLeave}>
    <div class="chart-container">
        <plot-google-chart-sk
          style="display:${ele._state.show_google_plot ? 'block' : 'none'}"
          ${ref(ele.googleChartPlot)}
          @google-chart-select=${ele.onChartSelect}
          @plot-data-mouseover=${ele.onChartOver}
          @plot-chart-mousedown=${ele.onChartMouseDown}
          @selection-changing=${ele.OnSelectionRange}
          @selection-changed=${ele.OnSelectionRange}>
          <md-icon slot="untriage">question_exchange</md-icon>
          <md-icon slot="regression">report</md-icon>
          <md-icon slot="improvement">check</md-icon>
        </plot-google-chart-sk>
        <plot-simple-sk
          style="display:${ele._state.show_google_plot ? 'none' : 'block'}"
          .summary=${ele._state.summary}
          ${ref(ele.plotSimple)}
          @trace_selected=${ele.traceSelected}
          @zoom=${ele.plotZoom}
          @trace_focused=${ele.plotTraceFocused}
          class="hide_on_pivot_table hide_on_query_only hide_on_spinner"
          .scrollable=${ele.scrollable}>
        </plot-simple-sk>
      <chart-tooltip-sk></chart-tooltip-sk>
      </div>
      <div id="trace-details-container">
        <div id="traceDetailsCopy" class="icon-sk copy-content">content_copy</div>
        <span id=traceDetails></span>
      </div>
      ${when(ele._state.plotSummary && ele.tracesRendered, () => ele.plotSummaryTemplate())}
      <div id=spin-container class="hide_on_query_only hide_on_pivot_table hide_on_pivot_plot hide_on_plot">
        <spinner-sk id=spinner active></spinner-sk>
        <pre id=percent></pre>
      </div>
  </div>

    <pivot-table-sk
      @change=${ele.pivotTableSortChange}
      disable_validation
      class="hide_on_plot hide_on_pivot_plot hide_on_query_only hide_on_spinner">
    </pivot-table-sk>

    <dialog id='query-dialog'>
      <h2>Query</h2>
      <div class=query-parts>
        <query-sk
          id=query
          @query-change=${ele.queryChangeHandler}
          @query-change-delayed=${ele.queryChangeDelayedHandler}
          > </query-sk>
          <div id=selections>
            <h3>Selections</h3>
            <button id="closeQueryIcon" @click=${ele.closeQueryDialog}>
              <close-icon-sk></close-icon-sk>
            </button>
            <paramset-sk id=summary removable_values @paramset-value-remove-click=${
              ele.paramsetRemoveClick
            }></paramset-sk>
            <div class=query-counts>
              Matches: <query-count-sk url='/_/count/' @paramset-changed=${ele.paramsetChanged}>
              </query-count-sk>
            </div>
          </div>
      </div>

      <details id=time-range-summary>
        <summary>Time Range</summary>
        <domain-picker-sk id=range>
        </domain-picker-sk>
      </details>

      <tabs-sk>
        <button>Plot</button>
        <button>Calculations</button>
        <button>Pivot</button>
      </tabs-sk>
      <tabs-panel-sk>
        <div>
          <button @click=${() => ele.add(true, 'query')} class=action>Plot</button>
          <button @click=${() => ele.add(false, 'query')}>Add to Plot</button>
        </div>
        <div>
          <div class=formulas>
            <label>
              Enter a formula:
              <textarea id=formula rows=3 cols=80></textarea>
            </label>
            <div>
              <button @click=${() => ele.add(true, 'formula')} class=action>Plot</button>
              <button @click=${() => ele.add(false, 'formula')}>Add to Plot</button>
              <a href=/help/ target=_blank>
                <help-icon-sk></help-icon-sk>
              </a>
            </div>
          </div>
        </div>
        <div>
          <pivot-query-sk
            @pivot-changed=${ele.pivotChanged}
            .pivotRequest=${ele._state.pivotRequest}
          >
          </pivot-query-sk>
          <div>
            <button
              id=pivot-display-button
              @click=${() => ele.add(true, 'pivot')}
              class=action
              .disabled=${validatePivotRequest(ele._state.pivotRequest) !== ''}
            >Display</button>
          </div>
        </div>
      </tabs-panel-sk>
      <div class=footer>
        <button @click=${ele.closeQueryDialog} id='close_query_dialog'>Close</button>
      </div>
    </dialog>

    <dialog id='bisect-dialog'>
      <h2>Bisect</h2>
      <button id="bisectCloseIcon" @click=${ele.closeBisectDialog}>
        <close-icon-sk></close-icon-sk>
      </button>
      <h3>Test Path</h3>
      <input id="testpath" type="text" value=${ele.testPath} readonly></input>
      <h3>Bug ID</h3>
      <input id="bug-id" type="text" value=${ele.bugId}></input>
      <h3>Start Commit</h3>
      <input id="start-commit" type="text" value=${ele.startCommit}></input>
      <h3>End Commit</h3>
      <input id="end-commit" type="text" value=${ele.endCommit}></input>
      <h3>Story</h3>
      <input id="story" type="text" value=${ele.story}></input>
      <h3>Patch to apply to the entire job(optional)</h3>
      <input id="patch" type="text"></input>
      <div class=footer>
        <button id="bisect-button" @click=${ele.postBisect}>Bisect</button>
        <button @click=${ele.closeBisectDialog}>Close</button>
      </div>
    </dialog>

    <!--
    This is the quick-add dialog that appears when you click the '+' sign on any of
    the Params rows displayed in the details tab (See #simple_paramset).
    -->
    <dialog id='from-params-query-dialog'>
      <h2>Query</h2>
      <div class=query-parts>
        <query-sk
          id=from-params-query
          values_only
          hide_invert
          hide_regex
          >
        </query-sk>
      </div>
      <div class=query-counts>
        Matches: <query-count-sk
          id=from-params-query-count
          url='/_/count/'
          @paramset-changed=${ele.fromParamsParamsetChanged}
        >
        </query-count-sk>
      </div>
      <div class=footer>
        <button class=action @click=${ele.fromParamsOKQueryDialog}>Plot</button>
        <button @click=${ele.fromParamsCloseQueryDialog}>Close</button>
      </div>
    </dialog>

    <dialog id=help>
      <h2>Perf Help</h2>
      <table>
        <tr><td colspan=2><h3>Mouse Controls</h3></td></tr>
        <tr><td class=mono>Hover</td><td>Snap crosshair to closest point.</td></tr>
        <tr><td class=mono>Shift + Hover</td><td>Highlight closest trace.</td></tr>
        <tr><td class=mono>Click</td><td>Select closest point.</td></tr>
        <tr><td class=mono>Drag</td><td>Zoom into rectangular region.</td></tr>
        <tr><td class=mono>Wheel</td><td>Remove rectangular zoom.</td></tr>
        <tr><td colspan=2><h3>Keyboard Controls</h3></td></tr>
        <tr><td class=mono>'w'/'s'</td><td>Zoom in/out.<sup>1</sup></td></tr>
        <tr><td class=mono>'a'/'d'</td><td>Pan left/right.<sup>1</sup></td></tr>
        <tr><td class=mono>'?'</td><td>Show help.</td></tr>
        <tr><td class=mono>Esc</td><td>Stop showing help.</td></tr>
      </table>
      <div class=footnote>
        <sup>1</sup> And Dvorak equivalents.
      </div>
      <div class=help-footer>
        <button class=action @click=${ele.closeHelp}>Close</button>
      </div>
    </dialog>

    <div id=tabs class="hide_on_query_only hide_on_spinner hide_on_pivot_table">
      <button class="collapser" id="collapseButton" @click=${(_e: Event) => ele.toggleDetails()}>
      ${
        ele.navOpen
          ? html`<expand-less-icon-sk></expand-less-icon-sk>`
          : html`<expand-more-icon-sk></expand-more-icon-sk>`
      }
      </button>
      <collapse-sk id=collapseDetails .closed=${!ele.navOpen}>
        <tabs-sk id=detailTab>
          <button>Params</button>
          <button id=commitsTab disabled>Details</button>
        </tabs-sk>
        <tabs-panel-sk>
          <div>
            <p>
              <b>Time</b>: <span title='Commit Time' id=commit_time></span>
            </p>

            <paramset-sk
              id=paramset
              clickable_values
              checkbox_values
              @paramset-key-value-click=${(e: CustomEvent<ParamSetSkClickEventDetail>) => {
                ele.paramsetKeyValueClick(e);
              }}
              @paramset-checkbox-click=${ele.paramsetCheckboxClick}
            >
            </paramset-sk>
          </div>
          <div id=details>
            <div id=params_and_logentry>
              <paramset-sk
                id=simple_paramset
                clickable_plus
                clickable_values
                copy_content
                @paramset-key-value-click=${(e: CustomEvent<ParamSetSkClickEventDetail>) => {
                  ele.paramsetKeyValueClick(e);
                }}
                @plus-click=${ele.plusClick}
                >
              </paramset-sk>
              <code><pre id=logEntry></pre></code>
              <anomaly-sk id=anomaly></anomaly-sk>
            </div>
            <div>
              <commit-range-sk id="commit-range-link"></commit-range-sk>
              <point-links-sk id="point-links"></point-links-sk>
              <commit-detail-panel-sk id=commits selectable .hide=${
                window.perf.hide_list_of_commits_on_explore
              }></commit-detail-panel-sk>
              <ingest-file-links-sk class="hide_on_pivot_plot" id=ingest-file-links>
              </ingest-file-links-sk>
              <json-source-sk class="hide_on_pivot_plot" id=jsonsource></json-source-sk>
            </div>
          </div>
        </tabs-panel-sk>
      </collapse-sk>
    </div>
  </div>
  </dataframe-repository-sk>
  <toast-sk id="pinpoint-job-toast" duration=10000>
    Pinpoint bisection started: <a href=${ele.jobUrl} target=_blank>${ele.jobId}</a>.
    <button id="hide-pinpoint-toast" class="action">Close</button>
  </toast-sk>
  <toast-sk id="triage-result-toast" duration=0>
    <span id="triage-result-text"></span><a id="triage-result-link"></a>
    <button id="hide-triage-toast" class="action">Close</button>
  </toast-sk>
  `;

  private plotSummaryTemplate() {
    return html`<plot-summary-sk
        ${ref(this.plotSummary)}
        @summary_selected=${this.summarySelected}
        selectionType=${!this._state.disableMaterial ? 'material' : 'canvas'}
        ?hasControl=${!this._state.disableMaterial}
        class="hide_on_pivot_table hide_on_query_only hide_on_spinner">
      </plot-summary-sk>
      <div id="summaryPicker">
        <picker-field-sk
          ${ref(this.summaryOptionsField)}
          @value-changed=${this.onSummaryPickerChanged}>
        </picker-field-sk>
        <label>${this.getPlotSummaryTraceLabel()}</label>
      </div>`;
  }

  private onSummaryPickerChanged(e: CustomEvent) {
    const selectedTrace = e.detail.value;
    this.traceKeyForSummary = this.traceNameIdMap.get(selectedTrace) || '';

    const plot = this.plotSummary.value;
    if (plot) {
      plot.selectedTrace = this.traceKeyForSummary;
    }
  }

  // onChartSelect shows the tooltip whenever a user clicks on a data
  // point and the tooltip will lock in place until it is closed.
  private onChartSelect(e: CustomEvent) {
    const chart = this.googleChartPlot!.value!;
    const selection = e.detail.chart.getSelection()[0];
    // TODO(b/370804498): Clicking on the same data point to show tooltip
    // and then clicking on it again to close it will create errors because
    // selection.row will be empty. Handle this use case.
    const index = {
      row: selection.row,
      col: selection.column,
    };

    const commitPos = chart.getCommitPosition(index.row);
    const position = chart.getPositionByIndex(index);
    const key = JSON.stringify([chart.getTraceName(index.col), commitPos]);
    const commit = this.pointToCommitDetailMap.get(key) || null;

    this.enableTooltip(
      {
        x: index.row - (this.selectedRange?.begin || 0),
        y: chart.getYValue(index),
        xPos: position.x,
        yPos: position.y,
        name: chart.getTraceName(index.col),
      },
      commit,
      false,
      true
    );

    this.tooltipSelected = true;
  }

  // if the tooltip is opened and the user is not shift-clicking,
  // close it when clicking on the chart
  // i.e. clicking away from the tooltip closes it
  private onChartMouseDown(): void {
    const tooltipElem = $$<ChartTooltipSk>('chart-tooltip-sk', this);
    this.tooltipSelected = false;
    tooltipElem?.moveTo(null);
  }

  // onChartOver shows the tooltip whenever a user hovers their mouse
  // over a data point in the google chart
  private onChartOver({ detail }: CustomEvent<PlotShowTooltipEventDetails>): void {
    // do not show tooltip if tooltip is selected
    if (this.tooltipSelected) {
      return;
    }
    const chart = this.googleChartPlot!.value!;
    const index = detail;
    const commitPos = chart.getCommitPosition(index.row);
    const position = chart.getPositionByIndex(index);
    const key = JSON.stringify([chart.getTraceName(index.col), commitPos]);
    const commit = this.pointToCommitDetailMap.get(key) || null;

    this.enableTooltip(
      {
        x: index.row - (this.selectedRange?.begin || 0),
        y: chart.getYValue(index),
        xPos: position.x,
        yPos: position.y,
        name: chart.getTraceName(index.col),
      },
      commit,
      false,
      true
    );
  }

  connectedCallback(): void {
    super.connectedCallback();
    if (this._initialized) {
      return;
    }
    this._initialized = true;
    this._render();

    this.anomalyTable = this.querySelector('#anomaly');
    this.commits = this.querySelector('#commits');
    this.commitsTab = this.querySelector('#commitsTab');
    this.detailTab = this.querySelector('#detailTab');
    this.formula = this.querySelector('#formula');
    this.jsonsource = this.querySelector('#jsonsource');
    this.ingestFileLinks = this.querySelector('#ingest-file-links');
    this.pointLinks = this.querySelector<PointLinksSk>('#point-links');
    this.logEntry = this.querySelector('#logEntry');
    this.paramset = this.querySelector('#paramset');
    this.percent = this.querySelector('#percent');
    this.pivotControl = this.querySelector('pivot-query-sk');
    this.pivotDisplayButton = this.querySelector('#pivot-display-button');
    this.pivotTable = this.querySelector('pivot-table-sk');
    this.query = this.querySelector('#query');
    this.fromParamsQueryCount = this.querySelector('#from-params-query-count');
    this.fromParamsQuery = this.querySelector('#from-params-query');
    this.queryCount = this.querySelector('query-count-sk');
    this.range = this.querySelector('#range');
    this.simpleParamset = this.querySelector('#simple_paramset');
    this.spinner = this.querySelector('#spinner');
    this.summary = this.querySelector('#summary');
    this.commitTime = this.querySelector('#commit_time');
    this.csvDownload = this.querySelector('#csv_download');
    this.queryDialog = this.querySelector('#query-dialog');
    this.bisectDialog = this.querySelector('#bisect-dialog');
    this.fromParamsQueryDialog = this.querySelector('#from-params-query-dialog');
    this.helpDialog = this.querySelector('#help');
    this.commitRangeSk = this.querySelector('#commit-range-link');
    this.pinpointJobToast = this.querySelector('#pinpoint-job-toast');
    this.closePinpointToastButton = this.querySelector('#hide-pinpoint-toast');
    this.triageResultToast = this.querySelector('#triage-result-toast');
    this.closeTriageToastButton = this.querySelector('#hide-triage-toast');
    this.bisectButton = this.querySelector('#bisect-button');
    this.collapseButton = this.querySelector('#collapseButton');
    this.traceDetails = this.querySelector('#traceDetails');
    this.traceDetailsCopy = this.querySelector('#traceDetailsCopy');
    this.graphTitle = this.querySelector<GraphTitleSk>('#graphTitle');

    // material UI stuff
    this.settingsDialog = this.querySelector<MdDialog>('#settings-dialog');

    // Populate the query element.
    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
    LoggedIn()
      .then((status: LoginStatus) => {
        this.user = status.email;
      })
      .catch(errorMessage);

    fetch(`/_/initpage/?tz=${tz}`, {
      method: 'GET',
    })
      .then(jsonOrThrow)
      .then((json) => {
        const now = Math.floor(Date.now() / 1000);
        this._state.begin = now - 60 * 60 * 24;
        this._state.end = now;
        this.range!.state = {
          begin: this._state.begin,
          end: this._state.end,
          num_commits: this._state.numCommits,
          request_type: this._state.requestType,
        };

        this.query!.key_order = window.perf.key_order || [];
        this.query!.paramset = json.dataframe.paramset;
        this.pivotControl!.paramset = json.dataframe.paramset;

        // Remove the paramset so it doesn't get displayed in the Params tab.
        json.dataframe.paramset = {};
      })
      .catch(errorMessage);

    this.closePinpointToastButton!.addEventListener('click', () => this.pinpointJobToast?.hide());
    this.closeTriageToastButton!.addEventListener('click', () => this.triageResultToast?.hide());

    // Add an event listener for when a new bug is filed or an existing bug is submitted in the tooltip.
    this.addEventListener('anomaly-changed', (e) => {
      this.plotSimple.value?.redrawOverlayCanvas();
      const detail = (e as CustomEvent).detail;
      if (!detail) {
        this.triageResultToast?.hide();
        return;
      }
      const bugId = (e as CustomEvent).detail.bugId;
      const newBug = (e as CustomEvent).detail.newBug;
      const toastText = document.getElementById('triage-result-text')! as HTMLSpanElement;
      const toastLink = document.getElementById('triage-result-link')! as HTMLAnchorElement;
      if (newBug === true) {
        toastText.textContent = `New bug created: `;
      } else {
        toastText.textContent = `Anomalies associated with: `;
      }
      const link = `https://issues.chromium.org/issues/${bugId}`;
      toastLink.setAttribute('href', `${link}`);
      toastLink.setAttribute('target', '_blank');
      toastLink.innerText = `${bugId}`;
      this.triageResultToast?.show();
    });
  }

  render(): void {
    this._render();
  }

  showSettingsDialog(_event: Event) {
    this.settingsDialog!.show();
  }

  switchXAxis(target: MdSwitch | null) {
    const plot = this.plotSimple.value;
    if (!plot) {
      return;
    }
    if (target!.selected) {
      this._state.labelMode = LabelMode.CommitPosition;
    } else {
      this._state.labelMode = LabelMode.Date;
    }

    const anomalyMap = plot.anomalyDataMap;
    plot.removeAll();
    this.AddPlotLines(this._dataframe.traceset, this.getLabels(this._dataframe.header!));
    plot.anomalyDataMap = anomalyMap;
    this._stateHasChanged();
  }

  private summaryChangeHandler(target: MdSwitch | null) {
    this._state.summary = target!.selected;
    this._userSpecifiedCustomizationParams.add('summary');
    this._stateHasChanged();
    this._render();
  }

  // Call this anytime something in private state is changed. Will be replaced
  // with the real function once stateReflector has been setup.
  // eslint-disable-next-line @typescript-eslint/no-empty-function
  private _stateHasChanged = () => {
    this.showRemoveAll = this._state.show_remove_all;

    // If chart tooltip is enabled do not show crosshair label
    if (this.plotSimple.value) {
      this.plotSimple.value.showCrosshairLabel = !this._state.enable_chart_tooltip;
    }

    this.dispatchEvent(new CustomEvent('state_changed', {}));
  };

  private _renderedTraces = () => {
    this.dispatchEvent(new CustomEvent('rendered_traces', {}));
  };

  private closeQueryDialog(): void {
    this.queryDialog!.close();
    this._dialogOn = false;
  }

  private closeBisectDialog(): void {
    this.bisectDialog!.close();
    this._dialogOn = false;
  }

  private postBisect(): void {
    this.bisectButton!.disabled = true;
    const parts: string[] = this.simpleParamset!.paramsets[0]!.test[0].split('_');
    const tail: string = parts.pop()!;
    const chart = STATISTIC_VALUES.includes(tail)
      ? parts.join('_')
      : this.simpleParamset!.paramsets[0]!.test[0];
    const statistic = STATISTIC_VALUES.includes(tail) ? tail : '';
    const bugId = document.getElementById('bug-id')! as HTMLInputElement;
    const startCommit = document.getElementById('start-commit')! as HTMLInputElement;
    const endCommit = document.getElementById('end-commit')! as HTMLInputElement;
    const story = document.getElementById('story')! as HTMLInputElement;
    const patch = document.getElementById('patch')! as HTMLInputElement;
    const req: CreateBisectRequest = {
      comparison_mode: 'performance',
      start_git_hash: startCommit.value,
      end_git_hash: endCommit.value,
      configuration: this.testPath.split('/')[1],
      benchmark: this.testPath.split('/')[2],
      story: story.value,
      chart: chart,
      statistic: statistic,
      comparison_magnitude: '',
      pin: patch.value,
      project: 'chromium',
      bug_id: bugId.value,
      user: this.user,
    };
    fetch('/_/bisect/create', {
      method: 'POST',
      body: JSON.stringify(req),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((json: CreateBisectResponse) => {
        this.jobUrl = json.jobUrl;
        this.jobId = json.jobId;
        this._render();
        this.pinpointJobToast?.show();
      })
      .catch(errorMessage)
      .finally(() => {
        this.bisectButton!.disabled = false;
        this.closeBisectDialog();
      });
  }

  keyDown(e: KeyboardEvent) {
    // Ignore IME composition events.
    if (this._dialogOn || e.isComposing || e.keyCode === 229) {
      return;
    }

    // Allow user to type and not pan graph if the Existing Bug Dialog is showing.
    const existing_bug_dialog = $$<ExistingBugDialogSk>('existing-bug-dialog-sk', this);
    if (existing_bug_dialog && existing_bug_dialog.isActive) {
      return;
    }

    // Allow user to type and not pan graph if the New Bug Dialog is showing.
    const new_bug_dialog = $$<NewBugDialogSk>('new-bug-dialog-sk', this);
    if (new_bug_dialog && new_bug_dialog.opened) {
      return;
    }

    // Allow user to type and not pan graph if an input box is active.
    const activeElement = document.activeElement;
    if (activeElement instanceof HTMLInputElement) {
      return;
    }

    switch (e.key) {
      case '?':
        this.helpDialog!.showModal();
        break;
      case ',': // dvorak
      case 'w':
        this.zoomInKey();
        break;
      case 'o': // dvorak
      case 's':
        this.zoomOutKey();
        break;
      case 'a':
        this.zoomLeftKey();
        break;
      case 'e': // dvorak
      case 'd':
        this.zoomRightKey();
        break;
      case `Escape`:
        this.disableTooltip();
        break;
      case `Esc`:
        this.disableTooltip();
        break;
      default:
        break;
    }
  }

  /**
   * The current zoom and the length between the left and right edges of
   * the zoom as an object of the form:
   *
   *   {
   *     zoom: [2.0, 12.0],
   *     delta: 10.0,
   *   }
   */
  private getCurrentZoom(): ZoomWithDelta {
    let zoom = this.plotSimple.value?.zoom;
    if (!zoom) {
      zoom = [0, this._dataframe.header!.length - 1];
    }
    let delta = zoom[1] - zoom[0];
    if (delta < MIN_ZOOM_RANGE) {
      const mid = (zoom[0] + zoom[1]) / 2;
      zoom[0] = mid - MIN_ZOOM_RANGE / 2;
      zoom[1] = mid + MIN_ZOOM_RANGE / 2;
      delta = MIN_ZOOM_RANGE;
    }
    return {
      zoom: zoom as CommitRange,
      delta: delta as CommitNumber,
    };
  }

  /**
   * Clamp a single zoom endpoint.
   */
  private clampZoomIndexToDataFrame(z: CommitNumber): CommitNumber {
    if (z < 0) {
      z = CommitNumber(0);
    }
    if (z > this._dataframe.header!.length - 1) {
      z = (this._dataframe.header!.length - 1) as CommitNumber;
    }
    return z as CommitNumber;
  }

  /**
   * Fixes up the zoom range so it always make sense.
   *
   * @param {Array<Number>} zoom - The zoom range.
   * @returns {Array<Number>} The zoom range.
   */
  private rationalizeZoom(zoom: CommitRange): CommitRange {
    if (zoom[0] > zoom[1]) {
      const left = zoom[0];
      zoom[0] = zoom[1];
      zoom[1] = left;
    }
    return zoom;
  }

  /**
   * Zooms to the desired range, or changes the range of commits being displayed
   * if the zoom range extends past either end of the current commits.
   *
   * @param zoom is the desired zoom range. Each number is an index into the
   * dataframe.
   */
  private zoomOrRangeChange(zoom: CommitRange) {
    zoom = this.rationalizeZoom(zoom);
    const clampedZoom: CommitRange = [
      this.clampZoomIndexToDataFrame(zoom[0]),
      this.clampZoomIndexToDataFrame(zoom[1]),
    ];
    const offsets: [CommitNumber, CommitNumber] = [
      this._dataframe.header![0]!.offset,
      this._dataframe.header![this._dataframe.header!.length - 1]!.offset,
    ];

    const result = calculateRangeChange(zoom, clampedZoom, offsets);

    if (result.rangeChange) {
      // Convert the offsets into timestamps, which are needed when building
      // dataframes.
      const req: ShiftRequest = {
        begin: result.newOffsets![0],
        end: result.newOffsets![1],
      };
      fetch('/_/shift/', {
        method: 'POST',
        body: JSON.stringify(req),
        headers: {
          'Content-Type': 'application/json',
        },
      })
        .then(jsonOrThrow)
        .then((json: ShiftResponse) => {
          this._state.begin = json.begin;
          this._state.end = json.end;
          this._state.requestType = 0;
          this._stateHasChanged();
          this.rangeChangeImpl();
        })
        .catch(errorMessage);
    } else {
      if (this.plotSimple.value) {
        this.plotSimple.value.zoom = zoom;
      }
    }
  }

  private pivotChanged(e: CustomEvent<PivotQueryChangedEventDetail>): void {
    // Only enable the Display button if we have a valid pivot.Request and a
    // query.
    this.pivotDisplayButton!.disabled =
      validatePivotRequest(e.detail) !== '' || this.query!.current_query.trim() === '';
    if (!e.detail || e.detail.summary!.length === 0) {
      this.pivotDisplayButton!.textContent = 'Display';
    } else {
      this.pivotDisplayButton!.textContent = 'Display Table';
    }
  }

  private zoomInKey() {
    const cz = this.getCurrentZoom();
    const zoom: CommitRange = [
      (cz.zoom[0] + ZOOM_JUMP_PERCENT * cz.delta) as CommitNumber,
      (cz.zoom[1] - ZOOM_JUMP_PERCENT * cz.delta) as CommitNumber,
    ];
    this.zoomOrRangeChange(zoom);
  }

  private zoomOutKey() {
    const cz = this.getCurrentZoom();
    const zoom: CommitRange = [
      (cz.zoom[0] - ZOOM_JUMP_PERCENT * cz.delta) as CommitNumber,
      (cz.zoom[1] + ZOOM_JUMP_PERCENT * cz.delta) as CommitNumber,
    ];
    this.zoomOrRangeChange(zoom);
  }

  private zoomLeftKey() {
    const cz = this.getCurrentZoom();
    const zoom: CommitRange = [
      (cz.zoom[0] - ZOOM_JUMP_PERCENT * cz.delta) as CommitNumber,
      (cz.zoom[1] - ZOOM_JUMP_PERCENT * cz.delta) as CommitNumber,
    ];
    this.zoomOrRangeChange(zoom);
  }

  private zoomRightKey() {
    const cz = this.getCurrentZoom();
    const zoom: CommitRange = [
      (cz.zoom[0] + ZOOM_JUMP_PERCENT * cz.delta) as CommitNumber,
      (cz.zoom[1] + ZOOM_JUMP_PERCENT * cz.delta) as CommitNumber,
    ];
    this.zoomOrRangeChange(zoom);
  }

  /**  Returns true if we have any traces to be displayed. */
  private hasData() {
    // We have data if at least one traceID isn't a special name.
    return Object.keys(this._dataframe.traceset).some(
      (traceID) => !SPECIAL_TRACE_NAMES.includes(traceID)
    );
  }

  /** Open the query dialog box. */
  openQuery() {
    this._render();
    this._dialogOn = true;
    this.queryDialog!.showModal();
    // If there is a query already plotted, update the counts on the query dialog.
    if (this._state.queries.length > 0) {
      this.queryCount!.current_query = this.applyDefaultsToQuery(this.query!.current_query);
    }
  }

  /** Open the bisect dialog box. */
  private openBisect() {
    this._render();
    this._dialogOn = true;
    this.bisectDialog!.showModal();
  }

  private paramsetChanged(e: CustomEvent<ParamSet>) {
    this.query!.paramset = e.detail;
    this.pivotControl!.paramset = e.detail;
    this._render();
  }

  /** Called when the query-count-sk element has finished querying the server
   * for an updated ParamSet. */
  private fromParamsParamsetChanged(e: CustomEvent<ParamSet>) {
    this.fromParamsQuery!.paramset = e.detail;
    this.fromParamsQuery!.selectKey(this.fromParamsKey);
    this._render();
  }

  private fromParamsCloseQueryDialog() {
    this.fromParamsQueryDialog!.close();
  }

  private fromParamsOKQueryDialog() {
    // This query only contains the key this.fromParamsKey and it's values, so we need
    // to construct the full query using the traceID.
    // Note:  toParamSet(s: string) returns CommonSkParamSet, not ParamSet. Hence the cast.
    const updatedParamValues = ParamSet(toParamSet(this.fromParamsQuery!.current_query));
    const traceIDAsQuery: ParamSet = paramsToParamSet(fromKey(this._state.selected.name));

    // Merge the two ParamSets.
    const newQuery: ParamSet = Object.assign(traceIDAsQuery, updatedParamValues);
    this.addFromQueryOrFormula(false, 'query', fromParamSet(newQuery), '');
    this.fromParamsQueryDialog!.close();
  }

  /** Handles clicks on the '+' icons on the Details tab Params. */
  plusClick(e: CustomEvent<ParamSetSkPlusClickEventDetail>): void {
    // Record the Params key that was clicked on.
    this.fromParamsKey = e.detail.key;

    // Convert the traceID into a ParamSet.
    const keyAsParamSet: ParamSet = paramsToParamSet(fromKey(this._state.selected.name));

    // And remove the Params key that was clicked on.
    keyAsParamSet[this.fromParamsKey] = [];

    // Convert the ParamSet back into a query to pass to
    // this.fromParamsQueryCount, which will query the server for the number of
    // traces that match the new query, and also return a ParamSet we can use to
    // populate the query-sk control.
    this.fromParamsQueryCount!.current_query = fromParamSet(keyAsParamSet);

    // To avoid the dialog displaying state data we populate the ParamSet
    // and select our key which will display and empty set of value choices
    // until this.fromParamsQueryCount is done.
    this.fromParamsQuery!.paramset = keyAsParamSet;
    this.fromParamsQuery!.selectKey(this.fromParamsKey);

    this.fromParamsQueryDialog?.showModal();
  }

  private queryChangeDelayedHandler(e: CustomEvent<QuerySkQueryChangeEventDetail>) {
    this.queryCount!.current_query = this.applyDefaultsToQuery(e.detail.q);
  }

  /** Reflect the current query to the query summary. */
  private queryChangeHandler(e: CustomEvent<QuerySkQueryChangeEventDetail>) {
    const query = e.detail.q;
    this.summary!.paramsets = [toParamSet(query)];
    const formula = this.formula!.value;
    if (formula === '') {
      this.formula!.value = `filter("${query}")`;
    } else if ((formula.match(/"/g) || []).length === 2) {
      // Only update the filter query if there's one string in the formula.
      this.formula!.value = formula.replace(/".*"/, `"${query}"`);
    }
  }

  private pivotTableSortChange(e: CustomEvent<PivotTableSkChangeEventDetail>): void {
    this._state.sort = e.detail;
    this._stateHasChanged();
  }

  /** Reflect the focused trace in the paramset. */
  private plotTraceFocused({ detail }: CustomEvent<PlotSimpleSkTraceEventDetails>) {
    const header = this.dfRepo.value?.dataframe.header;
    const selected = header![(this.selectedRange?.begin || 0) + detail.x]!;
    this.paramset!.highlight = fromKey(detail.name);
    this.commitTime!.textContent = new Date(selected.timestamp * 1000).toLocaleString();
    const formattedTrace = this.traceFormatter!.formatTrace(fromKey(detail.name));
    this.traceDetails!.textContent = formattedTrace;
    this.traceDetailsCopy!.onclick = () => {
      navigator.clipboard.writeText(formattedTrace);
    };
    this.traceDetailsCopy!.style.display = 'block';

    if (this._state.enable_chart_tooltip && !this.tooltipSelected) {
      // if the commit details for a point is already loaded then
      // show those commit details on hover
      let c = null;
      const commitPos = selected.offset;
      const key = JSON.stringify([detail.name, commitPos]);
      if (this.pointToCommitDetailMap.has(key)) {
        c = this.pointToCommitDetailMap.get(key) || null;
      }

      this.enableTooltip(detail, c, false, false);
    }
  }

  /** User has zoomed in on the graph. */
  private plotZoom() {
    this._render();
  }

  /** get story name for pinpoint. */
  private getLastSubtest(d: any) {
    const tmp =
      d.subtest_7 ||
      d.subtest_6 ||
      d.subtest_5 ||
      d.subtest_4 ||
      d.subtest_3 ||
      d.subtest_2 ||
      d.subtest_1;
    return tmp ? tmp[0] : '';
  }

  private selectedRange?: range;

  /**
   * React to the summary_selected event.
   * @param e Event object.
   */
  summarySelected({ detail }: CustomEvent<PlotSummarySkSelectionEventDetails>): void {
    this.updateSelectedRangeWithUpdatedDataframe(detail.value, detail.domain);
  }

  private extendRange(range: range) {
    const dfRepo = this.dfRepo.value;
    const header = dfRepo?.dataframe?.header;
    if (!dfRepo || !header || dfRepo.loading) {
      return;
    }

    if (range.begin < header[0]!.offset) {
      dfRepo.extendRange(-monthInSec);
    } else if (range.end > header[header.length - 1]!.offset) {
      dfRepo.extendRange(monthInSec);
    }
  }

  private OnSelectionRange({ type, detail }: CustomEvent<PlotSelectionEventDetails>): void {
    if (type === 'selection-changed') {
      this.extendRange(detail.value);
    }

    if (this.plotSummary.value) {
      this.plotSummary.value.selectedValueRange = detail.value;
    }
  }

  private updateSelectedRangeWithUpdatedDataframe(
    range: range,
    domain: 'commit' | 'date',
    replot = true
  ) {
    // Only supports commit value range.
    if (this.googleChartPlot.value && domain === 'commit') {
      this.googleChartPlot.value.selectedRange = range;
    }

    const plot = this.plotSimple.value;
    const df = this.dfRepo.value?.dataframe;
    const header = df?.header || [];
    const selected = findSubDataframe(header!, range, domain);
    this.selectedRange = selected;

    const subDataframe = generateSubDataframe(df!, selected);
    const anomalyMap = findAnomalyInRange(this.dfRepo.value?.anomaly || {}, {
      begin: header[Math.min(selected.begin, header.length - 1)]!.offset,
      end: header[Math.min(selected.end, header.length - 1)]!.offset,
    });

    // Update the current dataframe to reflect the selection.
    this._dataframe.traceset = subDataframe.traceset;
    this._dataframe.header = subDataframe.header;

    if (!plot) {
      return;
    }

    if (replot) {
      plot.removeAll();
      this.AddPlotLines(subDataframe.traceset, this.getLabels(subDataframe.header!));
    }

    if (anomalyMap) {
      plot.anomalyDataMap = getAnomalyDataMap(
        subDataframe.traceset,
        subDataframe.header!,
        anomalyMap,
        this.state.highlight_anomalies
      );
    }
  }

  enableTooltip(
    pointDetails: PlotSimpleSkTraceEventDetails,
    commit: Commit | null,
    displayFileLinks: boolean,
    fixTooltip: boolean
  ): void {
    // explore-simple-sk is used multiple times on the multi-graph view. To
    // make sure that appropriate chart-tooltip-sk element is selected, we
    // start the search from the explore-simple-sk that the user is hovering/
    // clicking on
    const tooltipElem = $$<ChartTooltipSk>('chart-tooltip-sk', this);

    const x = (this.selectedRange?.begin || 0) + pointDetails.x;
    const testName = pointDetails.name;
    const commitPosition = this.dfRepo.value!.dataframe.header![x]!.offset;
    const anomaly = this.dfRepo.value?.getAnomaly(testName, commitPosition) || null;

    // TODO(b/370804498): To be refactored into google plot / dataframe.
    // The anomaly data is indirectly referenced from simple-plot, and the anomaly data gets
    // updated in place in triage popup. This may cause the data inconsistency to manipulate
    // data in several places.
    // Ideally, dataframe_context should nudge anomaly data.
    const anomalyDataMap = this.plotSimple.value?.anomalyDataMap;
    let anomalyDataInPlot: AnomalyData | null = null;
    if (anomalyDataMap) {
      const traceAnomalies = anomalyDataMap[testName];
      if (traceAnomalies) {
        for (let i = 0; i < traceAnomalies.length; i++) {
          if (pointDetails.x === traceAnomalies[i].x) {
            anomalyDataInPlot = traceAnomalies[i];
            break;
          }
        }
      }
    }
    // -- to be refactored. see above--

    // Map an anomaly ID to a list of Nudge Entries.
    // TODO(b/375678060): Reflect anomaly coordinate changes unto summary bar.
    const nudgeList: NudgeEntry[] = [];
    if (anomaly) {
      // This is only to be backward compatible with anomaly data in simple-plot.
      const anomalyData = this.plotSimple.value
        ? anomalyDataInPlot
        : {
            anomaly: anomaly,
            x: commitPosition,
            y: pointDetails.y,
            highlight: false,
          };

      const headerLength = this.dfRepo.value!.dataframe.header!.length;
      for (let i = -NUDGE_RANGE; i <= NUDGE_RANGE; i++) {
        if (x + i <= 0 || x + i >= headerLength) {
          continue;
        }
        const start_revision = this.dfRepo.value!.dataframe.header![x + i - 1]!.offset + 1;
        const end_revision = this.dfRepo.value!.dataframe.header![x + i]!.offset;
        const y = this.dfRepo.value!.dataframe.traceset![testName][x + i];
        nudgeList.push({
          display_index: i,
          anomaly_data: anomalyData,
          selected: i === 0,
          start_revision: start_revision,
          end_revision: end_revision,
          x: pointDetails.x + i,
          y: y,
        });
      }
    }

    tooltipElem!.moveTo({ x: pointDetails.xPos!, y: pointDetails.yPos! });

    const closeBtnAction = fixTooltip
      ? () => {
          this.tooltipSelected = false;
          tooltipElem!.moveTo(null);
        }
      : () => {};

    tooltipElem!.load(
      this.traceFormatter!.formatTrace(fromKey(testName)),
      testName,
      pointDetails.y,
      commitPosition,
      anomaly,
      nudgeList,
      commit,
      displayFileLinks,
      fixTooltip,
      closeBtnAction
    );
  }

  // if the user's cursor leaves the graph, close the tooltip
  // unless the tooltip is 'fixed', meaning the user has anchored it.
  mouseLeave(): void {
    if (this.tooltipSelected) {
      return;
    }

    this.disableTooltip();
  }

  /** Hides the tooltip. Generally called when mouse moves out of the graph */
  disableTooltip(): void {
    const tooltipElem = $$<ChartTooltipSk>('chart-tooltip-sk', this);
    tooltipElem!.moveTo(null);
    this._render();
  }

  /** Highlight a trace when it is clicked on. */
  traceSelected({ detail }: CustomEvent<PlotSimpleSkTraceEventDetails>): void {
    this.plotSimple.value!.highlight = [detail.name];
    this.plotSimple.value!.xbar = detail.x;
    this.commits!.details = [];

    // If traces are rendered and summary bar is enabled, show
    // summary for the trace clicked on the graph.
    if (this.summaryOptionsField.value) {
      const formattedTraceId = this.traceFormatter!.formatTrace(fromKey(detail.name));
      this.summaryOptionsField.value!.setValue(formattedTraceId);
    }

    const selected = this.selectedRange?.begin || 0;
    const x = selected + detail.x;

    if (x < 0) {
      return;
    }
    // loop backwards from x until you get the next
    // non MISSING_DATA_SENTINEL point.
    const commit: CommitNumber = this.dfRepo.value?.dataframe.header![x]?.offset as CommitNumber;
    if (!commit) {
      return;
    }

    const commits: CommitNumber[] = [commit];

    // Find all the commit ids between the commit that was clicked on, and the
    // previous commit on the display, inclusive of the commit that was clicked,
    // and non-inclusive of the previous commit.

    // We always do this, but the response may not contain all the commit info
    // if alerts.DefaultSparse==true, in which case only info for the first
    // commit is returned.

    // First skip back to the next point with data.
    const trace = this.dfRepo.value?.dataframe.traceset[detail.name] || [];
    const header = this.dfRepo.value?.header || [];
    let prevCommit: CommitNumber = CommitNumber(-1);
    for (let i = x - 1; i >= 0; i--) {
      if (trace![i] !== MISSING_DATA_SENTINEL) {
        prevCommit = header[i]!.offset as CommitNumber;
        break;
      }
    }

    // Populate the commit-range-sk element.
    this.commitRangeSk!.trace = trace;
    this.commitRangeSk!.commitIndex = detail.x;
    this.commitRangeSk!.header = header;

    if (prevCommit !== -1) {
      for (let c = commit - 1; c > prevCommit; c--) {
        commits.push(c as CommitNumber);
      }
    }

    // Find if selected point is an anomaly.
    let selected_anomaly: Anomaly | null = null;
    // TODO(b/362831653) - Update this to Google Chart once plot-simple-sk is deprecated.
    if (detail.name in this.plotSimple.value!.anomalyDataMap) {
      const anomalyData = this.plotSimple.value!.anomalyDataMap[detail.name];
      for (let i = 0; i < anomalyData.length; i++) {
        if (anomalyData[i].x === detail.x) {
          selected_anomaly = anomalyData[i].anomaly;
          break;
        }
      }
    }

    const paramset = ParamSet({});
    this.simpleParamset!.paramsets = [];

    // If the trace name is a function key like norm(,a=1,b=2,)
    // extract the actual trace name ,a=1,b=1, from it to display
    // the params and values on the paramset table
    const funcKeyRegex = new RegExp(/\w+\(,.*,\)/);
    const isFuncKey = detail.name.match(funcKeyRegex);

    // Convert the trace id (if valid) into a paramset to display.
    let tName = '';
    if (isFuncKey) {
      const keyRegex = new RegExp(/(?<=\()(.*?)(?=\))/);
      tName = detail.name.match(keyRegex)![0];
    } else if (validKey(detail.name)) {
      tName = detail.name;
    }

    if (tName !== '') {
      const params: { [key: string]: string } = fromKey(tName);
      Object.keys(params).forEach((key) => {
        paramset[key] = [params[key]];
      });

      this.simpleParamset!.paramsets = [paramset as CommonSkParamSet];
    }

    this._render();

    this._state.selected.name = detail.name;
    this._state.selected.commit = commit;
    this._stateHasChanged();

    // Request populated commits from the server.
    fetch('/_/cid/', {
      method: 'POST',
      body: JSON.stringify(commits),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((json: CIDHandlerResponse) => {
        this.commits!.details = json.commitSlice || [];
        this.commitsTab!.disabled = false;
        this.simpleParamset!.paramsets = [paramset as CommonSkParamSet];
        this.logEntry!.innerHTML = escapeAndLinkifyToString(json.logEntry);
        this.anomalyTable!.anomaly = selected_anomaly;
        this.anomalyTable!.bugHostUrl = window.perf.bug_host_url;
        this.detailTab!.selected = COMMIT_TAB_INDEX;
        const cid = commits[0]!;
        const traceid = detail.name;
        const parts = [];
        this.story = this.getLastSubtest(this.simpleParamset!.paramsets[0]!);
        if (
          this.simpleParamset!.paramsets[0]!.master &&
          this.simpleParamset!.paramsets[0]!.master.length > 0
        ) {
          parts.push(this.simpleParamset!.paramsets[0]!.master[0]);
        }
        if (
          this.simpleParamset!.paramsets[0]!.bot &&
          this.simpleParamset!.paramsets[0]!.bot.length > 0
        ) {
          parts.push(this.simpleParamset!.paramsets[0]!.bot[0]);
        }
        if (
          this.simpleParamset!.paramsets[0]!.benchmark &&
          this.simpleParamset!.paramsets[0]!.benchmark.length > 0
        ) {
          parts.push(this.simpleParamset!.paramsets[0]!.benchmark[0]);
        }
        if (
          this.simpleParamset!.paramsets[0]!.test &&
          this.simpleParamset!.paramsets[0]!.test.length > 0
        ) {
          parts.push(this.simpleParamset!.paramsets[0]!.test[0]);
        }
        parts.push(this.story);
        this.testPath = parts.join('/');
        this.startCommit = prevCommit.toString();
        this.endCommit = commit.toString();
        if (selected_anomaly !== null && selected_anomaly.bug_id !== -1) {
          this.bugId = selected_anomaly.bug_id.toString();
        } else {
          this.bugId = '';
        }
        if (this.displayMode === 'display_plot') {
          this.jsonsource!.cid = cid;
          this.jsonsource!.traceid = traceid;
          this.ingestFileLinks!.load(cid, traceid);
          // Populate the point links element.
          this.pointLinks!.load(
            commit,
            prevCommit,
            detail.name,
            window.perf.keys_for_commit_range!
          );
        }

        // when the commit details are loaded, add those info to
        // pointToCommitDetailMap map which can be used to fetch commit
        // info on hover without making an API call
        this.pointToCommitDetailMap.set(JSON.stringify([detail.name, cid]), json.commitSlice![0]);

        const tooltipEnabled = this._state.enable_chart_tooltip;
        const hasValidTooltipPos = detail.xPos !== undefined && detail.yPos !== undefined;
        if (tooltipEnabled && hasValidTooltipPos) {
          this.tooltipSelected = true;

          this.enableTooltip(detail, json.commitSlice![0], true, true);
        }
      })
      .catch(errorMessage);

    // Open the details section if it is currently collapsed
    if (!this.navOpen) {
      this.collapseButton?.click();
    }
  }

  private clearSelectedState() {
    const plot = this.plotSimple.value;
    if (plot) {
      plot.highlight = [];
      plot.xbar = -1;
    }

    // Switch back to the params tab since we are about to hide the details tab.
    this.detailTab!.selected = PARAMS_TAB_INDEX;
    this.commitsTab!.disabled = true;
    this.logEntry!.textContent = '';

    this._state.selected = defaultPointSelected();
    this._stateHasChanged();
  }

  /**
   * Fixes up the time ranges in the state that came from query values.
   *
   * It is possible for the query URL to specify just the begin or end time,
   * which may end up giving us an inverted time range, i.e. end < begin.
   */
  private rationalizeTimeRange(state: State): State {
    if (state.end <= state.begin) {
      // If dense then just make sure begin is before end.
      if (state.requestType === 1) {
        state.begin = state.end - DEFAULT_RANGE_S;
      } else if (this._state.begin !== state.begin) {
        state.end = state.begin + DEFAULT_RANGE_S;
      } else {
        // They set 'end' in the URL.
        state.begin = state.end - DEFAULT_RANGE_S;
      }
    }
    return state;
  }

  /**
   * Handler for the event when the remove paramset value button is clicked
   * @param e Remove event
   */
  private paramsetRemoveClick(e: CustomEvent<ParamSetSkRemoveClickEventDetail>) {
    // Remove the specified value from the query
    this.query?.removeKeyValue(e.detail.key, e.detail.value);
  }

  /**
   * Handler for the event when the paramset checkbox is clicked.
   * @param e Checkbox click event
   */
  private paramsetCheckboxClick(e: CustomEvent<ParamSetSkCheckboxClickEventDetail>) {
    if (!e.detail.selected) {
      // Find the matching traces and remove them from the dataframe's traceset.
      const keys: string[] = [];
      Object.keys(this._dataframe.traceset).forEach((key) => {
        if (_matches(key, e.detail.key, e.detail.value!)) {
          keys.push(key);
        }
      });
      this.removeKeys(keys, false);
    } else {
      // Adding is slightly more involved. The current dataframe may have matching traces removed,
      // so we need to look at the original trace set to find matching traces. If we find any
      // match, we add it to the current dataframe and then add the lines to the rendered plot.
      const traceSet = TraceSet({});
      Object.keys(this.originalTraceSet).forEach((key) => {
        if (_matches(key, e.detail.key, e.detail.value!)) {
          if (!(key in this._dataframe.traceset)) {
            this._dataframe.traceset[key] = this.originalTraceSet[key];
          }
          traceSet[key] = this.originalTraceSet[key];
        }
      });
      this.AddPlotLines(traceSet, []);
    }
  }

  /**
   * Adds the plot lines to the plot-simple-sk module.
   * @param traceSet The traceset input.
   * @param labels The xAxis labels.
   */
  private AddPlotLines(traceSet: { [key: string]: number[] }, labels: tick[]) {
    this.plotSimple.value?.addLines(traceSet, labels);
    if (this._state.plotSummary) {
      this.addPlotSummaryOptions();
    }
  }

  /**
   * Returns the label for the selected plot summary trace.
   */
  private getPlotSummaryTraceLabel(): string {
    if (this.traceKeyForSummary !== '') {
      return this.traceFormatter!.formatTrace(fromKey(this.traceKeyForSummary));
    }

    return '';
  }

  /**
   * Adds the option list for the plot summary selection.
   */
  private addPlotSummaryOptions() {
    if (!this.dfRepo.value?.dataframe) {
      return;
    }
    const traceIds: string[] = [];
    Object.keys(this.dfRepo.value.dataframe.traceset!).forEach((traceId) => {
      // Ignore the zero trace if present.
      if (traceId !== ZERO_NAME) {
        const traceName = this.traceFormatter!.formatTrace(fromKey(traceId));
        this.traceNameIdMap.set(traceName, traceId);
        traceIds.push(traceName);
      }
    });

    if (this.summaryOptionsField.value) {
      this.summaryOptionsField.value!.options = traceIds;
      this.summaryOptionsField.value!.label = 'Trace for summary bar';
    }
  }

  private paramsetKeyValueClick(e: CustomEvent<ParamSetSkClickEventDetail>) {
    const keys: string[] = [];
    Object.keys(this._dataframe.traceset).forEach((key) => {
      if (_matches(key, e.detail.key, e.detail.value!)) {
        keys.push(key);
      }
    });

    if (this.plotSimple.value) {
      // Additively highlight if the ctrl key is pressed.
      if (e.detail.ctrl) {
        this.plotSimple.value!.highlight = this.plotSimple.value!.highlight.concat(keys);
      } else {
        this.plotSimple.value!.highlight = keys;
      }
    }
    this._render();
  }

  private closeHelp() {
    this.helpDialog!.close();
  }

  /** Create a FrameRequest that will fill in data specified by this._state.begin and end,
   *  but is not yet present in this._dataframe.
   */
  private requestFrameBodyDeltaFromState(): FrameRequest {
    return this.requestFrameBodyFullFromState();
  }

  /** Create a FrameRequest that will re-create the current state of the page. */
  private requestFrameBodyFullFromState(): FrameRequest {
    return this.requestFrameBodyFullForRange(this._state.begin, this._state.end);
  }

  /**
   * Create a FrameRequest that recreates the current state of the page
   * for the given range.
   * @param begin Start time.
   * @param end End time.
   * @returns FrameRequest object.
   */
  private requestFrameBodyFullForRange(begin: number, end: number): FrameRequest {
    return {
      begin: begin,
      end: end,
      num_commits: this._state.numCommits,
      request_type: this._state.requestType,
      formulas: this._state.formulas,
      queries: this._state.queries,
      keys: this._state.keys,
      tz: Intl.DateTimeFormat().resolvedOptions().timeZone,
      pivot:
        validatePivotRequest(this._state.pivotRequest) === '' ? this._state.pivotRequest : null,
      disable_filter_parent_traces: this._state.disable_filter_parent_traces,
    };
  }

  /** Reload all the queries/formulas on the given time range. */
  private rangeChangeImpl() {
    if (!this._state) {
      return;
    }
    if (
      this._state.formulas.length === 0 &&
      this._state.queries.length === 0 &&
      this._state.keys === ''
    ) {
      return;
    }

    const body = this.requestFrameBodyDeltaFromState();

    if (body.begin === body.end) {
      console.log('skipped fetching this dataframe because it would be empty anyways');
      return;
    }
    this.commitTime!.textContent = '';
    const switchToTab = body.formulas!.length > 0 || body.queries!.length > 0 || body.keys !== '';
    this.requestFrame(body, (json) => {
      if (json === null || json === undefined) {
        errorMessage('Failed to find any matching traces.');
        return;
      }
      this.dfRepo.value
        ?.resetWithDataframeAndRequest(json.dataframe!, json.anomalymap, body)
        .then(() => {
          this.addTraces(json, switchToTab);
          this._render();
          if (isValidSelection(this._state.selected)) {
            const e = selectionToEvent(this._state.selected, this._dataframe.header);
            // If the range has moved to no longer include the selected commit then
            // clear the selection.
            if (e.detail.x === -1) {
              this.clearSelectedState();
            } else {
              this.traceSelected(e);
            }
          }
        });
    });
  }

  private zeroChangeHandler(target: MdSwitch | null) {
    this._state.showZero = target!.selected;
    this._stateHasChanged();
    this.zeroChanged();
  }

  private toggleDotsHandler() {
    this._state.dots = !this._state.dots;
    this._stateHasChanged();
    if (this.plotSimple.value) {
      this.plotSimple.value.dots = this._state.dots;
    }
  }

  private zeroChanged() {
    if (!this._dataframe.header) {
      return;
    }
    if (this._state.showZero) {
      const lines: { [key: string]: number[] } = {};
      lines[ZERO_NAME] = Array(this._dataframe.header.length).fill(0);
      this.AddPlotLines(lines, []);
    } else {
      this.plotSimple.value?.deleteLines([ZERO_NAME]);
    }
  }

  private autoRefreshHandler(target: MdSwitch | null) {
    this._state.autoRefresh = target!.selected;
    this._stateHasChanged();
    this.autoRefreshChanged();
  }

  private autoRefreshChanged() {
    if (!this._state.autoRefresh) {
      if (this._refreshId !== -1) {
        clearInterval(this._refreshId);
      }
    } else {
      this._refreshId = window.setInterval(() => this.autoRefresh(), REFRESH_TIMEOUT);
    }
  }

  private autoRefresh() {
    // Update end to be now.
    this._state.end = Math.floor(Date.now() / 1000);
    const body = this.requestFrameBodyFullFromState();
    const switchToTab = body.formulas!.length > 0 || body.queries!.length > 0 || body.keys !== '';
    this.requestFrame(body, (json) => {
      this.dfRepo.value
        ?.resetWithDataframeAndRequest(json.dataframe!, json.anomalymap, body)
        .then(() => {
          this.plotSimple.value?.removeAll();
          this.addTraces(json, switchToTab);
        });
    });
  }

  /**
   * Add traces to the display. Always called from within the
   * this._requestFrame() callback.
   *
   * There are three distinct cases of incoming FrameResponse to handle in this method:
   * - user panned left to incrementally load older data points to an existing query
   * - user panned right to incrementally load newer data points to an existing query
   * - a new page load, user made an entirely new query, or made a zoom-out request that
   *   includes both newer and older data points than are currently loaded.
   * The first two cases mean we can merge the existing dataframe with the incoming dataframe.
   * The third case means we need to replace the existing dataframe with the incoming dataframe.
   *
   * @param {Object} json - The parsed JSON returned from the server.
   * otherwise replace them all with the new ones.
   * @param {Boolean} tab - If true then switch to the Params tab.
   */
  private addTraces(json: FrameResponse, tab: boolean) {
    const dataframe = json.dataframe!;
    if (dataframe.traceset === null || Object.keys(dataframe.traceset).length === 0) {
      this.displayMode = 'display_query_only';
      this._render();
      return;
    }

    this.tracesRendered = true;
    this.displayMode = json.display_mode;
    this._render();

    if (this.displayMode === 'display_pivot_table') {
      this.pivotTable!.removeAttribute('disable_validation');
      this.pivotTable!.set(
        dataframe,
        this.pivotControl!.pivotRequest!,
        this._state.queries[0],
        this._state.sort
      );
      return;
    }

    // Add in the 0-trace.
    if (this._state.showZero) {
      dataframe.traceset[ZERO_NAME] = Trace(Array(dataframe.header!.length).fill(0));
    }

    const mergedDataframe = dataframe;
    this.traceKeyForSummary = '';

    this.originalTraceSet = deepCopy(mergedDataframe.traceset);

    const header = dataframe.header;
    const selectedRange = range(header![0]!.offset, header![header!.length - 1]!.offset);
    this.updateSelectedRangeWithUpdatedDataframe(selectedRange, 'commit');

    // Normalize bands to be just offsets.
    const bands: number[] = [];
    mergedDataframe.header!.forEach((h, i) => {
      if (json.skps!.indexOf(h!.offset) !== -1) {
        bands.push(i);
      }
    });

    const plot = this.plotSimple.value;
    if (plot) {
      plot.bands = bands;
    }

    // Populate the xbar if present.
    if (this._state.xbaroffset !== -1) {
      const xbaroffset = this._state.xbaroffset;
      let xbar = -1;
      mergedDataframe.header!.forEach((h, i) => {
        if (h!.offset === xbaroffset) {
          xbar = i;
        }
      });

      if (plot) {
        plot.xbar = xbar;
      }
    } else {
      if (plot) {
        plot.xbar = -1;
      }
    }
    if (this.state.use_titles) {
      this.updateTitle();
    }

    // Populate the paramset element.
    this.paramset!.paramsets = [mergedDataframe.paramset as CommonSkParamSet];
    if (tab) {
      this.detailTab!.selected = PARAMS_TAB_INDEX;
    }
    this._renderedTraces();
    if (this._state.plotSummary) {
      const header = dataframe.header!;
      this.plotSummary.value?.Select(header![0]!, header[header.length - 1]!);

      this.dfRepo.value?.extendRange(-3 * monthInSec).then(() => {
        // Already plotted, just need to update the data.
        this.updateSelectedRangeWithUpdatedDataframe(selectedRange, 'commit', false);
      });
    }
  }

  /**
   * Plot the traces that match either the current query or the current formula,
   * depending on the value of plotType.
   *
   * @param replace - If true then replace all the traces with ones
   * that match this query, otherwise add them to the current traces being
   * displayed.
   *
   * @param plotType - The type of traces being added.
   */
  add(replace: boolean, plotType: addPlotType): void {
    const q = this.query!.current_query;
    const f = this.formula!.value;
    this.addFromQueryOrFormula(replace, plotType, q, f);
  }

  /**
   * Returns the labels for the plot
   * @param dataframe The dataframe to use for generating labels
   * @returns a list of labels
   */
  private getLabels(columnHeader: (ColumnHeader | null)[]): tick[] {
    let labels: tick[] = [];
    const dates: Date[] = [];
    switch (this.state.labelMode) {
      case LabelMode.CommitPosition:
        columnHeader.forEach((header, i) => {
          labels.push({
            x: i,
            text: header!.offset.toString(),
          });
        });
        labels = fixTicksLength(labels);
        break;
      case LabelMode.Date:
        columnHeader.forEach((header) => {
          dates.push(new Date(header!.timestamp * 1000));
        });
        labels = ticks(dates);
        break;
      default:
        break;
    }

    return labels;
  }

  /**
   * Plot the traces that match either the given query or the given formula,
   * depending on the value of plotType.
   *
   * @param replace - If true then replace all the traces with ones that match
   * this query, otherwise add them to the current traces being displayed.
   *
   * @param plotType - The type of traces being added.
   */
  addFromQueryOrFormula(replace: boolean, plotType: addPlotType, q: string, f: string) {
    this.queryDialog!.close();
    this._dialogOn = false;

    if (plotType === 'query') {
      if (!q || q.trim() === '') {
        errorMessage('The query must not be empty.');
        return;
      }
    } else if (plotType === 'formula') {
      if (f.trim() === '') {
        errorMessage('The formula must not be empty.');
        return;
      }
    } else if (plotType === 'pivot') {
      if (!q || q.trim() === '') {
        errorMessage('The query must not be empty.');
        return;
      }

      const pivotMsg = validatePivotRequest(this.pivotControl!.pivotRequest!);
      if (pivotMsg !== '') {
        errorMessage(pivotMsg);
        return;
      }
    } else {
      errorMessage('Unknown plotType');
      return;
    }
    this._state.begin = this.range!.state.begin;
    this._state.end = this.range!.state.end;
    this._state.numCommits = this.range!.state.num_commits;
    this._state.requestType = this.range!.state.request_type;
    this._state.sort = '';
    if (replace || plotType === 'pivot') {
      this.removeAll(true);
    }

    this._state.pivotRequest = defaultPivotRequest();
    if (plotType === 'query') {
      if (this._state.queries.indexOf(q) === -1) {
        this._state.queries.push(q);
      }
    } else if (plotType === 'formula') {
      if (this._state.formulas.indexOf(f) === -1) {
        this._state.formulas.push(f);
      }
    } else if (plotType === 'pivot') {
      if (this._state.queries.indexOf(q) === -1) {
        this._state.queries.push(q);
      }
      this._state.pivotRequest = this.pivotControl!.pivotRequest!;
    }

    this.applyQueryDefaultsIfMissing();
    this._stateHasChanged();
    const body = this.requestFrameBodyFullFromState();
    this.requestFrame(body, (json) => {
      this.dfRepo.value
        ?.resetWithDataframeAndRequest(json.dataframe!, json.anomalymap, body)
        .then(() => {
          this.addTraces(json, true);
        });
    });
  }

  // take a query string, and update the parameters with default values if needed
  private applyDefaultsToQuery(queryString: string): string {
    const paramSet = toParamSet(queryString);
    for (const defaultParamKey in this._defaults?.default_param_selections) {
      if (!(defaultParamKey in paramSet)) {
        paramSet[defaultParamKey] = this._defaults!.default_param_selections![defaultParamKey]!;
      }
    }

    return fromParamSet(paramSet);
  }

  // applyQueryDefaultsIfMissing updates the fields in the state object to
  // specify the default values provided for the instance if they haven't
  // been specified by the user explicitly.
  private applyQueryDefaultsIfMissing() {
    const updatedQueries: string[] = [];

    // Check the current query to see if the default params have been specified.
    // If not, add them with the default value in the instance config.
    this._state.queries.forEach((query) => {
      updatedQueries.push(this.applyDefaultsToQuery(query));
    });

    this._state.queries = updatedQueries;

    // Check if the user has specified the params provided in the default url config.
    // If not, add them to the state object
    for (const urlKey in this._defaults?.default_url_values) {
      const stringToBool = function (str: string): boolean {
        return str.toLowerCase() === 'true';
      };
      if (this._userSpecifiedCustomizationParams.has(urlKey) === false) {
        const paramValue = stringToBool(this._defaults!.default_url_values![urlKey]);
        switch (urlKey) {
          case 'summary':
            this._state.summary = paramValue;
            break;
          case 'plotSummary':
            this._state.plotSummary = paramValue;
            break;
          case 'disableMaterial':
            this._state.disableMaterial = paramValue;
            break;
          case 'showZero':
            this._state.showZero = paramValue;
            break;
          case 'useTestPicker':
            this.useTestPicker = paramValue;
            break;
          case 'use_test_picker_query':
            this._state.use_test_picker_query = paramValue;
            this.openQueryByDefault = !paramValue;
            break;
          case 'enable_chart_tooltip':
            this._state.enable_chart_tooltip = paramValue;
            break;
          case 'use_titles':
            this._state.use_titles = paramValue;
            break;
          case 'show_google_plot':
            this._state.show_google_plot = paramValue;
            break;
          default:
            break;
        }
      }
    }
  }

  /**
   * Removes all traces.
   *
   * @param skipHistory  If true then don't update the URL. Used
   * in calls like _normalize() where this is just an intermediate state we
   * don't want in history.
   */
  private removeAll(skipHistory: boolean) {
    this._state.formulas = [];
    this._state.queries = [];
    this._state.keys = '';
    this.plotSimple.value?.removeAll();
    this._dataframe.header = [];
    this._dataframe.traceset = TraceSet({});
    this.paramset!.paramsets = [];
    this.commitTime!.textContent = '';
    this.detailTab!.selected = PARAMS_TAB_INDEX;
    this.displayMode = 'display_query_only';
    this.tracesRendered = false;
    this.graphTitle!.set(null, 0);
    this.traceDetailsCopy!.style.display = 'none';
    this.traceDetails!.textContent = '';
    this.tooltipSelected = false;
    this.disableTooltip();
    this.dispatchEvent(new CustomEvent('remove-all', { bubbles: true }));

    // force unset autorefresh so that it doesn't re-appear when we remove all the chart.
    // the removeAll button from "remove all" or "X" will call invoke removeAll()
    // with skipHistory = false, so state should be updated.
    this._state.autoRefresh = false;
    this.autoRefreshChanged();

    this._render();
    if (!skipHistory) {
      this.clearSelectedState();
      this._stateHasChanged();
    }
  }

  /**
   * When Remove Highlighted or Highlighted Only are pressed then create a
   * shortcut for just the traces that are displayed.
   *
   * Note that removing a trace doesn't work if the trace came from a
   * formula that returns multiple traces. This is a known issue that
   * isn't currently worth solving.
   *
   * Returns the Promise that's creating the shortcut, or undefined if
   * there isn't a shortcut to create.
   */
  private reShortCut(keys: string[]): Promise<void> | undefined {
    if (keys.length === 0) {
      this._state.keys = '';
      this._state.queries = [];
      return undefined;
    }
    const state = {
      keys,
    };
    return fetch('/_/keys/', {
      method: 'POST',
      body: JSON.stringify(state),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((json) => {
        this._state.keys = json.id;
        this._state.queries = [];
        this.clearSelectedState();
        this._stateHasChanged();
        this._render();
      })
      .catch(errorMessage);
  }

  /**
   * Create a shortcut for all of the traces currently being displayed.
   *
   * Returns the Promise that's creating the shortcut, or undefined if
   * there isn't a shortcut to create.
   */
  private shortcutAll(): Promise<void> | undefined {
    const toShortcut: string[] = [];

    Object.keys(this._dataframe.traceset).forEach((key) => {
      if (key[0] === ',') {
        toShortcut.push(key);
      }
    });

    return this.reShortCut(toShortcut);
  }

  private async applyFuncToTraces(funcName: string) {
    // Move all non-formula traces into a shortcut.
    await this.shortcutAll();

    // Apply the func to the shortcut traces.
    let updatedFormulas: string[] = [];
    if (this._state.keys !== '') {
      updatedFormulas.push(`${funcName}(shortcut("${this._state.keys}"))`);
    }

    // Also apply the func to any existing formulas.
    updatedFormulas = updatedFormulas.concat(this._state.formulas.map((f) => `${funcName}(${f})`));

    this.removeAll(true);
    this._state.formulas = updatedFormulas;
    this._stateHasChanged();

    await this.requestFrame(this.requestFrameBodyFullFromState(), (json) => {
      this.addTraces(json, false);
    });
  }

  public createGraphConfigs(traceSet: TraceSet, attribute?: string): GraphConfig[] {
    const graphConfigs = [] as GraphConfig[];
    Object.keys(traceSet).forEach((key) => {
      const conf: GraphConfig = {
        keys: '',
        formulas: [],
        queries: [],
      };
      if (key[0] === ',') {
        conf.queries = [new URLSearchParams(fromKey(key, attribute)).toString()];
      } else {
        if (key.startsWith('special')) {
          return;
        }
        conf.formulas = [key];
      }
      graphConfigs.push(conf);
    });

    return graphConfigs;
  }

  public toggleGoogleChart() {
    this.state.show_google_plot = !this.state.show_google_plot;
    this._render();
  }

  // TODO(b/377772220): When splitting a chart with multiple traces,
  // this function will perform the split operation on every trace.
  // If a trace is selected, the chart should only split on that trace.
  // If no trace is selected, then default to splitting on every trace.
  public async splitByAttribute({
    detail,
  }: CustomEvent<SplitChartSelectionEventDetails>): Promise<void> {
    const graphConfigs: GraphConfig[] = this.createGraphConfigs(
      this._dataframe.traceset,
      detail.attribute
    );
    const newShortcut = await updateShortcut(graphConfigs);

    if (newShortcut === '') {
      return;
    }

    window.open(
      `/m/?begin=${this._state.begin}&end=${this._state.end}` +
        `&pageSize=${chartsPerPage}&shortcut=${newShortcut}` +
        `&totalGraphs=${graphConfigs.length}` +
        (this.state.show_google_plot ? `&show_google_plot=true` : ``),
      '_self'
    );
  }

  public async viewMultiGraph(): Promise<void> {
    const graphConfigs: GraphConfig[] = this.createGraphConfigs(this._dataframe.traceset);
    const newShortcut = await updateShortcut(graphConfigs);

    if (newShortcut === '') {
      return;
    }

    window.open(
      `/m/?begin=${this._state.begin}&end=${this._state.end}` +
        `&pageSize=${chartsPerPage}&shortcut=${newShortcut}` +
        `&totalGraphs=${graphConfigs.length}` +
        (this.state.show_google_plot ? `&show_google_plot=true` : ``),
      '_self'
    );
  }

  private queryHighlighted() {
    const detail = {
      key: this.plotSimple.value!.highlight[0],
    };
    this.dispatchEvent(new CustomEvent('populate-query', { detail: detail, bubbles: true }));
  }

  private removeHighlighted() {
    const ids = this.plotSimple.value!.highlight;
    this.removeKeys(ids, true);
  }

  private removeKeys(keysToRemove: string[], updateShortcut: boolean) {
    const toShortcut: string[] = [];
    Object.keys(this._dataframe.traceset).forEach((key) => {
      if (keysToRemove.indexOf(key) !== -1) {
        // Detect if it is a formula being removed.
        if (this._state.formulas.indexOf(key) !== -1) {
          this._state.formulas.splice(this._state.formulas.indexOf(key), 1);
        }
        return;
      }
      if (updateShortcut) {
        if (key[0] === ',') {
          toShortcut.push(key);
        }
      }
    });

    // Remove the traces from the traceset so they don't reappear.
    keysToRemove.forEach((key) => {
      if (this._dataframe.traceset[key] !== undefined) {
        delete this._dataframe.traceset[key];
      }
    });

    const plot = this.plotSimple.value;
    if (plot) {
      plot.deleteLines(keysToRemove);
      plot.highlight = [];
    }
    if (!this.hasData()) {
      this.displayMode = 'display_query_only';
      this._render();
    }
    if (updateShortcut) {
      this.reShortCut(toShortcut);
    }
  }

  private highlightedOnly() {
    const plot = this.plotSimple.value!;
    const ids = plot.highlight;
    const toremove: string[] = [];
    const toShortcut: string[] = [];

    let totalNonSpecialKeys = 0;
    Object.keys(this._dataframe.traceset).forEach((key) => {
      if (!key.startsWith('special')) {
        totalNonSpecialKeys++;
      }

      if (ids.indexOf(key) === -1 && !key.startsWith('special')) {
        // Detect if it is a formula being removed.
        if (this._state.formulas.indexOf(key) !== -1) {
          this._state.formulas.splice(this._state.formulas.indexOf(key), 1);
        } else {
          toremove.push(key);
        }
        return;
      }
      if (key[0] === ',') {
        toShortcut.push(key);
      }
    });

    if (toremove.length === totalNonSpecialKeys) {
      errorMessage("At least one trace much be selected for 'Highlighted Only' to work.");
      return;
    }

    // Remove the traces from the traceset so they don't reappear.
    toremove.forEach((key) => {
      delete this._dataframe.traceset[key];
    });

    plot.deleteLines(toremove);
    plot.highlight = [];
    if (!this.hasData()) {
      this.displayMode = 'display_query_only';
      this._render();
    }
    this.reShortCut(toShortcut);
  }

  /**
   * If there are tracesets in the Dataframe and IncludeParams config has been specified, we
   * update the title using only the common parameters of all present traces.
   *
   * If there are less than 3 common parameters, we use the default title.
   */
  private updateTitle() {
    const traceset = this.dfRepo.value?.dataframe.traceset;
    if (traceset === null || traceset === undefined) {
      return;
    }

    // If the params are not included in the json config key "include_params",
    // we pull the paramset from the dataframe response.
    // https://skia.googlesource.com/buildbot/+/refs/heads/main/perf/configs/v8-perf.json
    let params = this._defaults?.include_params;
    if (params === null || params === undefined) {
      const paramset = this.dfRepo.value?.dataframe.paramset;
      if (paramset === null || paramset === undefined) {
        return;
      }
      params = Object.keys(paramset).sort();
    }
    const numTraces = Object.keys(traceset).length;
    const titleEntries = new Map();

    // For each param, we found out the unique values in each trace. If there's only 1 unique value,
    // that means that they all share a value in common and we can add this to the title.
    params!.forEach((param) => {
      const uniqueValues = new Set(Object.keys(traceset).map((traceId) => fromKey(traceId)[param]));
      if (uniqueValues.size === 1) {
        const value = uniqueValues.values().next().value;
        if (value) {
          titleEntries.set(param, value);
        }
      }
    });

    if (titleEntries.size >= 3) {
      this.graphTitle!.set(titleEntries, numTraces);
    } else {
      this.graphTitle!.set(new Map(), numTraces);
    }
  }

  /** Common catch function for _requestFrame and _checkFrameRequestStatus. */
  private catch(msg: any) {
    this._requestId = '';
    if (msg) {
      errorMessage(msg);
    }
    this.percent!.textContent = '';
    this.spinning = false;
  }

  /** @prop spinning - True if we are waiting to retrieve data from
   * the server.
   */
  set spinning(b: boolean) {
    this._spinning = b;
    if (b) {
      this.displayMode = 'display_spinner';
    }
    this._render();
  }

  get spinning(): boolean {
    return this._spinning;
  }

  /**
   * Requests a new dataframe, where body is a serialized FrameRequest:
   *
   * {
   *    begin:    1448325780,
   *    end:      1476706336,
   *    formulas: ["ave(filter("name=desk_nytimes.skp&sub_result=min_ms"))"],
   *    queries:  [
   *        "name=AndroidCodec_01_original.jpg_SampleSize8",
   *        "name=AndroidCodec_1.bmp_SampleSize8"],
   *    tz:       "America/New_York"
   * };
   *
   * The 'cb' callback function will be called with the decoded JSON body
   * of the response once it's available.
   */
  private async requestFrame(body: FrameRequest, cb: RequestFrameCallback) {
    if (this._requestId !== '') {
      errorMessage('There is a pending query already running.');
      return;
    }

    this._requestId = 'About to make request';
    this.spinning = true;
    try {
      await this.sendFrameRequest(body, cb);
    } catch (msg) {
      this.catch(msg);
    } finally {
      this.spinning = false;
      this._requestId = '';
    }
  }

  private async sendFrameRequest(body: FrameRequest, cb: RequestFrameCallback) {
    body.tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
    const finishedProg = await startRequest(
      '/_/frame/start',
      body,
      200,
      this.spinner!,
      (prog: progress.SerializedProgress) => {
        this.percent!.textContent = messagesToPreString(prog.messages);
      }
    );
    if (finishedProg.status !== 'Finished') {
      throw new Error(messagesToErrorString(finishedProg.messages));
    }
    const msg = messageByName(finishedProg.messages, 'Message');
    if (msg) {
      errorMessage(msg);
    }
    cb(finishedProg.results as FrameResponse);
  }

  // Download all the displayed data as a CSV file.
  private csv() {
    if (this._csvBlobURL) {
      URL.revokeObjectURL(this._csvBlobURL);
      this._csvBlobURL = '';
    }
    const csvBody = dataFrameToCSV(this._dataframe);
    const blob = new Blob([csvBody], { type: 'text/csv' });
    const url = URL.createObjectURL(blob);
    this.csvDownload!.href = url;
    this._csvBlobURL = url;
    this.csvDownload!.click();
  }

  private isZero(n: number) {
    return n === 0;
  }

  get state(): State {
    return this._state;
  }

  set state(state: State) {
    state = this.rationalizeTimeRange(state);
    this._state = state;
    this.range!.state = {
      begin: this._state.begin,
      end: this._state.end,
      num_commits: this._state.numCommits,
      request_type: this._state.requestType,
    };

    this._render();

    if (this.plotSimple.value) {
      this.plotSimple.value!.dots = this._state.dots;
    }
    // If there is at least one query, the use the last one to repopulate the
    // query-sk dialog.
    const numQueries = this._state.queries.length;
    if (numQueries >= 1) {
      this.query!.paramset = toParamSet(this._state.queries[numQueries - 1]);
      this.query!.current_query = this._state.queries[numQueries - 1];
      this.summary!.paramsets = [toParamSet(this._state.queries[numQueries - 1])];
    }

    this.applyQueryDefaultsIfMissing();
    if (
      numQueries === 0 &&
      this._state.keys === '' &&
      this._state.formulas.length === 0 &&
      this.openQueryByDefault
    ) {
      this.openQuery();
    }

    this.zeroChanged();
    this.autoRefreshChanged();
    this.rangeChangeImpl();
  }

  get openQueryByDefault(): boolean {
    return this.hasAttribute('open-query-by-default');
  }

  set openQueryByDefault(val: boolean) {
    if (val) {
      this.setAttribute('open-query-by-default', '');
    } else {
      this.removeAttribute('open-query-by-default');
    }
  }

  get navOpen(): boolean {
    return this.hasAttribute('nav-open');
  }

  set navOpen(val: boolean) {
    if (val) {
      this.setAttribute('nav-open', '');
    } else {
      this.removeAttribute('nav-open');
    }
  }

  private toggleDetails() {
    this.navOpen = !this.navOpen;
    this._render();
  }

  getTraceset(): { [key: string]: number[] } {
    return this._dataframe.traceset;
  }

  set defaults(val: QueryConfig | null) {
    this._defaults = val;
  }
}

define('explore-simple-sk', ExploreSimpleSk);
