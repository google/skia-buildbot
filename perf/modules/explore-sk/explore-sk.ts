/**
 * @module module/explore-sk
 * @description <h2><code>explore-sk</code></h2>
 *
 * Main page of Perf, for exploring data.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { toParamSet } from 'common-sk/modules/query';
import dialogPolyfill from 'dialog-polyfill';
import { TabsSk } from 'elements-sk/tabs-sk/tabs-sk';
import { ParamSet as CommonSkParamSet } from 'common-sk/modules/query';
import { HintableObject } from 'common-sk/modules/hintable';
import { SpinnerSk } from 'elements-sk/spinner-sk/spinner-sk';
import { errorMessage } from '../errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import 'elements-sk/checkbox-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/spinner-sk';
import 'elements-sk/styles/buttons';
import 'elements-sk/tabs-panel-sk';
import 'elements-sk/tabs-sk';

import '../../../infra-sk/modules/query-sk';
import '../../../infra-sk/modules/paramset-sk';

import '../commit-detail-panel-sk';
import '../domain-picker-sk';
import '../json-source-sk';
import '../ingest-file-links-sk';
import '../plot-simple-sk';
import '../query-count-sk';
import '../window/window';

import {
  DataFrame,
  RequestType,
  ParamSet,
  FrameRequest,
  FrameResponse,
  ShiftRequest,
  ShiftResponse,
  progress,
} from '../json';
import {
  PlotSimpleSk,
  PlotSimpleSkTraceEventDetails,
} from '../plot-simple-sk/plot-simple-sk';
import { CommitDetailPanelSk } from '../commit-detail-panel-sk/commit-detail-panel-sk';
import { JSONSourceSk } from '../json-source-sk/json-source-sk';
import {
  ParamSetSk,
  ParamSetSkClickEventDetail,
} from '../../../infra-sk/modules/paramset-sk/paramset-sk';
import {
  QuerySk,
  QuerySkQueryChangeEventDetail,
} from '../../../infra-sk/modules/query-sk/query-sk';
import { QueryCountSk } from '../query-count-sk/query-count-sk';
import { DomainPickerSk } from '../domain-picker-sk/domain-picker-sk';
import { MISSING_DATA_SENTINEL } from '../plot-simple-sk/plot-simple-sk';
import { messageByName, messagesToErrorString, startRequest } from '../progress/progress';
import { IngestFileLinksSk } from '../ingest-file-links-sk/ingest-file-links-sk';

// The trace id of the zero line, a trace of all zeros.
const ZERO_NAME = 'special_zero';

// How often to refresh if the auto-refresh checkmark is checked.
const REFRESH_TIMEOUT = 30 * 1000; // milliseconds

// The default query range in seconds.
const DEFAULT_RANGE_S = 24 * 60 * 60; // 2 days in seconds.

// The index of the params tab.
const PARAMS_TAB_INDEX = 0;

// The index of the commit detail info tab.
const COMMIT_TAB_INDEX = 1;

// The percentage of the current zoom window to pan or zoom on a keypress.
const ZOOM_JUMP_PERCENT = 0.1;

// When we are zooming around and bump into the edges of the graph, how much
// should we widen the range of commits, as a percentage of the currently
// displayed commit.
const RANGE_CHANGE_ON_ZOOM_PERCENT = 0.5;

// The minimum length [right - left] of a zoom range.
const MIN_ZOOM_RANGE = 0.1;

type RequestFrameCallback = (frameResponse: FrameResponse)=> void;

// State is reflected to the URL via stateReflector.
class State {
  begin: number = Math.floor(Date.now() / 1000 - DEFAULT_RANGE_S);

  end: number = Math.floor(Date.now() / 1000);

  formulas: string[] = [];

  queries: string[] = [];

  keys: string = ''; // The id of the shortcut to a list of trace keys.

  xbaroffset: number = -1; // The offset of the commit in the repo.

  showZero: boolean = true;

  dots: boolean = true; // Whether to show dots when plotting traces.

  autoRefresh: boolean = false;

  numCommits: number = 50;

  requestType: RequestType = 1; // TODO(jcgregorio) Use constants in domain-picker-sk.
}

// TODO(jcgregorio) Move to a 'key' module.
// Returns true if paramName=paramValue appears in the given structured key.
function _matches(key: string, paramName: string, paramValue: string): boolean {
  return key.indexOf(`,${paramName}=${paramValue},`) >= 0;
}

// TODO(jcgregorio) Move to a 'key' module.
// Parses the structured key and returns a populated object with all
// the param names and values.
function toObject(key: string): { [key: string]: string } {
  const ret: { [key: string]: string } = {};
  key.split(',').forEach((s, i) => {
    if (i === 0) {
      return;
    }
    if (s === '') {
      return;
    }
    const parts = s.split('=');
    if (parts.length !== 2) {
      return;
    }
    ret[parts[0]] = parts[1];
  });
  return ret;
}

interface RangeChange {
  /**
   * If true then do a range change with the provided offsets, otherwise just
   * do a zoom.
   */
  rangeChange: boolean;

  newOffsets?: [number, number];
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
  zoom: [number, number],
  clampedZoom: [number, number],
  offsets: [number, number],
): RangeChange {
  // How much we will change the offset if we zoom beyond an edge.
  const offsetDelta = Math.floor(
    (offsets[1] - offsets[0]) * RANGE_CHANGE_ON_ZOOM_PERCENT,
  );
  const exceedsLeftEdge = zoom[0] !== clampedZoom[0];
  const exceedsRightEdge = zoom[1] !== clampedZoom[1];
  if (exceedsLeftEdge && exceedsRightEdge) {
    // shift both
    return {
      rangeChange: true,
      newOffsets: [
        clampToNonNegative(offsets[0] - offsetDelta),
        offsets[1] + offsetDelta,
      ],
    };
  } if (exceedsLeftEdge) {
    // shift left
    return {
      rangeChange: true,
      newOffsets: [clampToNonNegative(offsets[0] - offsetDelta), offsets[1]],
    };
  } if (exceedsRightEdge) {
    // shift right
    return {
      rangeChange: true,
      newOffsets: [offsets[0], offsets[1] + offsetDelta],
    };
  }
  return {
    rangeChange: false,
  };
}

export class ExploreSk extends ElementSk {
  private _dataframe: DataFrame = {
    traceset: {},
    header: [],
    paramset: {},
    skip: 0,
  };

  // The state that does into the URL.
  private state = new State();

  // Are we waiting on data from the server.
  private _spinning: boolean = false;

  // The id of the current frame request. Will be the empty string if there
  // is no pending request.
  private _requestId = '';

  private _numShift = window.sk.perf.num_shift;

  // The id of the interval timer if we are refreshing.
  private _refreshId = -1;

  // All the data converted into a CVS blob to download.
  private _csvBlobURL: string = '';

  private _initialized: boolean = false;

  private commits: CommitDetailPanelSk | null = null;

  private commitsTab: HTMLButtonElement | null = null;

  private detailTab: TabsSk | null = null;

  private formula: HTMLTextAreaElement | null = null;

  private jsonsource: JSONSourceSk | null = null;

  private ingestFileLinks: IngestFileLinksSk | null = null;

  private paramset: ParamSetSk | null = null;

  private percent: HTMLSpanElement | null = null;

  private plot: PlotSimpleSk | null = null;

  private query: QuerySk | null = null;

  private queryCount: QueryCountSk | null = null;

  private range: DomainPickerSk | null = null;

  private simpleParamset: ParamSetSk | null = null;

  private spinner: SpinnerSk | null = null;

  private summary: ParamSetSk | null = null;

  private traceID: HTMLSpanElement | null = null;

  private csvDownload: HTMLAnchorElement | null = null;

  private queryDialog: HTMLDialogElement | null = null;

  private helpDialog: HTMLDialogElement | null = null;

  constructor() {
    super(ExploreSk.template);
  }

  private static template = (ele: ExploreSk) => html`
  <div id=buttons>
    <button @click=${ele.openQuery}>Query</button>
    <div id=traceButtons ?hide_if_no_data=${!ele.hasData()}>
      <button
        @click=${() => ele.removeAll(false)}
        title='Remove all the traces.'>
        Remove All
      </button>

      <button
        @click=${ele.removeHighlighted}
        ?hidden=${!(ele.plot && ele.plot!.highlight.length)}
        title='Remove all the highlighted traces.'>
        Remove Highlighted
      </button>

      <button
        @click=${ele.highlightedOnly}
        ?hidden=${!(ele.plot && ele.plot!.highlight.length)}
        title='Remove all but the highlighted traces.'>
        Highlighted Only
      </button>

      <span
        title='Number of commits skipped between each point displayed.'
        ?hidden=${ele.isZero(ele._dataframe.skip)}
        id=skip>
          ${ele._dataframe.skip}
      </span>
      <checkbox-sk
        name=zero
        @change=${ele.zeroChangeHandler}
        ?checked=${ele.state.showZero}
        label='Zero'
        title='Toggle the presence of the zero line.'>
      </checkbox-sk>
      <checkbox-sk
        name=dots
        @change=${ele.toggleDotsHandler}
        ?checked=${ele.state.dots}
        label='Dots'
        title='Toggle the presence of dots at each commit.'>
      </checkbox-sk>
      <checkbox-sk
        name=auto
        @change=${ele.autoRefreshHandler}
        ?checked=${ele.state.autoRefresh}
        label='Auto-refresh'
        title='Auto-refresh the data displayed in the graph.'>
      </checkbox-sk>
      <div
        id=calcButtons
        ?hide_if_no_data=${!ele.hasData()}>
        <button
          @click=${() => ele.applyFuncToTraces('norm')}
          title='Apply norm() to all the traces.'>
          Normalize
        </button>
        <button
          @click=${() => ele.applyFuncToTraces('scale_by_avg')}
          title='Apply scale_by_avg() to all the traces.'>
          Scale By Avg
        </button>
        <button
          @click=${() => { ele.applyFuncToTraces('iqrr'); }}
          title='Apply iqrr() to all the traces.'>
          Remove outliers
        </button>
        <button
          @click=${ele.csv}
          title='Download all displayed data as a CSV file.'>
          CSV
        </button>
        <a href='' target=_blank download='traces.csv' id=csv_download></a>
      </div>
    </div>
  </div>

  <div id=spin-overlay>
    <plot-simple-sk
      summary
      width=1200
      height=400
      id=plot
      ?spinning=${ele.spinning}
      @trace_selected=${ele.traceSelected}
      @zoom=${ele.plotZoom}
      @trace_focused=${ele.plotTraceFocused}
      ?hide_if_no_data=${!ele.hasData()}
      >
    </plot-simple-sk>
    <div id=spin-container ?spinning=${ele.spinning}>
      <spinner-sk id=spinner ?active=${ele.spinning}></spinner-sk>
      <span id=percent></span>
    </div>
  </div>

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
          <paramset-sk id=summary></paramset-sk>
          <div class=query-counts>
            Matches: <query-count-sk url='/_/count/' @paramset-changed=${
              ele.paramsetChanged
            }>
            </query-count-sk>
          </div>
          <button @click=${() => ele.add(true)} class=action>Plot</button>
          <button @click=${() => ele.add(false)}>Add to Plot</button>
        </div>
    </div>
    <details>
      <summary>Time Range</summary>
      <domain-picker-sk id=range>
      </domain-picker-sk>
    </details>

    <details>
      <summary>Calculated Traces</summary>
      <div class=formulas>
        <textarea id=formula rows=3 cols=80></textarea>
        <button @click=${() => ele.addCalculated(true)}>Plot</button>
        <button @click=${() => ele.addCalculated(false)}>Add to Plot</button>
        <a href=/help/ target=_blank>
          <help-icon-sk></help-icon-sk>
        </a>
      </div>
    </details>
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
  </dialog>

    <div id=tabs ?hide_if_no_data=${!ele.hasData()}>
      <tabs-sk id=detailTab>
        <button>Params</button>
        <button id=commitsTab disabled>Details</button>
      </tabs-sk>
      <tabs-panel-sk>
        <div>
          <p>
            <b>Trace ID</b>: <span title='Trace ID' id=trace_id></span>
          </p>
          <paramset-sk
            id=paramset
            clickable_values
            @paramset-key-value-click=${ele.paramsetKeyValueClick}>
            </paramset-sk>
        </div>
        <div id=details>
          <paramset-sk
            id=simple_paramset
            clickable_values
            @paramset-key-value-click=${ele.paramsetKeyValueClick}
            >
          </paramset-sk>
          <div>
            <commit-detail-panel-sk id=commits selectable></commit-detail-panel-sk>
            <ingest-file-links-sk id=ingest-file-links></ingest-file-links-sk>
            <json-source-sk id=jsonsource></json-source-sk>
          </div>
        </div>
      </tabs-panel-sk>
    </div>
  `;

  connectedCallback(): void {
    super.connectedCallback();
    if (this._initialized) {
      return;
    }
    this._initialized = true;
    this._render();

    this.commits = this.querySelector('#commits');
    this.commitsTab = this.querySelector('#commitsTab');
    this.detailTab = this.querySelector('#detailTab');
    this.formula = this.querySelector('#formula');
    this.jsonsource = this.querySelector('#jsonsource');
    this.ingestFileLinks = this.querySelector('#ingest-file-links');
    this.paramset = this.querySelector('#paramset');
    this.percent = this.querySelector('#percent');
    this.plot = this.querySelector('#plot');
    this.query = this.querySelector('#query');
    this.queryCount = this.querySelector('query-count-sk');
    this.range = this.querySelector('#range');
    this.simpleParamset = this.querySelector('#simple_paramset');
    this.spinner = this.querySelector('#spinner');
    this.summary = this.querySelector('#summary');
    this.traceID = this.querySelector('#trace_id');
    this.csvDownload = this.querySelector('#csv_download');
    this.queryDialog = this.querySelector('#query-dialog');
    dialogPolyfill.registerDialog(this.queryDialog!);
    this.helpDialog = this.querySelector('#help');
    dialogPolyfill.registerDialog(this.helpDialog!);

    // Populate the query element.
    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;

    fetch(`/_/initpage/?tz=${tz}`, {
      method: 'GET',
    })
      .then(jsonOrThrow)
      .then((json) => {
        const now = Math.floor(Date.now() / 1000);
        this.state.begin = now - 60 * 60 * 24;
        this.state.end = now;
        this.range!.state = {
          begin: this.state.begin,
          end: this.state.end,
          num_commits: this.state.numCommits,
          request_type: this.state.requestType,
        };

        this.query!.key_order = window.sk.perf.key_order || [];
        this.query!.paramset = json.dataframe.paramset;

        // Remove the paramset so it doesn't get displayed in the Params tab.
        json.dataframe.paramset = {};

        // From this point on reflect the state to the URL.
        this.startStateReflector();
      })
      .catch(errorMessage);

    document.addEventListener('keydown', (e) => this.keyDown(e));
  }

  // Call this anytime something in private state is changed. Will be replaced
  // with the real function once stateReflector has been setup.
  // eslint-disable-next-line @typescript-eslint/no-empty-function
  private _stateHasChanged = () => {};

  private keyDown(e: KeyboardEvent) {
    // Ignore IME composition events.
    if (e.isComposing || e.keyCode === 229) {
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
  private getCurrentZoom() {
    let zoom = this.plot!.zoom;
    if (zoom === null) {
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
      zoom,
      delta,
    };
  }

  /**
   * Clamp a single zoom endpoint.
   */
  private clampZoomIndexToDataFrame(z: number): number {
    if (z < 0) {
      z = 0;
    }
    if (z > this._dataframe.header!.length - 1) {
      z = this._dataframe.header!.length - 1;
    }
    return z;
  }

  /**
   * Fixes up the zoom range so it always make sense.
   *
   * @param {Array<Number>} zoom - The zoom range.
   * @returns {Array<Number>} The zoom range.
   */
  private rationalizeZoom(zoom: [number, number]) {
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
  private zoomOrRangeChange(zoom: [number, number]) {
    zoom = this.rationalizeZoom(zoom);
    const clampedZoom: [number, number] = [
      this.clampZoomIndexToDataFrame(zoom[0]),
      this.clampZoomIndexToDataFrame(zoom[1]),
    ];
    const offsets: [number, number] = [
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
          this.state.begin = json.begin;
          this.state.end = json.end;
          this.state.requestType = 0;
          this._stateHasChanged();
          this.rangeChangeImpl();
        })
        .catch(errorMessage);
    } else {
      this.plot!.zoom = zoom;
    }
  }

  private zoomInKey() {
    const cz = this.getCurrentZoom();
    const zoom: [number, number] = [
      cz.zoom[0] + ZOOM_JUMP_PERCENT * cz.delta,
      cz.zoom[1] - ZOOM_JUMP_PERCENT * cz.delta,
    ];
    this.zoomOrRangeChange(zoom);
  }

  private zoomOutKey() {
    const cz = this.getCurrentZoom();
    const zoom: [number, number] = [
      cz.zoom[0] - ZOOM_JUMP_PERCENT * cz.delta,
      cz.zoom[1] + ZOOM_JUMP_PERCENT * cz.delta,
    ];
    this.zoomOrRangeChange(zoom);
  }

  private zoomLeftKey() {
    const cz = this.getCurrentZoom();
    const zoom: [number, number] = [
      cz.zoom[0] - ZOOM_JUMP_PERCENT * cz.delta,
      cz.zoom[1] - ZOOM_JUMP_PERCENT * cz.delta,
    ];
    this.zoomOrRangeChange(zoom);
  }

  private zoomRightKey() {
    const cz = this.getCurrentZoom();
    const zoom: [number, number] = [
      cz.zoom[0] + ZOOM_JUMP_PERCENT * cz.delta,
      cz.zoom[1] + ZOOM_JUMP_PERCENT * cz.delta,
    ];
    this.zoomOrRangeChange(zoom);
  }

  /**  Returns true if we have any traces to be displayed. */
  private hasData() {
    return Object.keys(this._dataframe.traceset).length > 0 || this._spinning;
  }

  /** Open the query dialog box. */
  private openQuery() {
    this.queryDialog!.showModal();
  }

  private paramsetChanged(e: CustomEvent<ParamSet>) {
    this.query!.paramset = e.detail as CommonSkParamSet;
  }

  private queryChangeDelayedHandler(
    e: CustomEvent<QuerySkQueryChangeEventDetail>,
  ) {
    this.queryCount!.current_query = e.detail.q;
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

  /** Reflect the focused trace in the paramset. */
  private plotTraceFocused(e: CustomEvent<PlotSimpleSkTraceEventDetails>) {
    this.paramset!.highlight = toObject(e.detail.name);
    this.traceID!.textContent = e.detail.name;
  }

  /** User has zoomed in on the graph. */
  private plotZoom() {
    this._render();
  }

  /** Highlight a trace when it is clicked on. */
  private traceSelected(e: CustomEvent<PlotSimpleSkTraceEventDetails>) {
    this.plot!.highlight = [e.detail.name];
    this.commits!.details = [];

    const x = e.detail.x;
    // loop backwards from x until you get the next
    // non MISSING_DATA_SENTINEL point.
    const commits = [this._dataframe.header![x]?.offset];
    const trace = this._dataframe.traceset[e.detail.name];
    for (let i = x - 1; i >= 0; i--) {
      if (trace![i] !== MISSING_DATA_SENTINEL) {
        break;
      }
      commits.push(this._dataframe.header![i]?.offset);
    }
    // Convert the trace id into a paramset to display.
    const params: { [key: string]: string } = toObject(e.detail.name);
    const paramset: ParamSet = {};
    Object.keys(params).forEach((key) => {
      paramset[key] = [params[key]];
    });

    this._render();

    // Request populated commits from the server.
    fetch('/_/cid/', {
      method: 'POST',
      body: JSON.stringify(commits),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((json) => {
        this.commits!.details = json;
        this.commitsTab!.disabled = false;
        this.simpleParamset!.paramsets = [paramset as CommonSkParamSet];
        this.detailTab!.selected = COMMIT_TAB_INDEX;
        const cid = commits[0]!;
        const traceid = e.detail.name;
        this.jsonsource!.cid = cid;
        this.jsonsource!.traceid = traceid;
        this.ingestFileLinks!.load(cid, traceid);
      })
      .catch(errorMessage);
  }

  private startStateReflector() {
    this._stateHasChanged = stateReflector(
      () => (this.state as unknown) as HintableObject,
      (hintableState) => {
        let state = (hintableState as unknown) as State;
        state = this.rationalizeTimeRange(state);
        this.state = state;
        this.range!.state = {
          begin: this.state.begin,
          end: this.state.end,
          num_commits: this.state.numCommits,
          request_type: this.state.requestType,
        };

        this._render();
        this.plot!.dots = this.state.dots;
        // If there is at least one query, the use the last one to repopulate the
        // query-sk dialog.
        const numQueries = this.state.queries.length;
        if (numQueries >= 1) {
          this.query!.current_query = this.state.queries[numQueries - 1];
          this.summary!.paramsets = [
            toParamSet(this.state.queries[numQueries - 1]),
          ];
        }
        this.zeroChanged();
        this.autoRefreshChanged();
        this.rangeChangeImpl();
      },
    );
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
      } else if (this.state.begin !== state.begin) {
        state.end = state.begin + DEFAULT_RANGE_S;
      } else {
        // They set 'end' in the URL.
        state.begin = state.end - DEFAULT_RANGE_S;
      }
    }
    return state;
  }

  private paramsetKeyValueClick(e: CustomEvent<ParamSetSkClickEventDetail>) {
    const keys: string[] = [];
    Object.keys(this._dataframe.traceset).forEach((key) => {
      if (_matches(key, e.detail.key, e.detail.value!)) {
        keys.push(key);
      }
    });
    // Additively highlight if the ctrl key is pressed.
    if (e.detail.ctrl) {
      this.plot!.highlight = this.plot!.highlight.concat(keys);
    } else {
      this.plot!.highlight = keys;
    }
    this._render();
  }

  /** Create a FrameRequest that will re-create the current state of the page. */
  private requestFrameBodyFullFromState(): FrameRequest {
    return {
      begin: this.state.begin,
      end: this.state.end,
      num_commits: this.state.numCommits,
      request_type: this.state.requestType,
      formulas: this.state.formulas,
      queries: this.state.queries,
      keys: this.state.keys,
      tz: Intl.DateTimeFormat().resolvedOptions().timeZone,
      pivot: null,
    };
  }

  /** Reload all the queries/formulas on the given time range. */
  private rangeChangeImpl() {
    if (!this.state) {
      return;
    }
    if (
      this.state.formulas.length === 0
      && this.state.queries.length === 0
      && this.state.keys === ''
    ) {
      return;
    }

    if (this.traceID) {
      this.traceID.textContent = '';
    }
    const body = this.requestFrameBodyFullFromState();
    const switchToTab = body.formulas!.length > 0 || body.queries!.length > 0 || body.keys !== '';
    this.requestFrame(body, (json) => {
      if (json == null) {
        errorMessage('Failed to find any matching traces.');
        return;
      }
      this.plot!.removeAll();
      this.addTraces(json, switchToTab);
      this._render();
    });
  }

  private zeroChangeHandler(e: MouseEvent) {
    this.state.showZero = (e.target! as HTMLInputElement).checked;
    this._stateHasChanged();
    this.zeroChanged();
  }

  private toggleDotsHandler() {
    this.state.dots = !this.state.dots;
    this._stateHasChanged();
    this.plot!.dots = this.state.dots;
  }

  private zeroChanged() {
    if (!this._dataframe.header) {
      return;
    }
    if (this.state.showZero) {
      const lines: { [key: string]: number[] } = {};
      lines[ZERO_NAME] = Array(this._dataframe.header.length).fill(0);
      this.plot!.addLines(lines, []);
    } else {
      this.plot!.deleteLines([ZERO_NAME]);
    }
  }

  private autoRefreshHandler(e: MouseEvent) {
    this.state.autoRefresh = (e.target! as HTMLInputElement).checked;
    this._stateHasChanged();
    this.autoRefreshChanged();
  }

  private autoRefreshChanged() {
    if (!this.state.autoRefresh) {
      if (this._refreshId !== -1) {
        clearInterval(this._refreshId);
      }
    } else {
      this._refreshId = window.setInterval(
        () => this.autoRefresh(),
        REFRESH_TIMEOUT,
      );
    }
  }

  private autoRefresh() {
    // Update end to be now.
    this.state.end = Math.floor(Date.now() / 1000);
    const body = this.requestFrameBodyFullFromState();
    const switchToTab = body.formulas!.length > 0 || body.queries!.length > 0 || body.keys !== '';
    this.requestFrame(body, (json) => {
      this.plot!.removeAll();
      this.addTraces(json, switchToTab);
    });
  }

  /**
   * Add traces to the display. Always called from within the
   * this._requestFrame() callback.
   *
   * @param {Object} json - The parsed JSON returned from the server.
   * otherwise replace them all with the new ones.
   * @param {Boolean} tab - If true then switch to the Params tab.
   */
  private addTraces(json: FrameResponse, tab: boolean) {
    const dataframe = json.dataframe!;
    if (dataframe.traceset === null) {
      return;
    }

    // Add in the 0-trace.
    if (this.state.showZero) {
      dataframe.traceset[ZERO_NAME] = Array(dataframe.header!.length).fill(0);
    }

    this._dataframe = dataframe;
    this.plot!.removeAll();
    const labels: Date[] = [];
    dataframe.header!.forEach((header) => {
      labels.push(new Date(header!.timestamp * 1000));
    });

    this.plot!.addLines(dataframe.traceset, labels);

    // Normalize bands to be just offsets.
    const bands: number[] = [];
    dataframe.header!.forEach((h, i) => {
      if (json.skps!.indexOf(h!.offset) !== -1) {
        bands.push(i);
      }
    });
    this.plot!.bands = bands;

    // Populate the xbar if present.
    if (this.state.xbaroffset !== -1) {
      const xbaroffset = this.state.xbaroffset;
      let xbar = -1;
      this._dataframe.header!.forEach((h, i) => {
        if (h!.offset === xbaroffset) {
          xbar = i;
        }
      });
      if (xbar !== -1) {
        this.plot!.xbar = xbar;
      } else {
        this.plot!.xbar = -1;
      }
    } else {
      this.plot!.xbar = -1;
    }

    // Populate the paramset element.
    this.paramset!.paramsets = [dataframe.paramset as CommonSkParamSet];
    if (tab) {
      this.detailTab!.selected = PARAMS_TAB_INDEX;
    }
  }

  /**
   * Plot the traces that match this._query.current_query.
   *
   * @param replace - If true then replace all the traces with ones
   * that match this query, otherwise add them to the current traces being
   * displayed.
   */
  private add(replace: boolean) {
    this.queryDialog!.close();
    const q = this.query!.current_query;
    if (!q || q.trim() === '') {
      errorMessage('The query must not be empty.');
      return;
    }
    this.state.begin = this.range!.state.begin;
    this.state.end = this.range!.state.end;
    this.state.numCommits = this.range!.state.num_commits;
    this.state.requestType = this.range!.state.request_type;
    if (replace) {
      this.removeAll(true);
    }
    if (this.state.queries.indexOf(q) === -1) {
      this.state.queries.push(q);
    }
    this._stateHasChanged();
    const body = this.requestFrameBodyFullFromState();
    this.requestFrame(body, (json) => {
      this.addTraces(json, true);
    });
  }

  /**
   * Removes all traces.
   *
   * @param skipHistory  If true then don't update the URL. Used
   * in calls like _normalize() where this is just an intermediate state we
   * don't want in history.
   */
  private removeAll(skipHistory: boolean) {
    this.state.formulas = [];
    this.state.queries = [];
    this.state.keys = '';
    this.plot!.removeAll();
    this._dataframe.traceset = {};
    this.paramset!.paramsets = [];
    this.traceID!.textContent = '';
    this.detailTab!.selected = PARAMS_TAB_INDEX;
    this._render();
    if (!skipHistory) {
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
      this.state.keys = '';
      this.state.queries = [];
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
        this.state.keys = json.id;
        this.state.queries = [];
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
    if (this.state.keys !== '') {
      updatedFormulas.push(`${funcName}(shortcut("${this.state.keys}"))`);
    }

    // Also apply the func to any existing formulas.
    updatedFormulas = updatedFormulas.concat(
      this.state.formulas.map((f) => `${funcName}(${f})`),
    );

    this.removeAll(true);
    this.state.formulas = updatedFormulas;
    this._stateHasChanged();
    await this.requestFrame(this.requestFrameBodyFullFromState(), (json) => {
      this.addTraces(json, false);
    });
  }

  private removeHighlighted() {
    const ids = this.plot!.highlight;
    const toShortcut: string[] = [];

    Object.keys(this._dataframe.traceset).forEach((key) => {
      if (ids.indexOf(key) !== -1) {
        // Detect if it is a formula being removed.
        if (this.state.formulas.indexOf(key) !== -1) {
          this.state.formulas.splice(this.state.formulas.indexOf(key), 1);
        }
        return;
      }
      if (key[0] === ',') {
        toShortcut.push(key);
      }
    });

    // Remove the traces from the traceset so they don't reappear.
    ids.forEach((key) => {
      if (this._dataframe.traceset[key] !== undefined) {
        delete this._dataframe.traceset[key];
      }
    });
    this.plot!.deleteLines(ids);
    this.plot!.highlight = [];
    this.reShortCut(toShortcut);
  }

  private highlightedOnly() {
    const ids = this.plot!.highlight;
    const toremove: string[] = [];
    const toShortcut: string[] = [];

    Object.keys(this._dataframe.traceset).forEach((key) => {
      if (ids.indexOf(key) === -1 && !key.startsWith('special')) {
        // Detect if it is a formula being removed.
        if (this.state.formulas.indexOf(key) !== -1) {
          this.state.formulas.splice(this.state.formulas.indexOf(key), 1);
        } else {
          toremove.push(key);
        }
        return;
      }
      if (key[0] === ',') {
        toShortcut.push(key);
      }
    });

    // Remove the traces from the traceset so they don't reappear.
    toremove.forEach((key) => {
      delete this._dataframe.traceset[key];
    });

    this.plot!.deleteLines(toremove);
    this.plot!.highlight = [];
    this.reShortCut(toShortcut);
  }

  /**
   * Plot the traces from the formula in this._formula.value;
   *
   * @param replace - If true then replace all the traces with the
   * calculated traces from this formula, otherwise add the calculated traces to
   * the current traces being displayed.
   */
  private addCalculated(replace: boolean) {
    this.queryDialog!.close();
    const f = this.formula!.value;
    if (f.trim() === '') {
      errorMessage('The formula must not be empty.');
      return;
    }

    this.state.begin = this.range!.state.begin;
    this.state.end = this.range!.state.end;
    this.state.numCommits = this.range!.state.num_commits;
    this.state.requestType = this.range!.state.request_type;

    if (replace) {
      this.removeAll(true);
    }
    if (this.state.formulas.indexOf(f) === -1) {
      this.state.formulas.push(f);
    }
    const body = this.requestFrameBodyFullFromState();
    this._stateHasChanged();
    this.requestFrame(body, (json) => {
      this.addTraces(json, false);
    });
  }

  /** Common catch function for _requestFrame and _checkFrameRequestStatus. */
  private catch(msg: string) {
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
    body.tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
    if (this._requestId !== '') {
      errorMessage('There is a pending query already running.');
      return;
    }

    this._requestId = 'About to make request';
    this.spinning = true;
    try {
      const finishedProg = await startRequest('/_/frame/start', body, 200, this.spinner!, (prog: progress.SerializedProgress) => {
        this.percent!.textContent = `${messageByName(prog.messages, 'Percent', '0')}%`;
      });
      if (finishedProg.status !== 'Finished') {
        throw (new Error(messagesToErrorString(finishedProg.messages)));
      }
      const msg = messageByName(finishedProg.messages, 'Message');
      if (msg) {
        errorMessage(msg);
      }
      cb(finishedProg.results as FrameResponse);
    } catch (msg) {
      this.catch(msg);
    } finally {
      this.spinning = false;
      this._requestId = '';
    }
  }

  // Download all the displayed data as a CSV file.
  private csv() {
    if (this._csvBlobURL) {
      URL.revokeObjectURL(this._csvBlobURL);
      this._csvBlobURL = '';
    }
    const csv = [];
    let line: (string | number)[] = ['id'];
    this._dataframe.header!.forEach((_, i) => {
      // TODO(jcgregorio) Look up the git hash and use that as the header.
      line.push(i.toString());
    });
    csv.push(line.join(','));
    Object.keys(this._dataframe.traceset).forEach((traceId) => {
      if (traceId === ZERO_NAME) {
        return;
      }
      line = [`"${traceId}"`];
      this._dataframe.traceset[traceId]!.forEach((f) => {
        if (f !== MISSING_DATA_SENTINEL) {
          line.push(f);
        } else {
          line.push('');
        }
      });
      csv.push(line.join(','));
    });
    const blob = new Blob([csv.join('\n')], { type: 'text/csv' });
    const url = URL.createObjectURL(blob);
    this.csvDownload!.href = url;
    this._csvBlobURL = url;
    this.csvDownload!.click();
  }

  private isZero(n: number) {
    return n === 0;
  }
}

define('explore-sk', ExploreSk);
