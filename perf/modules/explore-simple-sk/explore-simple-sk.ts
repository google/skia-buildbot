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
import { AnomalyData } from '../common/anomaly-data';
import { toParamSet, fromParamSet } from '../../../infra-sk/modules/query';
import { TabsSk } from '../../../elements-sk/modules/tabs-sk/tabs-sk';
import { ToastSk } from '../../../elements-sk/modules/toast-sk/toast-sk';
import { ParamSet as CommonSkParamSet } from '../../../infra-sk/modules/query';
import { SpinnerSk } from '../../../elements-sk/modules/spinner-sk/spinner-sk';
import { errorMessage } from '../errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { CheckOrRadio } from '../../../elements-sk/modules/checkbox-sk/checkbox-sk';

import '@material/web/button/outlined-button.js';
import '@material/web/icon/icon.js';
import '@material/web/iconbutton/outlined-icon-button.js';
import '@material/web/switch/switch.js';
import '@material/web/dialog/dialog.js';
import '../keyboard-shortcuts-help-sk/keyboard-shortcuts-help-sk';
import { KeyboardShortcutsHelpSk } from '../keyboard-shortcuts-help-sk/keyboard-shortcuts-help-sk';

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

import '../commit-detail-panel-sk';
import '../commit-range-sk';
import '../domain-picker-sk';
import '../json-source-sk';
import '../picker-field-sk';
import '../pivot-query-sk';
import '../pivot-table-sk';
import '../plot-google-chart-sk';
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
  Anomaly,
  DataFrame,
  RequestType,
  ParamSet,
  FrameRequest,
  FrameResponse,
  ShiftRequest,
  ShiftResponse,
  pivot,
  FrameResponseDisplayMode,
  ColumnHeader,
  QueryConfig,
  TraceSet,
  ReadOnlyParamSet,
  CommitNumberAnomalyMap,
  AnomalyMap,
  TraceMetadata,
  TraceCommitLink,
} from '../json';
import {
  ParamSetSk,
  ParamSetSkCheckboxClickEventDetail,
  ParamSetSkClickEventDetail,
  ParamSetSkKeyCheckboxClickEventDetail,
  ParamSetSkRemoveClickEventDetail,
} from '../../../infra-sk/modules/paramset-sk/paramset-sk';
import {
  QuerySk,
  QuerySkQueryChangeEventDetail,
} from '../../../infra-sk/modules/query-sk/query-sk';
import { QueryCountSk } from '../query-count-sk/query-count-sk';
import { DomainPickerSk } from '../domain-picker-sk/domain-picker-sk';
import { validatePivotRequest } from '../pivotutil';
import { PivotQueryChangedEventDetail, PivotQuerySk } from '../pivot-query-sk/pivot-query-sk';
import { PivotTableSk, PivotTableSkChangeEventDetail } from '../pivot-table-sk/pivot-table-sk';
import { addParamsToParamSet, fromKey, validKey } from '../paramtools';
import { CommitRangeSk } from '../commit-range-sk/commit-range-sk';
import { MISSING_DATA_SENTINEL, MISSING_VALUE_SENTINEL } from '../const/const';
import { LoggedIn } from '../../../infra-sk/modules/alogin-sk/alogin-sk';
import { Status as LoginStatus } from '../../../infra-sk/modules/json';
import { findSubDataframe, range } from '../dataframe';
import { TraceFormatter, GetTraceFormatter } from '../trace-details-formatter/traceformatter';
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
import { GraphTitleSk } from '../graph-title-sk/graph-title-sk';
import { NewBugDialogSk } from '../new-bug-dialog-sk/new-bug-dialog-sk';
import {
  PlotGoogleChartSk,
  PlotSelectionEventDetails,
  PlotShowTooltipEventDetails,
} from '../plot-google-chart-sk/plot-google-chart-sk';
import { DataFrameRepository, DataTable, UserIssueMap } from '../dataframe/dataframe_context';
import { ExistingBugDialogSk } from '../existing-bug-dialog-sk/existing-bug-dialog-sk';
import { generateSubDataframe, join as joinDataframes } from '../dataframe/index';
import { SplitChartSelectionEventDetails } from '../split-chart-menu-sk/split-chart-menu-sk';
import { getLegend, getTitle, isSingleTrace, legendFormatter } from '../dataframe/traceset';
import { BisectPreloadParams } from '../bisect-dialog-sk/bisect-dialog-sk';
import { TryJobPreloadParams } from '../pinpoint-try-job-dialog-sk/pinpoint-try-job-dialog-sk';
import { FavoritesDialogSk } from '../favorites-dialog-sk/favorites-dialog-sk';
import { CommitLinks } from '../point-links-sk/point-links-sk';
import { handleKeyboardShortcut, KeyboardShortcutHandler } from '../common/keyboard-shortcuts';
import { GraphConfig, updateShortcut } from '../common/graph-config';
import { DataService, DataServiceError } from '../data-service';

const DOMAIN_DATE = 'date';
const DOMAIN_COMMIT = 'commit';

/** The type of trace we are adding to a plot. */
type addPlotType = 'query' | 'formula' | 'pivot' | 'json';

// The trace id of the zero line, a trace of all zeros.
const ZERO_NAME = 'special_zero';

// A list of all special trace names.
const SPECIAL_TRACE_NAMES = [ZERO_NAME];

// Amount of datapoints that is expected to begin affecting performance.
const DATAPOINT_THRESHOLD = 10000;

// The default query range in seconds.
export const DEFAULT_RANGE_S = 24 * 60 * 60; // 2 days in seconds.

// The index of the params tab.
const PARAMS_TAB_INDEX = 0;

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

// Minimum amount of points to display on a graph.
const MIN_POINTS = 100;

const monthInSec = 30 * 24 * 60 * 60;

// max number of charts to show on a page
const chartsPerPage = 11;

const HOVER_DEBOUNCE_MS = 100;

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
  tableRow?: number; // The row in the table that this point corresponds to.
  tableCol?: number; // The column in the table that this point corresponds to.
}

export enum LabelMode {
  Date = 0,
  CommitPosition = 1,
}

interface TraceEventDetails {
  x: number;
  y: number;
  // DOM position of x in Pixels
  xPos?: number;
  // DOM position of y in Pixels
  yPos?: number;
  // The trace id.
  name: string;
}

/** Returns true if the PointSelected is valid. */
export const isValidSelection = (p: PointSelected): boolean => p.name !== '';

/** Converts a PointSelected into a CustomEvent<TraceEventDetails>,
 * so that it can be passed into traceSelected().
 *
 * Note that we need the _dataframe.header to convert the commit back into an
 * offset. Also note that might fail, in which case the 'x' value will be set to
 * -1.
 */
export const selectionToEvent = (
  p: PointSelected,
  header: (ColumnHeader | null)[] | null
): CustomEvent<TraceEventDetails> => {
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
  return new CustomEvent<TraceEventDetails>('', {
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
  tableRow: -1,
  tableCol: -1,
});

/**
 * Creates a shortcut ID for the given Graph Configs.
 *
 */

// State is reflected to the URL via stateReflector.
export class State {
  begin: number = -1;

  end: number = -1;

  formulas: string[] = [];

  queries: string[] = [];

  keys: string = ''; // The id of the shortcut to a list of trace keys.

  xbaroffset: number = -1; // The offset of the commit in the repo.

  showZero: boolean = false;

  dots: boolean = true;

  autoRefresh: boolean = false;

  show_google_plot: boolean = false;

  numCommits: number = 250;

  requestType: RequestType = 1; // 0 to use begin/end, 1 to use numCommits.

  pivotRequest: pivot.Request = defaultPivotRequest();

  sort: string = ''; // Pivot table sort order.

  summary: boolean = false; // Whether to show the zoom/summary area.

  selected: PointSelected = defaultPointSelected(); // The point on a trace that was clicked on.

  domain: string = DOMAIN_COMMIT; // The domain of the x-axis, either commit or date. Default commit

  // Deprecated.
  labelMode: LabelMode = LabelMode.Date; // The label mode for the x-axis, date or commit.

  incremental: boolean = false; // Enables a data fetching optimization.

  disable_filter_parent_traces: boolean = false;

  plotSummary: boolean = false;

  disableMaterial?: boolean = false;

  highlight_anomalies: string[] = [];

  enable_chart_tooltip: boolean = false;

  show_remove_all: boolean = true;

  use_titles: boolean = false;

  useTestPicker: boolean = false;

  use_test_picker_query: boolean = false;

  // boolean indicate for enabling favorites action. set by explore-sk.
  // requires user to be logged in so that the favorites can be saved for the user.
  enable_favorites: boolean = false;

  // boolean indicating if param set details shown be shown below a chart
  hide_paramset?: boolean = false;

  // boolean indicating if zoom should be horizontal or vertical
  horizontal_zoom: boolean = false;

  graph_index: number = 0; // The index of the graph in the list of graphs.

  // boolean indicating if the element should disable querying of data from backend.
  doNotQueryData: boolean = false;

  // boolean indicating if the chart should space x-axis points evenly, treating them as categories.
  evenXAxisSpacing: boolean = false;
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
      newOffsets: [
        clampToNonNegative(offsets[0] - offsetDelta) as CommitNumber,
        (offsets[1] - offsetDelta) as CommitNumber,
      ],
    };
  }
  if (exceedsRightEdge) {
    // shift right
    return {
      rangeChange: true,
      newOffsets: [
        (offsets[0] + offsetDelta) as CommitNumber,
        (offsets[1] + offsetDelta) as CommitNumber,
      ],
    };
  }
  return {
    rangeChange: false,
  };
}

export class ExploreSimpleSk extends ElementSk implements KeyboardShortcutHandler {
  private dataService = DataService.getInstance();

  private _dataframe: DataFrame = {
    traceset: TraceSet({}),
    header: [],
    paramset: ReadOnlyParamSet({}),
    skip: 0,
    traceMetadata: [],
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

  private testPath: string = '';

  private startCommit: string = '';

  private endCommit: string = '';

  private story: string = '';

  private bugId: string = '';

  private jobUrl: string = '';

  private jobId: string = '';

  user: string = '';

  private _defaults: QueryConfig | null = null;

  private _initialized: boolean = false;

  private detailTab: TabsSk | null = null;

  private formula: HTMLTextAreaElement | null = null;

  private json: HTMLTextAreaElement | null = null;

  private logEntry: HTMLPreElement | null = null;

  private paramset: ParamSetSk | null = null;

  private percent: HTMLSpanElement | null = null;

  private googleChartPlot = createRef<PlotGoogleChartSk>();

  private plotSummary = createRef<PlotSummarySk>();

  private query: QuerySk | null = null;

  private queryCount: QueryCountSk | null = null;

  private range: DomainPickerSk | null = null;

  private spinner: SpinnerSk | null = null;

  private summary: ParamSetSk | null = null;

  private commitTime: HTMLSpanElement | null = null;

  private pivotControl: PivotQuerySk | null = null;

  private pivotTable: PivotTableSk | null = null;

  private pivotDisplayButton: HTMLButtonElement | null = null;

  private queryDialog: HTMLDialogElement | null = null;

  private helpDialog: HTMLDialogElement | null = null;

  // TODO(b/372694234): consolidate the pinpoint and triage toasts.
  private pinpointJobToast: ToastSk | null = null;

  private triageResultToast: ToastSk | null = null;

  private closePinpointToastButton: HTMLButtonElement | null = null;

  private closeTriageToastButton: HTMLButtonElement | null = null;

  private collapseButton: HTMLButtonElement | null = null;

  private traceFormatter: TraceFormatter | null = null;

  private testPickerUrl: string = '#';

  private commitLinks: (CommitLinks | null)[] = [];

  chartTooltip: ChartTooltipSk | null = null;

  useTestPicker: boolean = false;

  is_chart_split: boolean = false;

  is_anomaly_table: boolean = false;

  enableRemoveButton: boolean = true;

  disablePointLinks: boolean = false;

  showHeader: boolean = true;

  showTabs: boolean = true;

  private xAxisSwitch = false;

  private zoomDirectionSwitch = false;

  private summaryOptionsField: Ref<PickerFieldSk> = createRef();

  // Map with displayed summary bar option as key and trace key
  // as value. For example, {'...k1=v1,k2=v2' => ',ck1=>cv1,ck2=>cv2,k1=v1,k2=v2,'}
  private summaryOptionTraceMap: Map<string, string> = new Map();

  // Map with displayed trace key as key and summary bar option
  // as value. For example, {',ck1=>cv1,ck2=>cv2,k1=v1,k2=v2,' => '...k1=v1,k2=v2'}
  private traceIdSummaryOptionMap: Map<string, string> = new Map();

  // tooltipSelected tracks whether someone has turned on the tooltip
  // by selecting a data point. A new tooltip will not be created on
  // mouseover unless the current selected tooltip is closed.
  // true - the tooltip is selected
  // false - the tooltip is not selected but could be on via mouse hover
  private tooltipSelected = false;

  private graphTitle: GraphTitleSk | null = null;

  tracesRendered = false;

  private selectedAnomaly: Anomaly | null = null;

  private hoverTimeout: number = -1;

  // material UI
  private settingsDialog: MdDialog | null = null;

  private dfRepo = createRef<DataFrameRepository>();

  public isReportPage: boolean = false;

  private static nextUniqueId = 0;

  private readonly uniqueId = `${ExploreSimpleSk.nextUniqueId++}`;

  public get dataLoading(): boolean {
    return this.dfRepo.value?.loading ?? false;
  }

  public set dataLoading(value: boolean) {
    if (this.dfRepo.value) {
      this.dfRepo.value.loading = value;
      if (!value) {
        // Dispatch an event to notify that data is loaded for the previous range.
        this.dispatchEvent(
          new CustomEvent('data-loaded', {
            bubbles: true,
          })
        );
      }
    }
  }

  public get requestComplete(): Promise<number> {
    return this.dfRepo.value?.requestComplete ?? Promise.resolve(0);
  }

  constructor(useTestPicker?: boolean) {
    super(ExploreSimpleSk.template);
    this.traceFormatter = GetTraceFormatter();
    this.useTestPicker = useTestPicker ?? false;
  }

  // TODO(b/380215495): The current implementation of splitting the chart by
  // an attribute is buggy and not very intuitive. The split-chart-menu module
  // has therefore been removed until the bugs are fixed.
  private static template = (ele: ExploreSimpleSk) => html`
  <dataframe-repository-sk ${ref(ele.dfRepo)}>
  <div id=explore class=${ele.displayMode}>
    <div id=buttons style="display: none">
      <button
        id=open_query_dialog
        ?hidden=${ele.useTestPicker}
        @click=${ele.openQuery}>
        Query
      </button>
    </div>

    <div id=chartHeader
      ?hidden=${!ele.showHeader}
      class="hide_on_query_only hide_on_pivot_table hide_on_spinner">
      <graph-title-sk id=graphTitle style="flex-grow:  1;"></graph-title-sk>
      ${
        ele.isReportPage
          ? html`
              <a href="${ele.testPickerUrl}">
                <md-icon-button title="Open in Multigraph">
                  <md-icon id="icon">north_west</md-icon>
                </md-icon-button>
              </a>
            `
          : html`
              <md-icon-button
                title="Load Test Picker with current Query"
                ?disabled=${ele.is_chart_split && !ele.useTestPicker}
                @click=${ele.loadDataFromExistingChart}>
                <md-icon id="icon">north_west</md-icon>
              </md-icon-button>
            `
      }
      <md-icon-button
        title="Show Zero on Axis"
        @click=${() => {
          ele.showZero();
        }}>
        ${
          ele._state.showZero
            ? html`<md-icon id="icon">radio_button_unchecked</md-icon>`
            : html`<md-icon id="icon">hide_source</md-icon>`
        }
      </md-icon-button>
      <favorites-dialog-sk id="fav-dialog"></favorites-dialog-sk>
      <md-icon-button
        title="Add Chart to Favorites"
        ?disabled=${!ele._state!.enable_favorites}
        @click=${() => {
          ele.openAddFavoriteDialog();
        }}>
        <md-icon id="icon">favorite</md-icon>
      </md-icon-button>
      <md-icon-button
      id="showSettingsDialog"
        title="Show Settings Dialog"
        @click=${ele.showSettingsDialog}>
        <md-icon id="icon">settings</md-icon>
      </md-icon-button>
      <md-icon-button
        id="removeAll"
        ?disabled=${!ele.enableRemoveButton}
        @click=${() => ele.closeExplore()}
        title='Remove all the traces.'>
        <close-icon-sk></close-icon-sk>
      </md-icon-button>
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
                  ?selected="${ele.xAxisSwitch}"
                  @change=${(e: InputEvent) => ele.switchXAxis(e.target as MdSwitch)}></md-switch>
                X-Axis as Commit Date
              </label>
            </li>
            <li>
              <label>
                <md-switch
                  form="form"
                  id="zoom-direction-switch"
                  ?selected="${ele.zoomDirectionSwitch}"
                  @change=${(e: InputEvent) => ele.switchZoom(e.target as MdSwitch)}></md-switch>
                Switch Zoom Direction
              </label>
            </li>
            <li>
              <label>
                <md-switch
                  form="form"
                  id="even-x-axis-spacing-switch"
                  ?selected="${ele._state.evenXAxisSpacing}"
                  @change=${(e: InputEvent) =>
                    ele.switchEvenXAxisSpacing(e.target as MdSwitch)}></md-switch>
                Even X-Axis Spacing
              </label>
            </li>
          </ul>
        </div>
      </md-dialog>
    </div>

    <div id=spin-overlay @mouseleave=${ele.mouseLeave}>
    <div class="chart-container">
        ${html`<plot-google-chart-sk
          .domain=${ele._state.domain}
          ${ref(ele.googleChartPlot)}
          .highlightAnomalies=${ele._state.highlight_anomalies}
          .useDiscreteAxis=${ele._state.evenXAxisSpacing}
          @plot-data-select=${ele.onChartSelect}
          @plot-data-mouseover=${ele.onChartOver}
          @plot-data-mousedown=${ele.onChartMouseDown}
          @selection-changing=${ele.OnSelectionRange}
          @selection-changed=${ele.OnSelectionRange}>
          <md-icon slot="untriage">help</md-icon>
          <md-icon slot="regression">report</md-icon>
          <md-icon slot="improvement">check_circle</md-icon>
          <md-icon slot="ignored">report_off</md-icon>
          <md-icon slot="issue">chat_bubble</md-icon>
          <md-text slot="xbar">|</md-text>
        </plot-google-chart-sk>`}
      <chart-tooltip-sk></chart-tooltip-sk>
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
        <button>JSON</button>
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
              <textarea id=formula-${ele.uniqueId} rows=3 cols=80></textarea>
            </label>
            <div>
              <button @click=${() => ele.add(true, 'formula')} class=action>Plot</button>
              <button @click=${() => ele.add(false, 'formula')}>Add to Plot</button>
              <button @click=${ele.onOpenHelp} title="Keyboard Shortcuts">Shortcuts</button>
              <a href=/help/ target=_blank title="Documentation">
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
              id=pivot-display-button-${ele.uniqueId}
              @click=${() => ele.add(true, 'pivot')}
              class=action
              .disabled=${validatePivotRequest(ele._state.pivotRequest) !== ''}
            >Display</button>
          </div>
        </div>
        <div>
          <div class=formulas>
            <label>
              Enter a JSON:
              <textarea id=json-${ele.uniqueId} rows=10 cols=80></textarea>
            </label>
            <div>
              <button @click=${() => ele.add(true, 'json')} class=action>Plot</button>
              <button @click=${() => ele.add(false, 'json')}>Add to Plot</button>
              <a href=/help/ target=_blank>
                <help-icon-sk></help-icon-sk>
              </a>
            </div>
          </div>
        </div>
      </tabs-panel-sk>
      <div class=footer>
        <button @click=${ele.closeQueryDialog} id='close_query_dialog'>Close</button>
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

    <keyboard-shortcuts-help-sk .handler=${ele}></keyboard-shortcuts-help-sk>

    ${
      ele.state.hide_paramset
        ? ''
        : html`
            <div id="tabs" class="hide_on_query_only hide_on_spinner hide_on_pivot_table">
              <button
                class="collapser"
                id="collapseButton"
                @click=${(_e: Event) => ele.toggleDetails()}>
                ${ele.navOpen
                  ? html`<expand-less-icon-sk></expand-less-icon-sk>`
                  : html`<expand-more-icon-sk></expand-more-icon-sk>`}
              </button>
              <collapse-sk id="collapseDetails" .closed=${!ele.navOpen}>
                <tabs-sk id="detailTab">
                  <button>Params</button>
                </tabs-sk>
                <tabs-panel-sk>
                  <div>
                    <p><b>Time</b>: <span title="Commit Time" id="commit_time"></span></p>

                    <paramset-sk
                      id="paramset"
                      clickable_values
                      checkbox_values
                      @paramset-key-value-click=${(e: CustomEvent<ParamSetSkClickEventDetail>) => {
                        ele.paramsetKeyValueClick(e);
                      }}
                      @paramset-checkbox-click=${ele.paramsetCheckboxClick}
                      @paramset-key-checkbox-click=${ele.paramsetKeyCheckboxClick}>
                    </paramset-sk>
                  </div>
                </tabs-panel-sk>
              </collapse-sk>
            </div>
          `
    }
  </div>
  </dataframe-repository-sk>
  <toast-sk id="pinpoint-job-toast" duration=10000>
    Pinpoint bisection started: <a href=${ele.jobUrl} target=_blank>${ele.jobId}</a>.
    <button id="hide-pinpoint-toast" class="action">Close</button>
  </toast-sk>
  <toast-sk id="triage-result-toast" duration=5000>
    <span id="triage-result-text"></span><a id="triage-result-link"></a>
    <button id="hide-triage-toast" class="action">Close</button>
  </toast-sk>
  `;

  private plotSummaryTemplate() {
    return html` <plot-summary-sk
      .domain=${this._state.domain}
      ${ref(this.plotSummary)}
      @summary_selected=${this.summarySelected}
      selectionType=${!this._state.disableMaterial ? 'material' : 'canvas'}
      ?hasControl=${!this._state.disableMaterial}
      class="hide_on_pivot_table hide_on_query_only hide_on_spinner">
    </plot-summary-sk>`;
  }

  private openAddFavoriteDialog = async () => {
    const d = $$<FavoritesDialogSk>('#fav-dialog', this) as FavoritesDialogSk;
    await d!.open();
  };

  private showZero = () => {
    this._state.showZero = !this._state.showZero;
    this.googleChartPlot.value!.showZero = this._state.showZero;
    this.render();
  };

  private loadDataFromExistingChart = async () => {
    const traceset = this.getTraceset();
    const paramSet = ParamSet({});
    if (traceset) {
      Object.keys(traceset).forEach((key) => {
        if (validKey(key)) {
          addParamsToParamSet(paramSet, fromKey(key));
        }
      });
    }

    this.dispatchEvent(
      new CustomEvent('populate-query', {
        detail: paramSet,
        bubbles: true,
      })
    );
  };

  private closeExplore() {
    // Remove the explore object from the list in `explore-multi-sk.ts`.
    const detail = { elem: this };
    this.dispatchEvent(new CustomEvent('remove-explore', { detail: detail, bubbles: true }));
  }

  reset() {
    this.removeAll(false);
    if (this.openQueryByDefault) {
      this.openQuery();
    }
  }

  // Show full graph title if the graph title exists.
  showFullTitle() {
    if (this.graphTitle === null || this.graphTitle === undefined) {
      return;
    }

    this.graphTitle.showFullTitle();
  }

  // Show short graph title if the graph title exists.
  showShortTitle() {
    if (this.graphTitle === null || this.graphTitle === undefined) {
      return;
    }

    this.graphTitle.showShortTitles();
  }

  // Use commit and trace number to find the previous commit in the trace.
  private getPreviousCommit(index: number, traceName: string): CommitNumber | null {
    // First the previous commit that has data.
    let prevIndex: number = index - 1;
    const trace = this.dfRepo.value?.dataframe.traceset[traceName] || [];
    while (prevIndex > -1 && trace[prevIndex] === MISSING_DATA_SENTINEL) {
      prevIndex = prevIndex - 1;
    }
    const previousCommit = this.dfRepo.value?.header[prevIndex]?.offset || null;
    if (previousCommit === null) {
      return null;
    }
    return previousCommit;
  }

  /**
   * getCommitDetails returns the commit details for the given commit position
   * from the dataframe header.
   * @param commitPosition Commit position to query.
   * @returns ColumnHeader object containing the details.
   */
  private getCommitDetails(commitPosition: CommitNumber | null): ColumnHeader | null {
    let colHeader = this.dfRepo.value?.header.find(
      (colHeader: ColumnHeader | null) => colHeader?.offset === commitPosition
    );
    if (colHeader === undefined) {
      colHeader = null;
    }
    return colHeader;
  }

  private getCommitIndex(
    value: number,
    type: string = DOMAIN_COMMIT,
    max: boolean = false
  ): number {
    // Ensure the index value is an integer.
    value = Math.round(value);
    const header = this.dfRepo.value?.header;
    if (!header || header.length === 0) {
      return -1;
    }

    const prop = type === DOMAIN_COMMIT ? 'offset' : 'timestamp';

    // First, try to find an exact match.
    let index = header.findIndex((h: ColumnHeader | null) => h?.[prop] === value);
    if (index !== -1) {
      return index;
    }

    // If no exact match, find the index of the first element greater than value.
    index = header.findIndex(
      (h: ColumnHeader | null) => h !== undefined && h !== null && h[prop] > value
    );

    // If no element is greater, 'value' is larger than all elements.
    // The closest is the last one.
    if (index === -1 || (index === 0 && max)) {
      return header.length - 1;
    }

    // If 'value' is smaller than all elements, the closest is the first one.
    if (index === 0) {
      return 0;
    }

    // We have two candidates, the one before and the one at the found index.
    // Compare with the previous element to see which is closer.
    const diffNext = header[index]![prop] - value;
    const diffPrev = value - header[index - 1]![prop];

    if (diffNext < diffPrev) {
      return index;
    }
    return index - 1;
  }

  // onChartSelect shows the tooltip whenever a user clicks on a data
  // point and the tooltip will lock in place until it is closed.
  private onChartSelect(e: CustomEvent) {
    const chart = this.googleChartPlot!.value!;
    const index = e.detail;
    const commitPos: CommitNumber = chart.getCommitPosition(index.tableRow);
    const traceName = chart.getTraceName(index.tableCol);
    const anomaly = this.dfRepo.value?.getAnomaly(traceName, commitPos) || null;
    this.selectedAnomaly = anomaly;
    const position = chart.getPositionByIndex(index);
    const commit = this.getCommitDetails(commitPos);
    const prevCommit = this.getCommitDetails(this.getPreviousCommit(index.tableRow, traceName));
    this.state.selected = {
      name: traceName,
      commit: commitPos,
      tableCol: index.tableCol,
      tableRow: index.tableRow,
    };

    this.enableTooltip(
      {
        x: index.tableRow - (this.selectedRange?.begin || 0),
        y: chart.getYValue(index),
        xPos: position.x,
        yPos: position.y,
        name: traceName,
      },
      [prevCommit, commit],
      true
    );
    this.tooltipSelected = true;
    this.updateBrowserURL();

    // If traces are rendered and summary bar is enabled, show
    // summary for the trace clicked on the graph.
    if (this.summaryOptionsField.value && this.traceIdSummaryOptionMap.size > 1) {
      const option = this.traceIdSummaryOptionMap.get(traceName) || '';
      if (option !== '') {
        this.summaryOptionsField.value!.setValue(option);
      } else {
        errorMessage(`Summary bar not properly set for this trace. Trace Name: ${traceName}`);
      }
    }
  }

  // if the tooltip is opened and the user is not shift-clicking,
  // close it when clicking on the chart
  // i.e. clicking away from the tooltip closes it
  // One edge case this implementation does not address is if the user
  // tries to click onto another data point while the tooltip is open.
  // This action is treated as onChartMouseDown and not as
  // onChartSelect so the tooltip only closes.
  private onChartMouseDown(): void {
    window.clearTimeout(this.hoverTimeout);
    this.closeTooltip();
  }

  // Close the tooltip when moving away while hovering.
  // Tooltip stays if the data point has been selected (clicked-on)
  private onChartMouseOut(): void {
    window.clearTimeout(this.hoverTimeout);
    if (!this.tooltipSelected) {
      this.closeTooltip();
    }
  }

  // onChartOver shows the tooltip whenever a user hovers their mouse
  // over a data point in the google chart
  private onChartOver({ detail }: CustomEvent<PlotShowTooltipEventDetails>): void {
    window.clearTimeout(this.hoverTimeout);
    const chart = this.googleChartPlot!.value!;
    if (this.paramset) {
      this.paramset!.highlight = fromKey(chart.getTraceName(detail.tableCol));
    }

    if (this.tooltipSelected) {
      return;
    }

    this.hoverTimeout = window.setTimeout(() => {
      const commitPos = chart.getCommitPosition(detail.tableRow);
      const position = chart.getPositionByIndex(detail);
      const traceName = chart.getTraceName(detail.tableCol);
      const currentCommit = this.getCommitDetails(commitPos);
      const prevCommit = this.getCommitDetails(this.getPreviousCommit(detail.tableRow!, traceName));
      this.enableTooltip(
        {
          x: detail.tableRow - (this.selectedRange?.begin || 0),
          y: chart.getYValue(detail),
          xPos: position.x,
          yPos: position.y,
          name: chart.getTraceName(detail.tableCol),
        },
        [prevCommit, currentCommit],
        false
      );
    }, HOVER_DEBOUNCE_MS);
  }

  connectedCallback(): void {
    super.connectedCallback();
    if (this._initialized) {
      return;
    }

    this.setUseDiscreteAxis(this._state.evenXAxisSpacing);

    this._initialized = true;
    this.render();

    this.detailTab = this.querySelector('#detailTab');
    this.formula = this.querySelector(`#formula-${this.uniqueId}`);
    this.json = this.querySelector(`#json-${this.uniqueId}`);
    if (this.json && this.json.value === '') {
      this.json.value = JSON.stringify(
        {
          graphs: [
            {
              queries: [],
              formulas: [],
              keys: '',
            },
          ],
        },
        null,
        2
      );
    }
    this.logEntry = this.querySelector('#logEntry');
    this.paramset = this.querySelector('#paramset');
    this.percent = this.querySelector('#percent');
    this.pivotControl = this.querySelector('pivot-query-sk');
    this.pivotDisplayButton = this.querySelector(`#pivot-display-button-${this.uniqueId}`);
    this.pivotTable = this.querySelector('pivot-table-sk');
    this.query = this.querySelector('#query');
    this.queryCount = this.querySelector('query-count-sk');
    this.range = this.querySelector('#range');
    this.spinner = this.querySelector('#spinner');
    this.summary = this.querySelector('#summary');
    this.commitTime = this.querySelector('#commit_time');
    this.queryDialog = this.querySelector('#query-dialog');
    this.helpDialog = this.querySelector('#help');
    this.pinpointJobToast = this.querySelector('#pinpoint-job-toast');
    this.closePinpointToastButton = this.querySelector('#hide-pinpoint-toast');
    this.triageResultToast = this.querySelector('#triage-result-toast');
    this.closeTriageToastButton = this.querySelector('#hide-triage-toast');
    this.collapseButton = this.querySelector('#collapseButton');
    this.graphTitle = this.querySelector<GraphTitleSk>('#graphTitle');
    this.is_anomaly_table = document.querySelector('#anomaly-table') ? true : false;

    // Update the range picker's state from our own state, since our state may
    // have been set before we were attached to the DOM and this.range was populated.
    if (this.range) {
      this.range.state = {
        begin: this._state.begin,
        end: this._state.end,
        num_commits: this._state.numCommits,
        request_type: this._state.requestType,
      };
    }
    // material UI stuff
    this.settingsDialog = this.querySelector<MdDialog>('#settings-dialog');

    // Populate the query element.
    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
    if (this.user === '') {
      LoggedIn()
        .then((status: LoginStatus) => {
          this.user = status.email;
        })
        .catch(errorMessage);
    }
    if (this.state.graph_index === 0) {
      this.dataService
        .getInitPage(tz)
        .then((json) => {
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
        .catch((msg) => {
          if (msg instanceof DataServiceError) {
            errorMessage(msg.message);
          } else {
            errorMessage(msg);
          }
        });
    }
    this.closePinpointToastButton!.addEventListener('click', () => this.pinpointJobToast?.hide());
    this.closeTriageToastButton!.addEventListener('click', () => this.triageResultToast?.hide());

    // Add an event listener for when a new bug is filed or an existing bug is submitted in the tooltip.
    this.addEventListener('anomaly-changed', (e) => {
      const detail = (e as CustomEvent).detail;
      if (!detail) {
        this.triageResultToast?.hide();
        return;
      }
      const toastText = document.getElementById('triage-result-text')! as HTMLSpanElement;
      // Display error passed in from event, do not attempt to change anomalies.
      if (detail.error) {
        toastText.textContent = `${detail.error}`;
        this.triageResultToast?.show();
        return;
      }

      const anomalies: Anomaly[] = detail.anomalies;
      const traceNames: string[] = detail.traceNames;
      if (anomalies.length === 0 || traceNames.length === 0) {
        this.triageResultToast?.hide();
        return;
      }
      for (let i = 0; i < anomalies.length; i++) {
        const commits: number[] = [];
        const anomalyMap: AnomalyMap = {};
        const commitMap: CommitNumberAnomalyMap = {};

        // TraceName should be the same across all anomalies.
        const traceName = traceNames[i] || traceNames[0];
        const startRevision: number | null = anomalies[i].start_revision;
        const endRevision: number | null = anomalies[i].end_revision;

        // Load all commits available on the plot.
        const data: (ColumnHeader | null)[] | null = this.dfRepo.value!.dataframe.header;
        if (data) {
          // Create array of all commits for easy lookup.
          data.forEach((data) => commits.push(data!.offset));
        }

        // Check for start or end commit and return first found.
        let anomalyCommit: number | undefined = 0;
        anomalyCommit = commits.find((commit) => {
          if (commit === startRevision || commit === endRevision) {
            return commit;
          }
        });

        if (anomalyCommit) {
          commitMap[anomalyCommit] = anomalies[i];
          anomalyMap[traceName] = commitMap;
          this.dfRepo.value?.updateAnomalies(anomalyMap, anomalies[i].id);
        }
        // Update pop-up with bug details from anomaly change.
        const bugId = detail.bugId;
        const displayIndex = detail.displayIndex;
        const editAction = detail.editAction;
        const toastLink = document.getElementById('triage-result-link')! as HTMLAnchorElement;
        toastLink.innerText = ``;
        // BugId dictates that a bug is tied to the anomaly. (New or Existing).
        if (bugId) {
          toastText.textContent = `Anomaly associated with: `;
          const link = `https://issues.chromium.org/issues/${bugId}`;
          toastLink.innerText = `${bugId}`;
          toastLink.setAttribute('href', `${link}`);
          toastLink.setAttribute('target', '_blank');
        }
        // EditAction dictates this is a state change.
        if (editAction) {
          toastText.textContent = `Anomaly state changed to ${editAction}`;
        }
        // DisplayIndex dictates this is a nudge change.
        if (displayIndex) {
          if (anomalyCommit) {
            toastText.textContent = `Anomaly Nudge Moved (${displayIndex}) to ${anomalyCommit}`;
            // Since anomaly is moving, close tooltip.
            this.closeTooltip();
          } else {
            // If no anomaly commit was found, then the nudge failed.
            toastText.textContent = `Anomaly Nudge Failed (${displayIndex}) to ${startRevision}!`;
          }
        }
        this.triageResultToast?.show();
        this._stateHasChanged();
        this.render();
      }
    });

    // Listens to the user-issue-changed event and does appropriate actions
    // to update the overall userIssue map along with re-rendering the
    // new set of user issues on the chart
    this.addEventListener('user-issue-changed', (e) => {
      const repo = this.dfRepo.value;
      const traceKey = (e as CustomEvent).detail.trace_key as string;
      const commitPosition = (e as CustomEvent).detail.commit_position as number;
      const bugId = (e as CustomEvent).detail.bug_id as number;

      // Update the over user issues map in dataframe_context
      repo!.updateUserIssue(traceKey, commitPosition, bugId);
    });
    this.addEventListener('plot-chart-mousedown', () => {
      this.onChartMouseDown();
    });
    this.addEventListener('plot-chart-mouseout', () => {
      this.onChartMouseOut();
    });
    this.addEventListener('split-chart-selection', (e) => {
      this.splitByAttribute(e as CustomEvent);
    });
  }

  render(): void {
    this._render();
    // Determine if in split chart mode.
    const chartTotal = document.querySelectorAll('explore-simple-sk');
    const testPicker = document.querySelector('test-picker-sk');
    this.is_chart_split = chartTotal.length > 1 && testPicker ? true : false;
  }

  showSettingsDialog(_event: Event) {
    this.settingsDialog!.show();
  }

  // Handle the x-axis switch toggle.
  // If the switch is selected, set the x-axis to date mode.
  switchXAxis(target: MdSwitch | null) {
    if (!target) return;
    const newDomain = target.selected ? DOMAIN_DATE : DOMAIN_COMMIT;

    // Dispatch the event first to sync other charts.
    if (this.is_chart_split) {
      const detail = {
        index: this.state.graph_index,
        domain: newDomain,
      };
      this.dispatchEvent(new CustomEvent('x-axis-toggled', { detail, bubbles: true }));
    }
    // Then update the current chart.
    this.updateXAxis(newDomain);
  }

  // Set the x-axis domain to either commit or date.
  // This is used to update the x-axis domain when the user toggles the switch.
  // It also is an entry point from multichart to update related charts which is
  // why it has a separate method.
  updateXAxis(domain: string) {
    if (this.state.domain === domain) {
      return;
    }

    this._updateSelectionRangeForDomainChange(domain);

    // Now, update the state and the child components' domains.
    this.xAxisSwitch = !this.xAxisSwitch;
    this._state.domain = domain;
    this._state.labelMode = domain === DOMAIN_DATE ? LabelMode.Date : LabelMode.CommitPosition;

    if (this.googleChartPlot.value) {
      this.googleChartPlot.value.domain = domain as 'commit' | 'date';
    }

    if (this.plotSummary.value) {
      this.plotSummary.value.domain = domain as 'commit' | 'date';
    }
    this.render();
    this._stateHasChanged();
  }

  private _updateSelectionRangeForDomainChange(newDomain: string) {
    const header = this.dfRepo.value?.header;
    const plotSummary = this.plotSummary.value;
    const oldDomain = this.state.domain;

    if (header && plotSummary && plotSummary.selectedValueRange) {
      const currentRange = plotSummary.selectedValueRange;

      const beginIndex = this.getCommitIndex(currentRange.begin, oldDomain);
      const endIndex = this.getCommitIndex(currentRange.end, oldDomain, true);

      if (beginIndex !== -1 && endIndex !== -1) {
        const newBegin =
          newDomain === DOMAIN_DATE ? header[beginIndex]!.timestamp : header[beginIndex]!.offset;
        const newEnd =
          newDomain === DOMAIN_DATE ? header[endIndex]!.timestamp : header[endIndex]!.offset;
        const newRange = { begin: newBegin, end: newEnd };

        // Set the cached value on the child. It will apply this selection once
        // its internal chart has re-rendered with the new domain.
        (
          plotSummary as unknown as { cachedSelectedValueRange: range | null }
        ).cachedSelectedValueRange = newRange;
      }
    }
  }

  // updates the chart height using a string input.
  // typically 500px for a single chart and 250px for multiple charts
  updateChartHeight(height: string) {
    const chart = this.querySelector('div.chart-container') as HTMLElement;
    chart.classList.remove('small-height', 'medium-height');
    chart.style.height = '';

    if (height === '250px') {
      chart.classList.add('small-height');
    } else if (height === '500px') {
      chart.classList.add('medium-height');
    } else {
      chart.style.height = height;
    }
  }

  /**
   * Asynchronously updates the URL for the Multigraph (/m/?shortcut=...) link.
   *
   * This method constructs a URL based on the current graph configuration
   * (queries, formulas, keys) and time range (`begin`, `end`, `requestType`)
   * stored in `this._state`.
   *
   * It calls the `updateShortcut` function to asynchronously obtain a shortcut ID
   * for the current graph state. Once the shortcut ID is retrieved, it builds
   * a new URL for the multi-graph page (`/m/`) including the state parameters
   * and the shortcut.
   *
   * Uninitialized link placeholder is "#".
   */
  private updateTestPickerUrl() {
    const currentState = this._state;
    const graphConfig: GraphConfig = {
      queries: currentState.queries,
      formulas: currentState.formulas,
      keys: currentState.keys,
    };

    if (
      graphConfig.queries.length === 0 &&
      graphConfig.formulas.length === 0 &&
      graphConfig.keys === ''
    ) {
      if (this.testPickerUrl !== '#') {
        this.testPickerUrl = '#';
        this._render();
      }
      return;
    }

    updateShortcut([graphConfig])
      .then((shortcut) => {
        let newUrl = shortcut
          ? `/m/?begin=${currentState.begin}` +
            `&end=${currentState.end}` +
            `&request_type=${currentState.requestType}` +
            `&shortcut=${shortcut}` +
            `&totalGraphs=1`
          : '#';

        if (newUrl !== '#' && window.perf.default_to_manual_plot_mode) {
          newUrl += '&manual_plot_mode=true';
        }

        if (this.testPickerUrl !== newUrl) {
          this.testPickerUrl = newUrl;
          this._render();
        }
      })
      .catch((error) => {
        if (error instanceof DataServiceError && error.status === 500) {
          errorMessage('Unable to update shortcut.', 2000);
        }
        console.error('Failed to update shortcut for Test Picker URL:', error);
        if (this.testPickerUrl !== '#') {
          this.testPickerUrl = '#';
          this._render();
        }
      });
  }

  // Call this anytime something in private state is changed. Will be replaced
  // with the real function once stateReflector has been setup.
  // eslint-disable-next-line @typescript-eslint/no-empty-function
  private _stateHasChanged = () => {
    this.dispatchEvent(new CustomEvent('state_changed', {}));
  };

  private _renderedTraces = () => {
    this.dispatchEvent(new CustomEvent('rendered_traces', {}));
  };

  private switchZoom(target: MdSwitch | null) {
    this.zoomDirectionSwitch = !this.zoomDirectionSwitch;
    if (target!.selected) {
      this.state.horizontal_zoom = true;
    } else {
      this.state.horizontal_zoom = false;
    }
    this.render();
    const detail = {
      key: this.state.horizontal_zoom,
    };
    this.dispatchEvent(new CustomEvent('switch-zoom', { detail: detail, bubbles: true }));
  }

  private switchEvenXAxisSpacing(target: MdSwitch | null) {
    if (!target) return;
    this._state.evenXAxisSpacing = target.selected;

    this.setUseDiscreteAxis(this._state.evenXAxisSpacing);
    this.render();
    this.dispatchEvent(
      new CustomEvent('even-x-axis-spacing-changed', {
        detail: { value: this._state.evenXAxisSpacing, graph_index: this._state.graph_index },
        bubbles: true,
      })
    );
    this._stateHasChanged();
  }

  public setUseDiscreteAxis(value: boolean) {
    this._state.evenXAxisSpacing = value;
    if (this.googleChartPlot.value) {
      this.googleChartPlot.value.useDiscreteAxis = value;
    }
  }

  private closeQueryDialog(): void {
    this.queryDialog!.close();
    this._dialogOn = false;
  }

  keyDown = (e: KeyboardEvent) => {
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

    if (e.key === 'Escape') {
      this.closeTooltip();
      return;
    }

    handleKeyboardShortcut(e, this);
  };

  onZoomIn(): void {
    this.applyZoomOrPan(1, 0);
  }

  onZoomOut(): void {
    this.applyZoomOrPan(-1, 0);
  }

  onPanLeft(): void {
    this.applyZoomOrPan(0, -1);
  }

  onPanRight(): void {
    this.applyZoomOrPan(0, 1);
  }

  onTriagePositive(): void {
    const tooltip = $$<ChartTooltipSk>('chart-tooltip-sk', this);
    if (tooltip && tooltip.anomaly && tooltip.anomaly.bug_id === 0) {
      tooltip.openNewBug();
    }
  }

  onTriageNegative(): void {
    const tooltip = $$<ChartTooltipSk>('chart-tooltip-sk', this);
    if (tooltip && tooltip.anomaly && tooltip.anomaly.bug_id === 0) {
      tooltip.ignoreAnomaly();
    }
  }

  onTriageExisting(): void {
    const tooltip = $$<ChartTooltipSk>('chart-tooltip-sk', this);
    if (tooltip && tooltip.anomaly && tooltip.anomaly.bug_id === 0) {
      tooltip.openExistingBug();
    }
  }

  onOpenHelp(): void {
    const help = this.querySelector('keyboard-shortcuts-help-sk') as KeyboardShortcutsHelpSk;
    help.open();
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
    const header = this._dataframe.header;
    // Default to the full range
    const zoom: [number, number] = [0, (header?.length || 1) - 1];

    const plot = this.googleChartPlot.value;
    if (header && header.length > 0 && plot && plot.selectedRange) {
      // If we have a selected range, we need to map the begin/end values back to
      // indices in the dataframe header.
      const isDate = this._state.domain === DOMAIN_DATE;
      const { begin, end } = plot.selectedRange;

      // Find the index that corresponds to the begin value.
      // We assume the header is sorted.
      const beginIndex = header.findIndex((h) => {
        if (!h) return false;
        const val = isDate ? h.timestamp : h.offset;
        return val >= begin;
      });

      // Find the index that corresponds to the end value.
      let endIndex = header.findIndex((h) => {
        if (!h) return false;
        const val = isDate ? h.timestamp : h.offset;
        return val > end;
      });

      // If endIndex is -1, it means the end value is beyond the last element,
      // so we select up to the last element.
      if (endIndex === -1) {
        endIndex = header.length;
      }
      // The endIndex from findIndex is the *first* element > endVal, so the
      // actual range inclusive end index is endIndex - 1.
      endIndex = endIndex - 1;

      if (beginIndex !== -1 && endIndex !== -1 && beginIndex <= endIndex) {
        zoom[0] = beginIndex;
        zoom[1] = endIndex;
      }
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
      this.dataService
        .shift(req)
        .then((json: ShiftResponse) => {
          this._state.begin = json.begin;
          this._state.end = json.end;
          this._state.requestType = 0;
          this._stateHasChanged();
          this.rangeChangeImpl();
        })
        .catch((msg) => {
          if (msg instanceof DataServiceError) {
            errorMessage(msg.message);
          } else {
            errorMessage(msg);
          }
        });
    } else {
      const header = this._dataframe.header;
      const plot = this.googleChartPlot.value;

      if (!header || !plot) {
        return;
      }

      const isDate = this._state.domain === DOMAIN_DATE;
      const getValue = (idx: number) => {
        const h = header[idx];
        return isDate ? h!.timestamp : h!.offset;
      };

      // If the zoom is within the current dataframe, we don't need to fetch
      // new data, but we do need to update the chart.
      plot.selectedValueRange = {
        begin: getValue(zoom[0]),
        end: getValue(zoom[1]),
      };
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

  /**
   * Applies zoom or pan operations to the current chart selection.
   *
   * This method calculates the new selection range based on the current selection
   * and the desired zoom/pan direction. It handles two scenarios:
   * 1. The new range is within the currently loaded data bounds:
   *    - Directly updates the visual selection (via plotSummary or googleChartPlot)
   *    - Triggers a 'summary_selected' event to update the main chart view.
   * 2. The new range extends beyond the loaded data:
   *    - Calls `zoomOrRangeChange` to initiate a data fetch for the new range.
   *
   * The logric uses the exact values (Commit Offsets or Timestamps) rather than indices
   * to ensue consistency across different chart components.
   *
   * @param zoomDir 1 for Zoom In (shrink range), -1 for Zoom Out (expand range).
   * @param panDir -1 for Pan Left (shift range left), 1 for Pan Right (shift range right), 0 for none.
   */
  private applyZoomOrPan(zoomDir: number, panDir: number) {
    // We update the selection on the plot summary, which will trigger an event
    // to update the main plot.
    const plotSummary = this.plotSummary.value;
    const dfRepo = this.dfRepo.value;
    if (!plotSummary || !dfRepo) {
      return;
    }
    const currentRange = plotSummary.selectedValueRange;
    if (!currentRange) {
      return;
    }

    const width = currentRange.end - currentRange.begin;
    // We rely on the decimal precision here to ensure smooth zooming.
    // If we rounded to integers, consecutive small zooms might cancel out or stuck
    // (e.g. 10 * 0.1 = 1, but if width is 5, 5 * 0.1 = 0.5 -> rounds to 0 or 1).
    // The visualization (Google Chart) handles floating point ranges correctly.
    const zoomDelta = width * ZOOM_JUMP_PERCENT;

    let newBegin = currentRange.begin;
    let newEnd = currentRange.end;

    if (panDir !== 0) {
      newBegin += panDir * zoomDelta;
      newEnd += panDir * zoomDelta;
    } else {
      newBegin += zoomDir * zoomDelta;
      newEnd -= zoomDir * zoomDelta;
    }

    // Clamp to the loaded data range.
    const header = dfRepo.dataframe.header;
    if (!header || header.length === 0) {
      return;
    }
    const isDate = this._state.domain === DOMAIN_DATE;
    const dataBegin = isDate ? header[0]!.timestamp : header[0]!.offset;
    const dataEnd = isDate
      ? header[header.length - 1]!.timestamp
      : header[header.length - 1]!.offset;

    // Check if we need to fetch more data.
    // If we are panning/zooming out beyond the loaded data, we need to fetch.
    if (newBegin < dataBegin || newEnd > dataEnd) {
      const cz = this.getCurrentZoom();
      const zoom: CommitRange = [
        (cz.zoom[0] +
          (panDir !== 0 ? panDir : zoomDir) * ZOOM_JUMP_PERCENT * cz.delta) as CommitNumber,
        (cz.zoom[1] +
          (panDir !== 0 ? panDir : -zoomDir) * ZOOM_JUMP_PERCENT * cz.delta) as CommitNumber,
      ];
      this.zoomOrRangeChange(zoom);
      return;
    }

    // Apply visual update
    plotSummary.selectedValueRange = { begin: newBegin, end: newEnd };
    this.summarySelected(
      new CustomEvent('summary_selected', {
        detail: {
          value: { begin: newBegin, end: newEnd },
          domain: this._state.domain as 'commit' | 'date',
          start: 0,
          end: 0,
        },
      })
    );
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
    this.render();
    this._dialogOn = true;
    this.queryDialog!.show();
    // If there is a query already plotted, update the counts on the query dialog.
    if (this._state.queries.length > 0) {
      this.queryCount!.current_query = this.applyDefaultsToQuery(this.query!.current_query);
    }
  }

  private paramsetChanged(e: CustomEvent<ParamSet>) {
    this.query!.paramset = e.detail;
    this.pivotControl!.paramset = e.detail;
    this.render();
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
    this.queryCount!.current_query = this.applyDefaultsToQuery(e.detail.q);
  }

  private pivotTableSortChange(e: CustomEvent<PivotTableSkChangeEventDetail>): void {
    this._state.sort = e.detail;
    this._stateHasChanged();
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
    return tmp ? tmp : '';
  }

  private selectedRange?: range;

  /**
   * React to the summary_selected event.
   * @param e Event object.
   */
  summarySelected({ detail }: CustomEvent<PlotSummarySkSelectionEventDetails>): void {
    this.updateSelectedRangeWithUpdatedDataframe(detail.value, detail.domain);
    // When summary selection is changed, fetch the comments for the new range
    // and update the plot.
    const dfRepo = this.dfRepo.value;
    const begin = Math.floor(detail.value.begin) ?? 0;
    const end = Math.ceil(detail.value.end) ?? 0;
    const invalidRange = begin === 0 || end === 0;
    if (dfRepo !== null && dfRepo !== undefined && !invalidRange) {
      dfRepo
        .getUserIssues(Object.keys(dfRepo.traces), begin, end)
        .then((userIssues: UserIssueMap) => {
          if (Object.keys(userIssues || {}).length > 0) {
            this.updateSelectedRangeWithUpdatedDataframe(detail.value, detail.domain);
          }
        });
    }
    detail.start = this.getCommitIndex(begin, this.state.domain);
    detail.end = this.getCommitIndex(end, this.state.domain, true);

    // If in multi-graph view, sync all graphs.
    // This event listener will not work on the alerts page
    detail.graphNumber = this.state.graph_index;
    this.dispatchEvent(
      new CustomEvent<PlotSelectionEventDetails>('selection-changing-in-multi', {
        bubbles: true,
        detail: detail,
      })
    );
    // Only close tooltip if the point is no longer on the chart.
    if (detail.start === 0) {
      this.closeTooltip();
    }
    this.updateBrowserURL();
  }

  async extendRange(range: range, offset?: number): Promise<void> {
    const dfRepo = this.dfRepo.value;
    const header = dfRepo?.dataframe?.header;
    if (!dfRepo || !header || header.length === 0) {
      return;
    }
    if (offset) {
      await dfRepo.extendRange(offset);
      return;
    }

    let extendDirection = 0;
    if (range.begin < header[0]!.offset) {
      extendDirection = -1;
    } else if (range.end > header[header.length - 1]!.offset) {
      extendDirection = 1;
    }
    if (extendDirection !== 0 || offset !== undefined) {
      await dfRepo.extendRange(extendDirection * monthInSec);
    }
  }

  private async OnSelectionRange({
    type,
    detail,
  }: CustomEvent<PlotSelectionEventDetails>): Promise<void> {
    if (type === 'selection-changed') {
      await this.extendRange(detail.value);
    }

    if (this.plotSummary.value) {
      this.plotSummary.value.selectedValueRange = detail.value;
    }
    this.updateBrowserURL();
    this.closeTooltip();
    // If in multi-graph view, sync all graphs.
    // This event listener will not work on the alerts page
    detail.graphNumber = this.state.graph_index;
    this.dispatchEvent(
      new CustomEvent<PlotSelectionEventDetails>('selection-changing-in-multi', {
        bubbles: true,
        detail: detail,
      })
    );
    this.updateSelectedRangeWithUpdatedDataframe(detail.value, detail.domain);
  }

  private clearTooltipDataFromURL(): void {
    const currentUrl = new URL(window.location.href);
    currentUrl.searchParams.delete('graph');
    currentUrl.searchParams.delete('commit');
    currentUrl.searchParams.delete('trace');
    window.history.pushState(null, '', currentUrl.toString());
  }

  /**
   * Reads parameters from the browser URL and applies them to the chart.
   * This is used to restore the state of the chart from a shared URL.
   *
   * It handles three main pieces of state from the URL:
   * 1. Time Range (begin/end): Sets the visible range of the chart. It converts
   *    the 'begin' and 'end' timestamp URL params to commit offsets and updates
   *    the plot summary's selection.
   * 2. Point Selection (graph/commit/trace): If the URL specifies a 'graph' that
   *    matches this component's index, it will select the specific 'commit' and
   *    'trace' (column) on the chart.
   * 3. Split by Key (splitByKey): If the 'splitByKey' param is present, it will
   *    trigger the action to split the chart by that parameter key.
   * 4. JSON (json): If the 'json' param is present, it will parse it and load the graph.
   *
   * @param selection - If true, it will select the commit and trace specified in the URL.
   *                    This is needed when popping the selection too early.
   * @param url - Optional URL to use instead of window.location.href. Useful for testing.
   */
  useBrowserURL(selection: boolean = true, url: URL = new URL(window.location.href)): void {
    if (this.isReportPage) {
      return;
    }
    const currentUrl = url;
    const json = currentUrl.searchParams.get('json');
    if (json) {
      if (this.json) {
        this.json.value = json;
      }
      this.add(true, 'json');
      return;
    }

    const commit: number = parseInt(
      currentUrl.searchParams.get('commit') ?? String(this._state.selected.commit)
    );
    const column: number = parseInt(
      currentUrl.searchParams.get('trace') ?? String(this._state.selected.tableCol)
    );
    const graph: number = parseInt(currentUrl.searchParams.get('graph') ?? '0');

    const begin = parseInt(currentUrl.searchParams.get('begin') ?? this.state.begin.toString());
    const end = parseInt(currentUrl.searchParams.get('end') ?? this.state.end.toString());
    if (isNaN(begin) || isNaN(end)) {
      // When no value is found in the URL, then use the first graph to update it.
      if (this.state.graph_index === 0) {
        this.updateBrowserURL();
      }
    }
    // Pull from URL which is always a timestamp.
    const beginIndex = this.getCommitIndex(begin, 'timestamp');
    const endIndex = this.getCommitIndex(end, 'timestamp', true);
    if (beginIndex === -1 || endIndex === -1) {
      // When no commit is found in the dataframe, return.
      // Only message if not anomaly table where graphs are not synced.
      if (!this.is_anomaly_table) {
        console.error(`Timestamp(s) not found in the dataframe: ${begin}, ${end}`);
      }
      return;
    }

    if (this.state.graph_index === 0 || this.is_anomaly_table) {
      const beginCommit = this.dfRepo.value?.header[beginIndex];
      const endCommit = this.dfRepo.value?.header[endIndex];
      if (beginCommit === undefined || endCommit === undefined) {
        console.error(`Begin or end commit offset not found in the dataframe.`);
        return;
      }

      let beginValue: number = beginCommit!.offset;
      let endValue: number = endCommit!.offset;
      if (this.state.domain === DOMAIN_DATE) {
        beginValue = beginCommit!.timestamp;
        endValue = endCommit!.timestamp;
      }
      if (this.parentNode) {
        const graphNumber = Array.from(this.parentNode!.children).indexOf(this);
        const detail: PlotSummarySkSelectionEventDetails = {
          graphNumber: graphNumber,
          value: { begin: beginValue, end: endValue },
          domain: this.state.domain as 'commit' | 'date',
          start: 0,
          end: 0,
        };
        if (this.plotSummary.value) {
          this.plotSummary.value!.selectedValueRange = detail.value;
          this.summarySelected(new CustomEvent('summary_selected', { detail: detail }));
        }
      }
    }

    if (this.state.graph_index === graph && commit && !isNaN(commit)) {
      // If the commit is specified, we need to select it in the chart.
      const commitIndex = this.getCommitIndex(commit, this.state.domain);
      if (commitIndex === null) {
        errorMessage(`Commit not found in the dataframe: ${commit}`);
        return;
      }
      const googlePlot = this.googleChartPlot.value;
      if (googlePlot && selection) {
        googlePlot.selectCommit(commitIndex, column);
      }
    }

    // When loading the chart, look if SplitByKey is in the URL.
    const splitKey = currentUrl.searchParams.get('splitByKeys');
    if (this.state.graph_index === 0 && splitKey) {
      const checkbox = document.querySelector(`checkbox-sk[name="${splitKey}"]`) as CheckOrRadio;
      // If not checked and present, then split the loaded data.
      if (checkbox && !checkbox.checked) {
        checkbox.checked = true;
        // Dispatch the event to split the chart by the given key.
        this.dispatchEvent(
          new CustomEvent('split-by-changed', {
            detail: {
              param: splitKey,
              split: true,
            },
            bubbles: true,
            composed: true,
          })
        );
      }
    }
  }

  /**
   * Updates the browser URL with the new begin and end values.
   * This is used to reflect the current state of the graph in the URL.
   */
  private updateBrowserURL(): void {
    if (this.isReportPage) {
      return;
    }
    const currentUrl = new URL(window.location.href);
    // The tooltip is opened to a commit, so we need to update the URL to match.
    if (this._state.selected.tableCol && this._state.selected.tableCol !== -1) {
      currentUrl.searchParams.set('graph', this.state.graph_index.toString());
      currentUrl.searchParams.set('commit', this.state.selected.commit.toString());
      currentUrl.searchParams.set('trace', this._state.selected.tableCol.toString());
    }
    try {
      // Update the URL with the current range.
      if (this.state.graph_index === 0) {
        currentUrl.searchParams.set('request_type', '0');
        currentUrl.searchParams.set('begin', this.state.begin.toString());
        currentUrl.searchParams.set('end', this.state.end.toString());
      }
      if (currentUrl.toString() !== new URL(window.location.href).toString()) {
        window.history.pushState(null, '', currentUrl.toString());
      }
    } catch (error) {
      console.error('Failed to update state to URL:', error);
    }
  }

  // updateSelectedRangeWithPlotSummary is used to synchronize
  // multiple explore-simple-sks on a explore-multi page
  // This function syncs google chart panning and plot-summary selection + panning
  public updateSelectedRangeWithPlotSummary(range: range, beginIndex: number, endIndex: number) {
    // Summary bar has not been loaded yet.
    if (!this.plotSummary.value) {
      return;
    }
    if (this.googleChartPlot.value) {
      this.googleChartPlot.value.selectedValueRange = range;
      const begin = this.dfRepo.value?.header[beginIndex]?.timestamp;
      const end = this.dfRepo.value?.header[endIndex]?.timestamp;
      this.state.begin = parseInt((begin ?? this.state.begin).toString());
      this.state.end = parseInt((end ?? this.state.end).toString());
    }
    this.plotSummary.value.selectedValueRange = range;
    this.updateBrowserURL();
    this.closeTooltip();
  }

  private updateSelectedRangeWithUpdatedDataframe(
    range: range,
    domain: 'commit' | 'date',
    replot = true
  ) {
    // Do not try to initialize graph if no time range yet provided.
    if (range.begin <= 0) {
      return;
    }
    if (this.googleChartPlot.value) {
      this.googleChartPlot.value.selectedRange = range;
      this.googleChartPlot.value.showZero = this.state.showZero;
    }

    const df = this.dfRepo.value?.dataframe;
    const header = df?.header || [];
    const selected = findSubDataframe(header!, range, domain);
    this.selectedRange = selected;

    const subDataframe = generateSubDataframe(df!, selected);
    if (!subDataframe || subDataframe.header?.length === 0) {
      // If the subDataframe is empty, we cannot proceed.
      errorMessage('Unable to find requested data range.');
      return;
    }
    this.state.begin = subDataframe.header![0]!.timestamp;
    this.state.end = subDataframe.header![subDataframe.header!.length - 1]!.timestamp;

    // Update the current dataframe to reflect the selection.
    this._dataframe.traceset = subDataframe.traceset;
    this._dataframe.header = subDataframe.header;
    this._dataframe.traceMetadata = subDataframe.traceMetadata;
    this.updateTracePointMetadata(subDataframe.traceMetadata);

    if (replot) {
      this.AddPlotLines();
    }
  }

  enableTooltip(
    pointDetails: TraceEventDetails,
    commits: (ColumnHeader | null)[],
    fixTooltip: boolean
  ): void {
    // explore-simple-sk is used multiple times on the multi-graph view. To
    // make sure that appropriate chart-tooltip-sk element is selected, we
    // start the search from the explore-simple-sk that the user is hovering/
    // clicking on
    const tooltipElem = $$<ChartTooltipSk>('chart-tooltip-sk', this);
    tooltipElem?.reset();

    const x = (this.selectedRange?.begin || 0) + pointDetails.x;
    let commitDate = new Date();

    // Store the hashes of the first two commits in the range.
    // Show the commit hashes in the tooltip without having to refetch.
    let hashes: string[] = [];
    // Ensure that at least 2 hashes exist and use null if needed.
    if (commits && commits.length >= 2) {
      hashes = [commits[0]?.hash ?? '', commits[1]?.hash ?? ''];
    }

    const commit = commits ? commits[1] : null;
    if (commit === null) {
      const chart = this.googleChartPlot!.value!;
      commitDate = chart.getCommitDate(x);
    } else {
      commitDate = new Date(commit!.timestamp * 1000);
    }

    const traceName = pointDetails.name;
    const header = this.dfRepo.value?.header || null;
    const commitPosition = this.dfRepo.value!.dataframe.header![x]!.offset;
    const chart = this.googleChartPlot!.value!;
    const color = chart.getTraceColor(traceName);
    const prevCommitPos = this.getPreviousCommit(x, traceName);
    const anomaly = this.dfRepo.value?.getAnomaly(traceName, commitPosition) || null;
    const trace = this.dfRepo.value?.dataframe.traceset[traceName] || [];
    this.startCommit = prevCommitPos?.toString() || '';
    this.endCommit = commitPosition.toString();

    if (!this.disablePointLinks) {
      tooltipElem!
        .loadPointLinks(
          commitPosition,
          prevCommitPos,
          traceName,
          window.perf.keys_for_commit_range!,
          window.perf.keys_for_useful_links!,
          this.commitLinks
        )
        .then((links) => {
          this.commitLinks = links;
        })
        .catch((errorMessage) => {
          console.error('Error loading point links:', errorMessage);
        });
    }
    // TODO(b/370804498): To be refactored into google plot / dataframe.
    // The anomaly data is indirectly referenced from simple-plot, and the anomaly data gets
    // updated in place in triage popup. This may cause the data inconsistency to manipulate
    // data in several places.
    // Ideally, dataframe_context should nudge anomaly data.
    // Map an anomaly ID to a list of Nudge Entries.
    // TODO(b/375678060): Reflect anomaly coordinate changes unto summary bar.
    const nudgeList: NudgeEntry[] = [];
    if (anomaly) {
      // This is only to be backward compatible with anomaly data in simple-plot.
      const anomalyData: AnomalyData = {
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
        const y = this.dfRepo.value!.dataframe.traceset![traceName][x + i];
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
    const closeBtnAction = fixTooltip
      ? () => {
          this.closeTooltip();
        }
      : () => {};

    let buganizerId = 0;
    const userIssueMap = this.dfRepo.value?.userIssues;
    if (userIssueMap && Object.keys(userIssueMap).length > 0) {
      const buganizerTraces = userIssueMap[traceName];
      if (buganizerTraces !== null && buganizerTraces !== undefined) {
        buganizerId = buganizerTraces[commitPosition]?.bugId || 0;
      }
    }

    let unitValue: string = '';
    traceName.split(',').forEach((test) => {
      if (test.startsWith('unit')) {
        unitValue = test.split('=')[1];
      }
    });

    // Get the legend index for the test name, subtract 2 to skip date/number.
    const legendIndex =
      this.dfRepo.value && this.dfRepo.value.data
        ? this.dfRepo.value.data.getColumnIndex(traceName) - 2
        : -1;
    const getLegendData = getLegend(this.dfRepo.value!.data);
    let testName = legendFormatter(getLegendData)[legendIndex];

    // Get index of legend and remove unit to display in tooltip.
    if (unitValue.length > 0) {
      testName = testName.replace('/' + unitValue, '');
    }

    // Populate the commit-range-sk element.
    // Backwards compatibility to tooltip load() function.
    const commitRangeSk = new CommitRangeSk();
    commitRangeSk.autoload = false;
    commitRangeSk!.trace = trace;
    commitRangeSk!.header = header;
    commitRangeSk!.hashes = hashes;
    commitRangeSk!.commitIndex = x;

    if (anomaly !== null && anomaly.bug_id > 0) {
      this.bugId = anomaly.bug_id.toString();
    } else {
      this.bugId = '';
    }
    this.constructTestPath(traceName);

    const preloadBisectInputs: BisectPreloadParams = {
      testPath: this.testPath,
      startCommit: this.startCommit,
      endCommit: this.endCommit,
      bugId: this.bugId,
      anomalyId: this.selectedAnomaly ? String(this.selectedAnomaly.id) : '',
      story: this.story ? this.story : '',
    };
    const preloadTryJobInputs: TryJobPreloadParams = {
      testPath: this.testPath,
      baseCommit: this.startCommit,
      endCommit: this.endCommit,
      story: this.story ? this.story : '',
    };

    tooltipElem!.moveTo({ x: pointDetails.xPos!, y: pointDetails.yPos! });
    tooltipElem!.setBisectInputParams(preloadBisectInputs);
    tooltipElem!.setTryJobInputParams(preloadTryJobInputs);
    tooltipElem!.load(
      legendIndex,
      testName,
      traceName,
      unitValue,
      pointDetails.y,
      commitDate,
      commitPosition,
      buganizerId,
      anomaly,
      nudgeList,
      commit,
      fixTooltip,
      commitRangeSk,
      closeBtnAction,
      color,
      this.user
    );
    if (window.perf.show_json_file_display) {
      tooltipElem!.loadJsonResource(commitPosition, traceName);
    }
  }

  // if the user's cursor leaves the graph, close the tooltip
  // unless the tooltip is 'fixed', meaning the user has anchored it.
  mouseLeave(): void {
    if (!this.tooltipSelected) {
      this.closeTooltip();
    }
  }

  clearTooltip(): void {
    const tooltipElem = $$<ChartTooltipSk>('chart-tooltip-sk', this);
    tooltipElem?.reset();
  }

  /** Hides the tooltip. Generally called when mouse moves out of the graph */
  closeTooltip(): void {
    const tooltipElem = $$<ChartTooltipSk>('chart-tooltip-sk', this);
    if (!tooltipElem || tooltipElem.index === -1) {
      return;
    }
    tooltipElem!.moveTo(null);
    if (this.tooltipSelected) {
      this.clearSelectedState();
      this.clearTooltipDataFromURL();
    }
    this.tooltipSelected = false;
    tooltipElem?.reset();
    // unselect all selected item on the chart
    this.googleChartPlot.value?.unselectAll();
    if (this.plotSummary.value?.selectedTrace) {
      this.plotSummary.value!.selectedTrace = '';
    }

    this.render();
  }

  /** Highlight a trace when it is clicked on. */
  // TODO(b/447094435) I suspect this function has no effect due to rerendering
  // and calls to google-chart. Need to verify.
  traceSelected({ detail }: CustomEvent<TraceEventDetails>): void {
    // If traces are rendered and summary bar is enabled, show
    // summary for the trace clicked on the graph.
    if (this.summaryOptionsField.value) {
      const option = this.traceIdSummaryOptionMap.get(detail.name) || '';
      this.summaryOptionsField.value!.setValue(option);
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

    if (prevCommit !== -1) {
      for (let c = commit - 1; c > prevCommit; c--) {
        commits.push(c as CommitNumber);
      }
    }

    // Find if selected point is an anomaly.
    const selected_anomaly: Anomaly | null =
      this.dfRepo.value?.getAnomaly(detail.name, detail.x) ?? null;
    this.selectedAnomaly = selected_anomaly;

    const paramset = ParamSet({});
    let paramsets = [];
    const params: { [key: string]: string } = fromKey(detail.name);
    Object.keys(params).forEach((key) => {
      paramset[key] = [params[key]];
    });

    paramsets = [paramset as CommonSkParamSet];

    this.render();

    this._state.selected.name = detail.name;
    this._state.selected.commit = commit;
    this._stateHasChanged();

    const parts: string[] = [];
    this.story = this.getLastSubtest(paramsets[0]!)[0];
    if (paramsets[0]!.master && paramsets[0]!.master.length > 0) {
      parts.push(paramsets[0]!.master[0]);
    }
    if (paramsets[0]!.bot && paramsets[0]!.bot.length > 0) {
      parts.push(paramsets[0]!.bot[0]);
    }
    if (paramsets[0]!.benchmark && paramsets[0]!.benchmark.length > 0) {
      parts.push(paramsets[0]!.benchmark[0]);
    }
    if (paramsets[0]!.test && paramsets[0]!.test.length > 0) {
      parts.push(paramsets[0]!.test[0]);
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

    // Open the details section if it is currently collapsed
    if (!this.navOpen) {
      this.collapseButton?.click();
    }
  }

  private clearSelectedState() {
    // Switch back to the params tab since we are about to hide the details tab.
    if (this.detailTab) {
      this.detailTab!.selected = PARAMS_TAB_INDEX;
    }
    if (this.logEntry) {
      this.logEntry!.textContent = '';
    }
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
    const defaultRangeS = this.getDefaultRange();
    const now = Math.floor(Date.now() / 1000);
    let currentBegin = state.begin;
    let currentEnd = state.end;

    // Default application should only happen if the state is uninitialized.
    // In the ExploreMultiSk context, begin and end should always be > -1.
    if (currentBegin === -1 && currentEnd === -1) {
      console.warn('ExploreSimpleSk: begin and end were not initialized.');
      currentEnd = now;
      currentBegin = now - defaultRangeS;
    } else if (currentBegin === -1) {
      console.warn('ExploreSimpleSk: begin was not initialized.');
      currentBegin = currentEnd - defaultRangeS;
    } else if (currentEnd === -1) {
      console.warn('ExploreSimpleSk: end was not initialized.');
      currentEnd = currentBegin + defaultRangeS;
    }

    // Ensure end is not in the future.
    if (currentEnd > now) {
      currentEnd = now;
    }

    // Ensure end is not before begin.
    if (currentEnd <= currentBegin) {
      currentEnd = currentBegin + defaultRangeS;
      if (currentEnd > now) {
        currentEnd = now;
        if (currentBegin >= currentEnd) {
          currentBegin = currentEnd - defaultRangeS;
        }
      }
    }

    state.begin = currentBegin;
    state.end = currentEnd;
    return state;
  }

  // Transfer trace name to test path that used for bisect job parameter
  private constructTestPath(traceName: string) {
    // Extract story (subtest) information
    const params: { [key: string]: string } = fromKey(traceName);
    this.story = this.getLastSubtest(params);

    // Construct the testPath
    const parts: string[] = [];
    if (params.master) {
      parts.push(params.master);
    }
    if (params.bot) {
      parts.push(params.bot);
    }
    if (params.benchmark) {
      parts.push(params.benchmark);
    }
    if (params.test) {
      parts.push(params.test);
    }
    // Only add subtests if they are not the same as story, up to 4.
    if (params.subtest_1 && params.subtest_1 !== this.story) {
      parts.push(params.subtest_1);
    }
    if (params.subtest_2 && params.subtest_2 !== this.story) {
      parts.push(params.subtest_2);
    }
    if (params.subtest_3 && params.subtest_3 !== this.story) {
      parts.push(params.subtest_3);
    }
    if (params.subtest_4 && params.subtest_4 !== this.story) {
      parts.push(params.subtest_4);
    }

    parts.push(this.story);
    this.testPath = parts.join('/');
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
  private paramsetKeyCheckboxClick(e: CustomEvent<ParamSetSkKeyCheckboxClickEventDetail>) {
    this.googleChartPlot.value?.updateChartForParam(
      e.detail.key,
      e.detail.values,
      e.detail.selected
    );
  }

  /**
   * Handler for the event when the paramset checkbox is clicked.
   * @param e Checkbox click event
   */
  private paramsetCheckboxClick(e: CustomEvent<ParamSetSkCheckboxClickEventDetail>) {
    this.googleChartPlot.value?.updateChartForParam(
      e.detail.key,
      [e.detail.value],
      e.detail.selected
    );
  }

  private AddPlotLines() {
    const dt = this.dfRepo.value?.data;
    const df = this.dfRepo.value?.dataframe;
    const shouldAddSummaryOptions = this._state.plotSummary && df !== undefined && dt !== undefined;
    if (shouldAddSummaryOptions) {
      this.addPlotSummaryOptions(df, dt);
    }
  }

  private getTraces(dt: DataTable): string[] {
    // getLegend returns trace Ids which are not common in all the graphs.
    // Since it is an object we convert it to the standard key format a=A,b=B,
    // so that it could be fed to the traceFormatter.
    //
    // Also note that some values in the legend object has "untitled_key" as value which is
    // used to signify a trace not having any values for a particular param.
    // We ignore this.
    const shortTraceIds: string[] = [];
    getLegend(dt).forEach((traceObject: object) => {
      let shortTraceId = '';
      const to = traceObject as { [key: string]: string | number };
      Object.keys(to).forEach((k) => {
        const v = String(to[k]);
        shortTraceId += v !== 'untitled_key' ? `${k}=${v},` : '';
      });

      // Add an extra comma at the beginning to make sure
      // the key is in standard ,a=1,b=2,c=3, format
      if (shortTraceId !== '') {
        shortTraceId = `,${shortTraceId}`;
      }

      let formattedShortTrace = '';
      if (shortTraceId !== '') {
        formattedShortTrace = this.traceFormatter!.formatTrace(fromKey(shortTraceId));
      }

      // Since this text is just the uncommon part of trace Id we want to remove
      // the label from the formatted trace id so the user doesn't get confused.
      formattedShortTrace = formattedShortTrace.replace('Trace ID: ', '');
      shortTraceIds.push(formattedShortTrace);
    });
    return shortTraceIds;
  }

  /**
   * Adds the option list for the plot summary selection.
   * @param df The dataframe object from dataframe context.
   * @param df The dataTable object from dataframe context.
   */
  private addPlotSummaryOptions(df: DataFrame, dt: DataTable) {
    if (!this.summaryOptionsField.value) {
      return;
    }

    if (isSingleTrace(dt)) {
      this.summaryOptionsField.value!.style.display = 'none';
      return;
    }

    const titleObj = getTitle(dt);
    let commonTitle = '';
    for (const [key, value] of Object.entries(titleObj)) {
      commonTitle += `${key}=${value},`;
    }
    // Add an extra comma at the beginning to make sure
    // the key is in standard ,a=1,b=2,c=3, format
    if (commonTitle !== '') {
      commonTitle = `,${commonTitle}`;
    }

    const shortTraceIds: string[] = this.getTraces(dt);
    const displayOptions: string[] = [];
    const traceIds = Object.keys(df.traceset);
    shortTraceIds.forEach((shortTraceId, i) => {
      const traceId = traceIds[i];
      const op = `...${shortTraceId}`;
      // These maps are useful to find summary drop down options from the trace key
      // and vice versa. The trace key's format is constant across the tool.
      // Hence these maps come in handy for that purpose. They're used primarily
      // when the summary drop down changes and when a trace is selected from
      // the graph.
      this.summaryOptionTraceMap.set(op, traceId);
      this.traceIdSummaryOptionMap.set(traceId, op);
      displayOptions.push(op);
    });

    const formattedTitle = this.traceFormatter!.formatTrace(fromKey(commonTitle));
    this.summaryOptionsField.value!.helperText = formattedTitle;
    this.summaryOptionsField.value!.options = displayOptions;
    this.summaryOptionsField.value!.label = 'Legend ';
  }

  private paramsetKeyValueClick(e: CustomEvent<ParamSetSkClickEventDetail>) {
    const keys: string[] = [];
    Object.keys(this._dataframe.traceset).forEach((key) => {
      if (_matches(key, e.detail.key, e.detail.value!)) {
        keys.push(key);
      }
    });

    this.render();
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
      disable_filter_parent_traces:
        this._state.disable_filter_parent_traces ||
        this._state.queries.some((q) => q.includes(MISSING_VALUE_SENTINEL)),
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
    if (this.commitTime) {
      this.commitTime!.textContent = '';
    }
    const switchToTab = body.formulas!.length > 0 || body.queries!.length > 0 || body.keys !== '';
    this.requestFrame(body)
      .then(async (json) => {
        if (json === null || json === undefined) {
          errorMessage('Failed to find any matching traces.');
          return;
        }
        return await this.UpdateWithFrameResponse(json, body, switchToTab, null, true, true);
      })
      .catch((msg) => {
        if (msg instanceof DataServiceError) {
          errorMessage(msg.message);
        } else {
          errorMessage(msg);
        }
      });
  }

  /**
   * UpdateWithFrameResponse updates the explore element with the given frame response.
   * @param frameResponse FrameResponse object containing the data frame.
   * @param frameRequest Frame Request object containing the corresponding backend request.
   * @param switchToTab Whether switch should be done.
   */
  public async UpdateWithFrameResponse(
    frameResponse: FrameResponse,
    frameRequest: FrameRequest | null,
    switchToTab: boolean,
    selectedRange: range | null = null,
    extendRange: boolean = true,
    replaceAnomalies: boolean = false
  ): Promise<void> {
    this.render();
    if (
      frameResponse.dataframe?.traceset &&
      Object.keys(frameResponse.dataframe.traceset).length === 0
    ) {
      errorMessage('No data found for the given query.');
      return;
    }
    const dfRepo = this.dfRepo.value;
    if (!dfRepo) {
      console.error('DataFrameRepository is not available.');
      return;
    }
    await this.dfRepo.value?.resetWithDataframeAndRequest(
      frameResponse.dataframe!,
      frameResponse.anomalymap,
      frameRequest!,
      replaceAnomalies
    );
    // Code previously in .then() now runs after await.
    const loadingMore: Promise<void> = this.addTraces(
      frameResponse,
      switchToTab,
      selectedRange,
      extendRange
    );
    this.updateTracePointMetadata(frameResponse.dataframe!.traceMetadata);
    this.updateTitle();
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
    this.render();
    return await loadingMore;
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
  private async addTraces(
    json: FrameResponse,
    tab: boolean,
    selectedRange: range | null = null,
    loadExtendedRange: boolean = true
  ) {
    const dataframe = json.dataframe!;
    if (dataframe.traceset === null || Object.keys(dataframe.traceset).length === 0) {
      this.displayMode = 'display_query_only';
      return;
    }

    if (dataframe.header!.length * Object.keys(dataframe.traceset).length > DATAPOINT_THRESHOLD) {
      errorMessage('Large amount of data requsted, performance may be affected.', 2000);
    }
    this.tracesRendered = true;
    this.displayMode = json.display_mode;

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

    const header = dataframe.header;
    if (selectedRange === null) {
      const prop = this._state.domain === 'date' ? 'timestamp' : 'offset';
      selectedRange = range(header![0]![prop], header![header!.length - 1]![prop]);
    }

    this.updateSelectedRangeWithUpdatedDataframe(
      selectedRange!,
      this._state.domain as 'commit' | 'date'
    );
    // Normalize bands to be just offsets.
    const bands: number[] = [];
    header!.forEach((h, i) => {
      if (json.skps!.indexOf(h!.offset) !== -1) {
        bands.push(i);
      }
    });
    const googleChart = this.googleChartPlot.value;
    if (googleChart) {
      // Populate the xbar if present.
      if (this.state.xbaroffset !== -1) {
        googleChart.xbar = this.state.xbaroffset;
      }
      googleChart.domain = this.state.domain as 'date' | 'commit';
    }
    if (this.state.use_titles) {
      this.updateTitle();
    }

    // Populate the paramset element.
    if (this.paramset) {
      this.paramset!.paramsets = [dataframe.paramset as CommonSkParamSet];
    }

    if (tab && this.detailTab) {
      this.detailTab!.selected = PARAMS_TAB_INDEX;
      // Asynchronously fetch the user issues for the rendered traces.
      this.dfRepo.value
        ?.getUserIssues(Object.keys(dataframe.traceset), selectedRange.begin, selectedRange.end)
        .then((_: UserIssueMap) => {
          this.updateSelectedRangeWithUpdatedDataframe(
            selectedRange!,
            this.state.domain as 'date' | 'commit'
          );
        });
    }
    this._renderedTraces();
    if (this._state.plotSummary) {
      if (this.state.doNotQueryData || !loadExtendedRange) {
        this.render();
        // The data is supposed to be already loaded.
        // Let's simply make the selection on the summary.
        const updatedRange = this.extendRangeToMinimumAllowed(header, selectedRange!);
        this.plotSummary.value?.SelectRange(updatedRange);
      } else {
        return await this.loadExtendedRangeData(selectedRange);
      }
    }
  }

  // Extend the selected range to include at least the minimum number of points.
  // Checks to ensure enough points exist, if not, then extends the range to the
  // total MIN_POINTS points in the header.
  private extendRangeToMinimumAllowed(
    header: string | (ColumnHeader | null)[] | null,
    selectedRange: { begin: number; end: number }
  ) {
    // Ensure at least the Minimum Points are displayed.
    if (header && header.length < MIN_POINTS) {
      const repoHeader = this.dfRepo.value?.header;
      // Check to ensure enough point exist.
      if (repoHeader && repoHeader.length > MIN_POINTS) {
        selectedRange = range(
          repoHeader[repoHeader.length - MIN_POINTS]!.offset,
          repoHeader[repoHeader.length - 1]!.offset
        );
      }
    }
    return selectedRange;
  }

  /**
   * Loads extended range data for the plot summary.
   * @param selectedRange The currently selected range.
   * @returns A Promise that resolves when the extended range data is loaded.
   */
  async loadExtendedRangeData(selectedRange: range): Promise<void> {
    const header = this.dfRepo.value?.header;
    if (!header) {
      return;
    }
    let extendRange = 3 * monthInSec;
    // Large amount of traces, limit the range extension.
    if (this.dfRepo.value?.traces && Object.keys(this.dfRepo.value.traces).length > 10) {
      extendRange = monthInSec as CommitNumber;
    }
    const promises = [
      this.dfRepo.value?.extendRange(-extendRange).then(() => {
        const updatedRange = this.extendRangeToMinimumAllowed(header, selectedRange!);
        // Already plotted, just need to update the data.
        this.updateSelectedRangeWithUpdatedDataframe(
          updatedRange,
          this.state.domain as 'date' | 'commit',
          false
        );
        this.plotSummary.value?.SelectRange(updatedRange);
      }),
      this.dfRepo.value?.extendRange(extendRange).then(() => {
        const updatedRange = this.extendRangeToMinimumAllowed(header, selectedRange!);
        this.updateSelectedRangeWithUpdatedDataframe(
          updatedRange,
          this.state.domain as 'date' | 'commit',
          false
        );
        this.plotSummary.value?.SelectRange(updatedRange);
        // Modify the Range if URL contains different values.
        this.useBrowserURL();
      }),
    ];
    await Promise.allSettled(promises);
    this.dataLoading = false;
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
    let j = this.json!.value;
    if (plotType === 'json' && j === '') {
      // If JSON is empty, check URL.
      const currentUrl = new URL(window.location.href);
      const jsonParam = currentUrl.searchParams.get('json');
      if (jsonParam) {
        j = jsonParam;
        // Populate the text area so the user sees it.
        this.json!.value = j;
      }
    }
    this.addFromQueryOrFormula(replace, plotType, q, f, j);
  }

  /** Create a FrameRequest for just a single query or formula. */
  private requestFrameBodyForTrace(
    plotType: addPlotType,
    q: string,
    f: string,
    j: string
  ): FrameRequest {
    let formulas: string[] = [];
    let queries: string[] = [];
    let keys = '';

    if (plotType === 'formula') {
      formulas = [f];
    } else if (plotType === 'query') {
      queries = [q];
    } else if (plotType === 'json') {
      const graphConfig = JSON.parse(j);
      // TODO(b/381139433): Support multiple graphs.
      if (graphConfig.graphs && graphConfig.graphs.length > 0) {
        queries = graphConfig.graphs[0].queries || [];
        formulas = graphConfig.graphs[0].formulas || [];
        keys = graphConfig.graphs[0].keys || '';
      }
    }

    return {
      begin: this._state.begin,
      end: this._state.end,
      num_commits: this._state.numCommits,
      request_type: this._state.requestType,
      formulas: formulas,
      queries: queries,
      keys: keys,
      tz: Intl.DateTimeFormat().resolvedOptions().timeZone,
      pivot: null,
      disable_filter_parent_traces:
        this._state.disable_filter_parent_traces ||
        queries.some((q) => q.includes(MISSING_VALUE_SENTINEL)),
    };
  }

  /* Plot the traces that match either the given query or the given formula,
   * depending on the value of plotType.
   *
   * @param replace - If true then replace all the traces with ones that match
   * this query, otherwise add them to the current traces being displayed.
   *
   * @param plotType - The type of traces being added.
   */
  async addFromQueryOrFormula(
    replace: boolean,
    plotType: addPlotType,
    q: string,
    f: string,
    j: string = '',
    loadExtendedRange: boolean = true
  ): Promise<void> {
    if (this.queryDialog !== null) {
      this.queryDialog!.close();
    }
    this._dialogOn = false;
    this.dataLoading = true;

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
    } else if (plotType === 'json') {
      if (j.trim() === '') {
        errorMessage('The JSON must not be empty.');
        return;
      }
      try {
        JSON.parse(j);
      } catch (e) {
        errorMessage(`Invalid JSON: ${e}`);
        return;
      }
    } else {
      errorMessage('Unknown plotType');
      return;
    }
    if (this.range !== null && !this.useTestPicker) {
      // Only lengthen the time range, do not shorten.
      if (this.range.state.begin < this._state.begin) {
        this._state.begin = this.range.state.begin;
      }
      if (this.range.state.end > this._state.end) {
        this._state.end = this.range.state.end;
      }
      this._state.numCommits = this.range!.state.num_commits;
      this._state.requestType = this.range!.state.request_type;
    }
    this._state.sort = '';
    const createFromScratch =
      replace ||
      plotType === 'pivot' ||
      plotType === 'json' ||
      (this._state.formulas.length === 0 && this._state.queries.length === 0);

    if (createFromScratch) {
      this.removeAll(true);
    }

    this._state.pivotRequest = defaultPivotRequest();
    if (plotType === 'query') {
      if (this._state.queries.length === 1 && this._state.queries[0] !== q) {
        this._state.queries = [];
      }
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
    } else if (plotType === 'json') {
      const graphConfig = JSON.parse(j);
      // TODO(b/381139433): Support multiple graphs.
      if (graphConfig.graphs && graphConfig.graphs.length > 0) {
        this._state.queries = graphConfig.graphs[0].queries || [];
        this._state.formulas = graphConfig.graphs[0].formulas || [];
        this._state.keys = graphConfig.graphs[0].keys || '';
      }
    }

    this.applyQueryDefaultsIfMissing();
    this._stateHasChanged();
    this.updateTestPickerUrl();

    try {
      if (createFromScratch) {
        const body = this.requestFrameBodyFullFromState();
        return this.requestFrame(body).then(async (json) => {
          if (json) {
            return await this.UpdateWithFrameResponse(
              json,
              body,
              /*switchToTab=*/ true,
              this.getSelectedRange(),
              loadExtendedRange,
              true
            );
          }
        });
      } else {
        const requestNewData = this.requestFrameBodyForTrace(plotType, q, f, j);
        const requestAllData = this.requestFrameBodyFullFromState();
        return this.requestFrame(requestNewData).then(async (json) => {
          if (!json) {
            return;
          }
          // Merge new data with the existing state.
          const frameResponse = json as FrameResponse;
          const newDf = frameResponse.dataframe!;
          const mergedDataFrame = joinDataframes(this._dataframe, newDf);
          frameResponse.dataframe = mergedDataFrame;
          // Don't merge anomalies because DataFrameRepository already does that.
          return await this.UpdateWithFrameResponse(
            frameResponse,
            requestAllData,
            /*switchToTab=*/ true,
            this.getSelectedRange(),
            loadExtendedRange
          );
        });
      }
    } catch (error) {
      // errorMessage is likely already called by requestFrame or its callees.
      console.error('Error in addFromQueryOrFormula during requestFrame:', error);
    }
  }

  /**
   * updateTracePointMetadata populates the commit links from the trace metadata
   * in the response.
   */
  private async updateTracePointMetadata(traceMetadatas: TraceMetadata[] | null) {
    if (traceMetadatas === null) {
      return;
    }

    // Use a Map for efficient lookups of existing commit links by a composite key.
    // The key will be a string like "cid-traceid".
    const uniqueLinksMap = new Map<string, CommitLinks>();

    // Populate the map with links already present in the class member this.commitLinks.
    // This ensures we don't add duplicates from previous updates.
    for (const link of this.commitLinks) {
      if (link === null) continue;
      const identifier = `${link.cid}-${link.traceid}`;
      uniqueLinksMap.set(identifier, link);
    }
    // Iterate through the new traceMetadatas.
    for (const traceMetadata of traceMetadatas) {
      if (traceMetadata.commitLinks !== null) {
        // Iterate through the commit links within the current traceMetadata.
        for (const commitnumStr in traceMetadata.commitLinks) {
          const commitnum = parseInt(commitnumStr);
          const traceid = traceMetadata.traceid; // traceid is directly on traceMetadata

          const identifier = `${commitnum}-${traceid}`;

          // Check if this specific link (cid-traceid pair) already exists.
          if (!uniqueLinksMap.has(identifier)) {
            const displayUrls: { [key: string]: string } = {};
            const displayTexts: { [key: string]: string } = {};
            // Populate displayUrls and displayTexts from the current commit's links.
            const linksForCommit = traceMetadata.commitLinks[commitnum];
            if (linksForCommit) {
              // Ensure linksForCommit is not null/undefined
              for (const linkKey in linksForCommit) {
                const linkObj = linksForCommit[linkKey];
                displayTexts[linkKey] = linkObj.Text;
                displayUrls[linkKey] = linkObj.Href;
              }
            }
            const commitLink: CommitLinks = {
              cid: commitnum,
              traceid: traceid,
              displayUrls: displayUrls,
              displayTexts: displayTexts,
            };

            // Add the new unique link to the map.
            uniqueLinksMap.set(identifier, commitLink);
          }
        }
      }
    }
    // Replace the class member with the newly built unique list from the map's values.
    this.commitLinks = Array.from(uniqueLinksMap.values());
  }

  // take a query string, and update the parameters with default values if needed
  private applyDefaultsToQuery(queryString: string): string {
    const paramSet = toParamSet(queryString);
    let defaultParams = this._defaults?.default_param_selections ?? {};
    if (window.perf.remove_default_stat_value) {
      defaultParams = {};
    }
    for (const defaultParamKey in defaultParams) {
      if (!(defaultParamKey in paramSet)) {
        paramSet[defaultParamKey] = defaultParams![defaultParamKey]!;
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
          case 'disable_filter_parent_traces':
            this._state.disable_filter_parent_traces = paramValue;
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
    this._dataframe.header = [];
    this._dataframe.traceset = TraceSet({});
    if (this.graphTitle) {
      this.graphTitle.set(null, 0);
    }
    if (this.paramset) {
      this.paramset.paramsets = [];
    }
    if (this.commitTime) {
      this.commitTime.textContent = '';
    }
    if (this.detailTab) {
      this.detailTab.selected = PARAMS_TAB_INDEX;
    }
    this.displayMode = 'display_query_only';
    this.tracesRendered = false;
    this.commitLinks = [];
    this.tooltipSelected = false;

    this.dfRepo.value?.clear();
    this.closeTooltip();

    this.render();
    if (!skipHistory) {
      this.clearSelectedState();
      this._stateHasChanged();
    }
    this.updateTestPickerUrl();
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
    return this.dataService
      .createShortcut(state)
      .then((json) => {
        this._state.keys = json.id;
        this._state.queries = [];
        this.clearSelectedState();
        this._stateHasChanged();
        this.render();
      })
      .catch((msg) => {
        if (msg instanceof DataServiceError) {
          errorMessage(msg.message);
        } else {
          errorMessage(msg);
        }
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
    try {
      const newShortcut = await updateShortcut(graphConfigs);

      if (newShortcut === '') {
        return;
      }

      window.open(
        `/m/?begin=${this._state.begin}&end=${this._state.end}` +
          `&pageSize=${chartsPerPage}&shortcut=${newShortcut}` +
          `&totalGraphs=${graphConfigs.length}` +
          `&show_google_plot=true`,
        '_self'
      );
    } catch (msg) {
      if (msg instanceof DataServiceError && msg.status === 500) {
        errorMessage('Unable to update shortcut.', 2000);
      } else if (msg instanceof DataServiceError) {
        errorMessage(msg.message);
      } else {
        errorMessage(msg as any);
      }
    }
  }

  removeKeys(keysToRemove: string[], updateShortcut: boolean) {
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
      if (this.dfRepo.value?.dataframe.traceset[key] !== undefined) {
        delete this.dfRepo.value?.dataframe.traceset[key];
      }
    });

    if (!this.hasData()) {
      this.displayMode = 'display_query_only';
      this.render();
    }
    if (updateShortcut) {
      this.reShortCut(toShortcut);
    }
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
      const uniqueValues = new Set(
        Object.keys(traceset).map((traceId) => {
          const val = fromKey(traceId)[param];
          // If the value is missing or empty, map it to 'Default' so it is accounted for.
          return val === undefined || val === '' ? 'Default' : val;
        })
      );
      let value = uniqueValues.values().next().value;
      if (uniqueValues.size > 1) {
        value = 'Various';
      }

      if (value !== undefined) {
        titleEntries.set(param, value);
      }
    });

    if (titleEntries.size >= 3) {
      this.graphTitle!.set(titleEntries, numTraces);
    } else {
      this.graphTitle!.set(new Map(), numTraces);
    }
  }

  /** @prop spinning - True if we are waiting to retrieve data from
   * the server.
   */
  set spinning(b: boolean) {
    this._spinning = b;
    if (b) {
      this.displayMode = 'display_spinner';
    }
    this.render();
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
   * @returns A Promise that resolves with the decoded JSON body
   * of the response once it's available.
   */
  private async requestFrame(body: FrameRequest): Promise<FrameResponse | null> {
    if (this._requestId !== '') {
      const err = new Error('There is a pending query already running.');
      errorMessage(err.message);
      return null;
    }
    this._requestId = 'foo';
    this.spinning = true;
    this.dataLoading = true;
    try {
      const jsonResponse = await this.sendFrameRequest(body);
      return jsonResponse;
    } catch (err: unknown) {
      // This catches errors from sendFrameRequest (e.g., status !== 'Finished')
      if (this.percent) {
        this.percent.textContent = '';
      }
      this.spinning = false;
      errorMessage(err as string);
      return null;
    } finally {
      // Ensuring the state is always cleaned up.
      this.spinning = false;
      this.dataLoading = false;
      this._requestId = '';
    }
  }

  /**
   * Starts the frame request and returns the resulting data.
   */
  private async sendFrameRequest(body: FrameRequest): Promise<FrameResponse> {
    return await this.dataService.sendFrameRequest(body, {
      onProgress: (prog: string) => {
        if (this.isConnected && this.percent !== null) {
          this.percent!.textContent = prog;
        }
      },
      onMessage: (msg: string) => {
        errorMessage(msg);
      },
      onStart: () => {
        if (this.isConnected && this.spinner) {
          this.spinner.active = true;
        }
      },
      onSettled: () => {
        if (this.isConnected && this.spinner) {
          this.spinner.active = false;
        }
      },
    });
  }

  get state(): State {
    return this._state;
  }

  set state(state: State) {
    const prevBegin = this._state?.begin;
    const prevEnd = this._state?.end;
    const prevRequestType = this._state?.requestType;
    const prevQueries = [...(this._state?.queries || [])];
    const prevFormulas = [...(this._state?.formulas || [])];
    const prevKeys = this._state?.keys;

    state = this.rationalizeTimeRange(state);

    this._state = state;

    // Synchronize the xAxisSwitch with the state's domain
    const isDateDomain = this._state.domain === 'date';
    if (this.xAxisSwitch !== isDateDomain) {
      this.xAxisSwitch = isDateDomain;
    }

    if (this.range) {
      this.range!.state = {
        begin: this._state.begin,
        end: this._state.end,
        num_commits: this._state.numCommits,
        request_type: this._state.requestType,
      };
    }

    this.render();

    // If there is at least one query, the use the last one to repopulate the
    // query-sk dialog.
    const numQueries = this._state.queries.length;
    if (numQueries >= 1 && this.query) {
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

    const encodedPrevQueries = prevQueries.map((q) => encodeURIComponent(q));
    const encodedCurrentQueries = this._state.queries.map((q) => encodeURIComponent(q));
    const queriesChanged =
      encodedPrevQueries.length !== encodedCurrentQueries.length ||
      encodedPrevQueries.some((q, i) => q !== encodedCurrentQueries[i]);
    const formulasChanged =
      prevFormulas.length !== this._state.formulas.length ||
      prevFormulas.some((f, i) => f !== this._state.formulas[i]);
    if (
      prevBegin !== this._state.begin ||
      prevEnd !== this._state.end ||
      prevRequestType !== this._state.requestType ||
      queriesChanged ||
      formulasChanged ||
      prevKeys !== this._state.keys
    ) {
      this.updateTestPickerUrl();
    }

    if (!state.doNotQueryData) {
      this.rangeChangeImpl();
    }
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
    this.render();
  }

  getTraceset(): { [key: string]: number[] } | null {
    return this.dfRepo.value?.dataframe.traceset ?? null;
  }

  getDataTraces(): { [key: string]: number[] } | null {
    return this._dataframe.traceset ?? null;
  }

  getSelectedRange(): range | null {
    return this.googleChartPlot.value?.selectedRange ?? null;
  }

  getHeader(): (ColumnHeader | null)[] | null {
    if (this.dfRepo.value) {
      return this.dfRepo.value!.header;
    }
    return null;
  }

  getCommitLinks(): (CommitLinks | null)[] {
    return this.commitLinks;
  }

  getAnomalyMap(): AnomalyMap {
    const anomalies = this.dfRepo.value?.getAllAnomalies();
    return anomalies ?? {};
  }

  getParamSet(): { [key: string]: string[] } {
    return this.query?.paramset ?? {};
  }

  set defaults(val: QueryConfig | null) {
    this._defaults = val;
  }

  public static getTraceMetadataFromCommitLinks(
    traces: string[],
    commitLinks: (CommitLinks | null)[]
  ): TraceMetadata[] {
    const traceMetadata: TraceMetadata[] = [];
    traces.forEach((trace) => {
      const metadata: TraceMetadata = {
        traceid: trace,
        commitLinks: {},
      };
      commitLinks.forEach((link) => {
        if (link?.traceid === trace) {
          const linksForCommit: { [key: string]: TraceCommitLink } = {};
          if (link.displayTexts) {
            for (const key in link.displayUrls) {
              linksForCommit[key] = { Text: link.displayTexts[key], Href: link.displayUrls[key] };
            }
          }
          metadata.commitLinks![link.cid] = linksForCommit;
        }
      });
      traceMetadata.push(metadata);
    });
    return traceMetadata;
  }

  private getDefaultRange(): number {
    if (this._defaults && this._defaults.default_range) {
      return this._defaults.default_range;
    }

    return DEFAULT_RANGE_S;
  }

  public clearHighlightedCircles() {
    this._state.highlight_anomalies = [];
    this._stateHasChanged();
    this.render();
  }

  public clearAnomalyMap() {
    this.dfRepo.value?.clearAnomalyMap();
    this.clearHighlightedCircles();
  }
}

define('explore-simple-sk', ExploreSimpleSk);
