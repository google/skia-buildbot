/**
 * @module module/explore-sk
 * @description <h2><code>explore-sk</code></h2>
 *
 * Main page of Perf, for exploring data.
 */
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { toParamSet } from 'common-sk/modules/query';
import dialogPolyfill from 'dialog-polyfill';
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
import '../plot-simple-sk';
import '../query-count-sk';

// MISSING_DATA_SENTINEL signifies a missing sample value.
//
// JSON doesn't support NaN or +/- Inf, so we need a valid float32 to signal
// missing data that also has a compact JSON representation.
//
// The mirror Go definition is in infra/go/vec32.
const MISSING_DATA_SENTINEL = 1e32;

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

// The minimum length [right - left] of a zoom range.
const MIN_ZOOM_RANGE = 0.1;

// TODO(jcgregorio) Move to a 'key' module.
// Returns true if paramName=paramValue appears in the given structured key.
function _matches(key, paramName, paramValue) {
  return key.indexOf(`,${paramName}=${paramValue},`) >= 0;
}

// TODO(jcgregorio) Move to a 'key' module.
// Parses the structured key and returns a populated object with all
// the param names and values.
function toObject(key) {
  const ret = {};
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

const template = (ele) => html`
  <div id=buttons>
    <button @click=${ele._openQuery}>Query</button>
    <div id=traceButtons ?hide_if_no_data=${!ele._hasData()}>
      <button @click=${() => ele._removeAll(false)} title='Remove all the traces.'>Remove All</button>
      <button @click=${ele._removeHighlighted} title='Remove all the highlighted traces.'>Remove Highlighted</button>
      <button @click=${ele._highlightedOnly} title='Remove all but the highlighted traces.'>Highlighted Only</button>
      <span title='Number of commits skipped between each point displayed.' ?hidden=${ele._isZero(ele._dataframe.skip)} id=skip>${ele._dataframe.skip}</span>
      <checkbox-sk name=zero @change=${ele._zeroChangeHandler} ?checked=${ele.state.show_zero} label='Zero' title='Toggle the presence of the zero line.'>Zero</checkbox-sk>
      <checkbox-sk name=auto @change=${ele._autoRefreshHandler} ?checked=${ele.state.auto_refresh} label='Auto-refresh'   title='Auto-refresh the data displayed in the graph.'>Auto-Refresh</checkbox-sk>
      <button @click=${ele._zoomToRange} ?disabled=${ele._zoomRange === null} title="Fit the time range to the current zoom window.">Zoom Range</button>
    </div>
  </div>

  <div id=spin-overlay>
    <plot-simple-sk
      summary
      width=1200
      height=400
      id=plot
      ?spinning=${ele.spinning}
      @trace_selected=${ele._traceSelected}
      @zoom=${ele._plotZoom}
      @trace_focused=${ele._plotTraceFocused}
      ?hide_if_no_data=${!ele._hasData()}
      >
    </plot-simple-sk>
    <div id=spin-container ?spinning=${ele.spinning}>
      <spinner-sk id=spinner ?active=${ele.spinning}></spinner-sk>
      <span id=percent></span>
    </div>
  </div>

  <div id=bottomButtons>
    <div id=shiftButtons ?hide_if_no_data=${!ele._hasData()}>
      <button @click=${ele._shiftLeft} title='Move ${ele._numShift} commits in the past.'>&lt;&lt; ${ele._numShift}</button>
      <button @click=${ele._shiftBoth} title='Expand the display ${ele._numShift} commits in both directions.'>&lt;&lt; ${+ele._numShift} &gt;&gt;</button>
      <button @click=${ele._shiftRight} title='Move ${ele._numShift} commits in the future.'>${+ele._numShift} &gt;&gt;</button>
    </div>
    <div id=calcButtons ?hide_if_no_data=${!ele._hasData()}>
      <button @click=${ele._normalize} title='Apply norm() to all the traces.'>Normalize</button>
      <button @click=${ele._scale_by_avg} title='Apply scale_by_avg() to all the traces.'>Scale By Avg</button>
      <button @click=${ele._csv} title='Download all displayed data as a CSV file.'>CSV</button>
        <a href='' target=_blank download='traces.csv' id=csv_download></a>
    </div>
  </div>

  <dialog id='query-dialog'>
    <h2>Query</h2>
    <div class=query-parts>
      <query-sk
        id=query
        @query-change=${ele._queryChangeHandler}
        @query-change-delayed=${ele._queryChangeDelayedHandler}
        ></query-sk>
        <div id=selections>
          <h3>Selections</h3>
          <paramset-sk id=summary></paramset-sk>
          <div class=query-counts>
            Matches: <query-count-sk url='/_/count/' @paramset-changed=${ele._paramsetChanged}>
            </query-count-sk>
          </div>
          <button @click=${() => ele._add(true)} class=action>Plot</button>
          <button @click=${() => ele._add(false)}>Add to Plot</button>
        </div>
    </div>
    <details>
      <summary><h2>Time Range</h2></summary>
      <domain-picker-sk id=range .state=${ele.state}>
      </domain-picker-sk>
    </details>

    <details>
      <summary><h2>Calculated Traces</h2></summary>
      <div class=formulas>
        <textarea id=formula rows=3 cols=80></textarea>
        <button @click=${() => ele._addCalculated(true)}>Plot</button>
        <button @click=${() => ele._addCalculated(false)}>Add to Plot</button>
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

    <div id=tabs ?hide_if_no_data=${!ele._hasData()}>
      <tabs-sk id=detailTab>
        <button>Params</button>
        <button id=commitsTab disabled>Details</button>
      </tabs-sk>
      <tabs-panel-sk>
        <div>
          <p>
            <b>Trace ID</b>: <span title='Trace ID' id=trace_id></span>
          </p>
          <paramset-sk id=paramset clickable_values @paramset-key-value-click=${ele._paramsetKeyValueClick}></paramset-sk>
        </div>
        <div id=details>
          <paramset-sk id=simple_paramset clickable_values @paramset-key-value-click=${ele._paramsetKeyValueClick}></paramset-sk>
          <div>
            <commit-detail-panel-sk id=commits></commit-detail-panel-sk>
            <json-source-sk id=jsonsource></json-source-sk>
          </div>
        </div>
      </tabs-panel-sk>
    </div>
  `;

define('explore-sk', class extends ElementSk {
  constructor() {
    super(template);

    // The data being displayed. This is a serialized dataframe.DataFrame.
    this._dataframe = {
      traceset: {},
    };

    // The state that goes into the URL.
    this.state = {
      begin: Math.floor(Date.now() / 1000 - DEFAULT_RANGE_S),
      end: Math.floor(Date.now() / 1000),
      formulas: [],
      queries: [],
      keys: '', // The id of the shortcut to a list of trace keys.
      xbaroffset: -1, // The offset of the commit in the repo.
      show_zero: true,
      auto_refresh: false,
      num_commits: 50,
      request_type: 1, // TODO(jcgregorio) Use constants in domain-picker-sk.
    };

    // Are we waiting on data from the server.
    this._spinning = false;

    // The id of the current frame request. Will be the empty string if there
    // is no pending request.
    this._requestId = '';

    this._numShift = window.sk.perf.num_shift;

    // The id of the interval timer if we are refreshing.
    this._refreshId = -1;

    // All the data converted into a CVS blob to download.
    this._csvBlob = null;

    // Either null if the user hasn't zoomed, or {xBegin: Date(), xEnd: Date()}.
    this._zoomRange = null;

    // Call this anytime something in this.state is changed. Will be replaced
    // with the real function once stateReflector has been setup.
    this._stateHasChanged = () => { };
  }

  connectedCallback() {
    super.connectedCallback();
    if (this._initialized) {
      return;
    }
    this._initialized = true;
    this._render();

    this._commits = this.querySelector('#commits');
    this._commitsTab = this.querySelector('#commitsTab');
    this._detailTab = this.querySelector('#detailTab');
    this._formula = this.querySelector('#formula');
    this._jsonsource = this.querySelector('#jsonsource');
    this._paramset = this.querySelector('#paramset');
    this._percent = this.querySelector('#percent');
    this._plot = this.querySelector('#plot');
    this._query = this.querySelector('#query');
    this._query_count = this.querySelector('query-count-sk');
    this._range = this.querySelector('#range');
    this._simple_paramset = this.querySelector('#simple_paramset');
    this._spinner = this.querySelector('#spinner');
    this._summary = this.querySelector('#summary');
    this._trace_id = this.querySelector('#trace_id');
    this._csv_download = this.querySelector('#csv_download');
    this._queryDialog = this.querySelector('#query-dialog');
    dialogPolyfill.registerDialog(this._queryDialog);
    this._helpDialog = this.querySelector('#help');
    dialogPolyfill.registerDialog(this._helpDialog);

    // Populate the query element.
    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
    fetch(`/_/initpage/?tz=${tz}`, {
      method: 'GET',
    }).then(jsonOrThrow).then((json) => {
      const now = Math.floor(Date.now() / 1000);
      this.state.begin = now - 60 * 60 * 24;
      this.state.end = now;
      this._range.state = this.state;

      this._query.key_order = window.sk.perf.key_order;
      this._query.paramset = json.dataframe.paramset;

      // Remove the paramset so it doesn't get displayed in the Params tab.
      json.dataframe.paramset = {};

      // From this point on reflect the state to the URL.
      this._startStateReflector();
    }).catch(errorMessage);

    document.addEventListener('keydown', (e) => this._keyDown(e));
  }

  _keyDown(e) {
    // Ignore IME composition events.
    if (e.isComposing || e.keyCode === 229) {
      return;
    }
    switch (e.key) {
      case '?':
        this._helpDialog.showModal();
        break;
      case ',': // dvorak
      case 'w':
        this._zoomInKey();
        break;
      case 'o': // dvorak
      case 's':
        this._zoomOutKey();
        break;
      case 'a':
        this._zoomLeftKey();
        break;
      case 'e': // dvorak
      case 'd':
        this._zoomRightKey();
        break;
      default:
        break;
    }
  }

  /**
   * @returns {Object} The current zoom and the length between the left and right edges of
   * the zoom as an object of the form:
   *
   *   {
   *     zoom: [2.0, 12.0],
   *     delta: 10.0,
   *   }
   */
  _getCurrentZoom() {
    let zoom = this._plot.zoom;
    if (zoom === null) {
      zoom = [0, this._dataframe.header.length - 1];
    }
    let delta = zoom[1] - zoom[0];
    if (delta < MIN_ZOOM_RANGE) {
      const mid = (zoom[0] + zoom[1]) / 2;
      zoom[0] = mid - MIN_ZOOM_RANGE / 2;
      zoom[1] = mid + MIN_ZOOM_RANGE / 2;
      delta = MIN_ZOOM_RANGE;
    }
    return {
      zoom: zoom,
      delta: delta,
    };
  }

  /**
   * Clamp a single zoom endpoint.
   *
   * @param {Number} z - One end of a zoom range.
   * @returns {Number} The value of z clamped to valid values.
   */
  _clampZoomEnd(z) {
    if (z < 0) {
      z = 0;
    }
    if (z > this._dataframe.header.length - 1) {
      z = this._dataframe.header.length - 1;
    }
    return z;
  }

  /**
   * Fixes up the zoom range so it always make sense.
   *
   * @param {Array<Number>} zoom - The zoom range.
   * @returns {Array<Number>} The zoom range.
   */
  _rationalizeZoom(zoom) {
    zoom[0] = this._clampZoomEnd(zoom[0]);
    zoom[1] = this._clampZoomEnd(zoom[1]);
    if (zoom[0] > zoom[1]) {
      const left = zoom[0];
      zoom[0] = zoom[1];
      zoom[1] = left;
    }
    return zoom;
  }

  _zoomInKey() {
    const cz = this._getCurrentZoom();
    const zoom = [
      cz.zoom[0] + ZOOM_JUMP_PERCENT * cz.delta,
      cz.zoom[1] - ZOOM_JUMP_PERCENT * cz.delta,
    ];
    this._plot.zoom = this._rationalizeZoom(zoom);
  }

  _zoomOutKey() {
    const cz = this._getCurrentZoom();
    const zoom = [
      cz.zoom[0] - ZOOM_JUMP_PERCENT * cz.delta,
      cz.zoom[1] + ZOOM_JUMP_PERCENT * cz.delta,
    ];
    this._plot.zoom = this._rationalizeZoom(zoom);
  }

  _zoomLeftKey() {
    const cz = this._getCurrentZoom();
    const zoom = [
      cz.zoom[0] - ZOOM_JUMP_PERCENT * cz.delta,
      cz.zoom[1] - ZOOM_JUMP_PERCENT * cz.delta,
    ];
    this._plot.zoom = this._rationalizeZoom(zoom);
  }

  _zoomRightKey() {
    const cz = this._getCurrentZoom();
    const zoom = [
      cz.zoom[0] + ZOOM_JUMP_PERCENT * cz.delta,
      cz.zoom[1] + ZOOM_JUMP_PERCENT * cz.delta,
    ];
    this._plot.zoom = this._rationalizeZoom(zoom);
  }

  // Returns true if we have any traces to be displayed.
  _hasData() {
    return Object.keys(this._dataframe.traceset).length > 0 || this._spinning;
  }

  // Open the query dialog box.
  _openQuery() {
    this._queryDialog.showModal();
  }

  _paramsetChanged(e) {
    this._query.paramset = e.detail;
  }

  _queryChangeDelayedHandler(e) {
    this._query_count.current_query = e.detail.q;
  }

  // Reflect the current query to the query summary.
  _queryChangeHandler(e) {
    const query = e.detail.q;
    this._summary.paramsets = [toParamSet(query)];
    const formula = this._formula.value;
    if (formula === '') {
      this._formula.value = `filter("${query}")`;
    } else if ((formula.match(/"/g) || []).length === 2) {
      // Only update the filter query if there's one string in the formula.
      this._formula.value = formula.replace(/".*"/, `"${query}"`);
    }
  }

  // Reflect the focused trace in the paramset.
  _plotTraceFocused(e) {
    this._paramset.highlight = toObject(e.detail.name);
    this._trace_id.textContent = e.detail.name;
  }

  // User has zoomed in on the graph.
  _plotZoom(e) {
    const shouldRender = this._zoomRange === null;
    this._zoomRange = e.detail;
    if (shouldRender) {
      this._render();
    }
  }

  // Fit the time range to the zoom being displayed.
  // Reload all the queries/formulas on the new time range.
  _zoomToRange() {
    this.state.begin = this._zoomRange.xBegin / 1000;
    this.state.end = this._zoomRange.xEnd / 1000;
    this._zoomRange = null;
    this._rangeChangeImpl();
  }

  // Highlight a trace when it is clicked on.
  _traceSelected(e) {
    this._plot.highlight = [e.detail.name];
    this._commits.details = [];

    const x = e.detail.x;
    // loop backwards from x until you get the next
    // non MISSING_DATA_SENTINEL point.
    const commits = [this._dataframe.header[x]];
    const trace = this._dataframe.traceset[e.detail.name];
    for (let i = x - 1; i >= 0; i--) {
      if (trace[i] !== MISSING_DATA_SENTINEL) {
        break;
      }
      commits.push(this._dataframe.header[i]);
    }
    // Convert the trace id into a paramset to display.
    const params = toObject(e.detail.name);
    const paramset = {};
    Object.keys(params).forEach((key) => {
      paramset[key] = [params[key]];
    });

    // Request populated commits from the server.
    fetch('/_/cid/', {
      method: 'POST',
      body: JSON.stringify(commits),
      headers: {
        'Content-Type': 'application/json',
      },
    }).then(jsonOrThrow).then((json) => {
      this._commits.details = json;
      this._commitsTab.disabled = false;
      this._simple_paramset.paramsets = [paramset];
      this._detailTab.selected = COMMIT_TAB_INDEX;
      this._jsonsource.cid = commits[0];
      this._jsonsource.traceid = e.detail.name;
    }).catch(errorMessage);
  }

  _startStateReflector() {
    this._stateHasChanged = stateReflector(() => this.state, (state) => {
      state = this._rationalizeTimeRange(state);
      this.state = state;
      this._range.state = this.state;
      this._render();
      // If there is at least one query, the use the last one to repopulate the
      // query-sk dialog.
      const numQueries = this.state.queries.length;
      if (numQueries >= 1) {
        this._query.current_query = this.state.queries[numQueries - 1];
        this._summary.paramsets = [toParamSet(this.state.queries[numQueries - 1])];
      }
      this._zeroChanged();
      this._autoRefreshChanged();
      this._rangeChangeImpl();
    });
  }

  /**
   * Fixes up the time ranges in the state that came from query values.
   *
   * It is possible for the query URL to specify just the begin or end time,
   * which may end up giving us an inverted time range, i.e. end < begin.
   */
  _rationalizeTimeRange(state) {
    if (state.end <= state.begin) {
      // If dense then just make sure begin is before end.
      if (state.request_type === 1) {
        state.begin = state.end - DEFAULT_RANGE_S;
      } else if (this.state.begin !== state.begin) {
        state.end = state.begin + DEFAULT_RANGE_S;
      } else { // They set 'end' in the URL.
        state.begin = state.end - DEFAULT_RANGE_S;
      }
    }
    return state;
  }

  _paramsetKeyValueClick(e) {
    const keys = [];
    Object.keys(this._dataframe.traceset).forEach((key) => {
      if (_matches(key, e.detail.key, e.detail.value)) {
        keys.push(key);
      }
    });
    // Additively highlight if the ctrl key is pressed.
    if (e.detail.ctrl) {
      this._plot.highlight = this._plot.highlight.concat(keys);
    } else {
      this._plot.highlight = keys;
    }
  }

  _shiftBoth() {
    this._shiftImpl(-this._numShift, this._numShift);
  }

  _shiftRight() {
    this._shiftImpl(this._numShift, this._numShift);
  }

  _shiftLeft() {
    this._shiftImpl(-this._numShift, -this._numShift);
  }

  // Change the current range by the following +/- offsets.
  _shiftImpl(beginOffset, endOffset) {
    const body = {
      begin: this.state.begin,
      begin_offset: beginOffset,
      end: this.state.end,
      end_offset: endOffset,
      num_commits: this.state.num_commits,
      request_type: this.state.request_type,
    };
    fetch('/_/shift/', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    }).then(jsonOrThrow).then((json) => {
      this.state.begin = json.begin;
      this.state.end = json.end;
      this.state.num_commits = json.num_commits;
      this._rangeChangeImpl();
    }).catch(errorMessage);
  }

  // Create a FrameRequest that will re-create the current state of the page.
  _requestFrameBodyFullFromState() {
    return {
      begin: this.state.begin,
      end: this.state.end,
      num_commits: this.state.num_commits,
      request_type: this.state.request_type,
      formulas: this.state.formulas,
      queries: this.state.queries,
      keys: this.state.keys,
    };
  }

  // Reload all the queries/formulas on the given time range.
  _rangeChangeImpl() {
    if (!this.state) {
      return;
    }
    if (this.state.formulas.length === 0 && this.state.queries.length === 0 && this.state.keys === '') {
      return;
    }

    if (this._trace_id) {
      this._trace_id.textContent = '';
    }
    const body = this._requestFrameBodyFullFromState();
    const switchToTab = body.formulas.length > 0 || body.queries.length > 0 || body.keys !== '';
    this._requestFrame(body, (json) => {
      if (json == null) {
        errorMessage('Failed to find any matching traces.');
        return;
      }
      this._plot.removeAll();
      this._addTraces(json, switchToTab);
      this._render();
    });
  }

  _zeroChangeHandler(e) {
    this.state.show_zero = e.target.checked;
    this._stateHasChanged();
    this._zeroChanged();
  }

  _zeroChanged() {
    if (!this._dataframe.header) {
      return;
    }
    if (this.state.show_zero) {
      const lines = {};
      lines[ZERO_NAME] = Array(this._dataframe.header.length).fill(0);
      this._plot.addLines(lines);
    } else {
      this._plot.deleteLines([ZERO_NAME]);
    }
  }

  _autoRefreshHandler(e) {
    this.state.auto_refresh = e.target.checked;
    this._stateHasChanged();
    this._autoRefreshChanged();
  }

  _autoRefreshChanged() {
    if (!this.state.auto_refresh) {
      if (this._refreshId !== -1) {
        clearInterval(this._refreshId);
      }
    } else {
      this._refreshId = setInterval(() => this._autoRefresh(), REFRESH_TIMEOUT);
    }
  }

  _autoRefresh() {
    // Update end to be now.
    this.state.end = Math.floor(Date.now() / 1000);
    const body = this._requestFrameBodyFullFromState();
    const switchToTab = body.formulas.length > 0 || body.queries.length > 0 || body.keys !== '';
    this._requestFrame(body, (json) => {
      this._plot.removeAll();
      this._addTraces(json, switchToTab);
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
  _addTraces(json, tab) {
    const dataframe = json.dataframe;
    if (dataframe.traceset === null) {
      return;
    }

    // Add in the 0-trace.
    if (this.state.show_zero) {
      dataframe.traceset[ZERO_NAME] = Array(dataframe.header.length).fill(0);
    }

    this._dataframe = dataframe;
    this._plot.removeAll();
    const labels = [];
    dataframe.header.forEach((header) => {
      labels.push(new Date(header.timestamp * 1000));
    });

    this._plot.addLines(dataframe.traceset, labels);

    this._plot.bands = json.skps;

    // Populate the xbar if present.
    if (this.state.xbaroffset !== -1) {
      const xbaroffset = this.state.xbaroffset;
      let xbar = -1;
      this._dataframe.header.forEach((h, i) => {
        if (h.offset === xbaroffset) {
          xbar = i;
        }
      });
      if (xbar !== -1) {
        this._plot.xbar = xbar;
      } else {
        this._plot.xbar = -1;
      }
    } else {
      this._plot.xbar = -1;
    }

    // Populate the paramset element.
    this._paramset.paramsets = [dataframe.paramset];
    if (tab) {
      this._detailTab.selected = PARAMS_TAB_INDEX;
    }
  }

  /**
   * Plot the traces that match this._query.current_query.
   *
   * @param {Boolean} replace - If true then replace all the traces with ones
   * that match this query, otherwise add them to the current traces being
   * displayed.
   */
  _add(replace) {
    this._queryDialog.close();
    const q = this._query.current_query;
    if (!q || q.trim() === '') {
      errorMessage('The query must not be empty.');
      return;
    }
    this.state = Object.assign({}, this.state, this._range.state);
    if (replace) {
      this._removeAll(true);
    }
    if (this.state.queries.indexOf(q) === -1) {
      this.state.queries.push(q);
    }
    const body = this._requestFrameBodyFullFromState();
    this._requestFrame(body, (json) => {
      this._addTraces(json, true);
    });
  }

  /**
   * Removes all traces.
   *
   * @param skipHistory {Boolean} - If true then don't update the URL. Used
   * in calls like _normalize() where this is just an intermediate state we
   * don't want in history.
   */
  _removeAll(skipHistory) {
    this.state.formulas = [];
    this.state.queries = [];
    this.state.keys = '';
    this._plot.removeAll();
    this._dataframe.traceset = {};
    this._paramset.paramsets = [];
    this._trace_id.textContent = '';
    this._zoomRange = null;
    this._detailTab.selected = PARAMS_TAB_INDEX;
    this._render();
    if (!skipHistory) {
      this._stateHasChanged();
    }
  }

  // When Remove Highlighted or Highlighted Only are pressed then create a
  // shortcut for just the traces that are displayed.
  //
  // Note that removing a trace doesn't work if the trace came from a
  // formula that returns multiple traces. This is a known issue that
  // isn't currently worth solving.
  //
  // Returns the Promise that's creating the shortcut, or undefined if
  // there isn't a shortcut to create.
  _reShortCut(keys) {
    if (keys.length === 0) {
      this.state.keys = '';
      this.state.queries = [];
      return undefined;
    }
    const state = {
      keys: keys,
    };
    return fetch('/_/keys/', {
      method: 'POST',
      body: JSON.stringify(state),
      headers: {
        'Content-Type': 'application/json',
      },
    }).then(jsonOrThrow).then((json) => {
      this.state.keys = json.id;
      this.state.queries = [];
      this._stateHasChanged();
    }).catch(errorMessage);
  }

  // Create a shortcut for all of the traces currently being displayed.
  //
  // Returns the Promise that's creating the shortcut, or undefined if
  // there isn't a shortcut to create.
  _shortcutAll() {
    const toShortcut = [];

    Object.keys(this._dataframe.traceset).forEach((key) => {
      if (key[0] === ',') {
        toShortcut.push(key);
      }
    });

    return this._reShortCut(toShortcut);
  }

  // Apply norm() to all the traces currently being displayed.
  _normalize() {
    const promise = this._shortcutAll();
    if (!promise) {
      errorMessage('No traces to normalize.');
      return;
    }
    promise.then(() => {
      const f = `norm(shortcut("${this.state.keys}"))`;
      this._removeAll(true);
      const body = this._requestFrameBodyFullFromState();
      Object.assign(body, {
        formulas: [f],
      });
      this.state.formulas.push(f);
      this._stateHasChanged();
      this._requestFrame(body, (json) => {
        this._addTraces(json, false);
      });
    });
  }

  // Apply scale_by_avg() to all the traces currently being displayed.
  _scale_by_avg() {
    const promise = this._shortcutAll();
    if (!promise) {
      errorMessage('No traces to scale.');
      return;
    }
    promise.then(() => {
      const f = `scale_by_avg(shortcut("${this.state.keys}"))`;
      this._removeAll(true);
      const body = this._requestFrameBodyFullFromState();
      Object.assign(body, {
        formulas: [f],
      });
      this.state.formulas.push(f);
      this._stateHasChanged();
      this._requestFrame(body, (json) => {
        this._addTraces(json, false);
      });
    });
  }

  _removeHighlighted() {
    const ids = this._plot.highlight;
    const toShortcut = [];

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
    this._plot.deleteLines(ids);
    this._reShortCut(toShortcut);
  }

  _highlightedOnly() {
    const ids = this._plot.highlight;
    const toremove = [];
    const toShortcut = [];

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

    this._plot.deleteLines(toremove);
    this._reShortCut(toShortcut);
  }

  /**
   * Plot the traces from the formula in this._formula.value;
   *
   * @param {Boolean} replace - If true then replace all the traces with the
   * calculated traces from this formula, otherwise add the calculated traces to
   * the current traces being displayed.
   */
  _addCalculated(replace) {
    this._queryDialog.close();
    const f = this._formula.value;
    if (f.trim() === '') {
      errorMessage('The formula must not be empty.');
      return;
    }
    this.state = Object.assign({}, this.state, this._range.state);
    if (replace) {
      this._removeAll(true);
    }
    if (this.state.formulas.indexOf(f) === -1) {
      this.state.formulas.push(f);
    }
    const body = this._requestFrameBodyFullFromState();
    this._requestFrame(body, (json) => {
      this._addTraces(json, false);
    });
  }

  // Common catch function for _requestFrame and _checkFrameRequestStatus.
  _catch(msg) {
    this._requestId = '';
    if (msg) {
      errorMessage(msg, 10000);
    }
    this._percent.textContent = '';
    this.spinning = false;
  }

  /** @prop {Boolean} spinning - True if we are waiting to retrieve data from
   * the server.
   */
  set spinning(b) {
    this._spinning = b;
    this._render();
  }

  get spinning() {
    return this._spinning;
  }

  // Requests a new dataframe, where body is a serialized FrameRequest:
  //
  // {
  //    begin:    1448325780,
  //    end:      1476706336,
  //    formulas: ["ave(filter("name=desk_nytimes.skp&sub_result=min_ms"))"],
  //    hidden:   [],
  //    queries:  [
  //        "name=AndroidCodec_01_original.jpg_SampleSize8",
  //        "name=AndroidCodec_1.bmp_SampleSize8"],
  //    tz:       "America/New_York"
  // };
  //
  // The 'cb' callback function will be called with the decoded JSON body
  // of the response once it's available.
  _requestFrame(body, cb) {
    body.tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
    if (this._requestId !== '') {
      errorMessage('There is a pending query already running.');
      return;
    }
    this._requestId = 'About to make request';

    this.spinning = true;

    this._stateHasChanged();
    fetch('/_/frame/start', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    }).then(jsonOrThrow).then((json) => {
      this._requestId = json.id;
      this._checkFrameRequestStatus(cb);
    }).catch((msg) => this._catch(msg));
  }

  // Periodically check the status of a pending FrameRequest, calling the
  // 'cb' callback with the decoded JSON upon success.
  _checkFrameRequestStatus(cb) {
    fetch(`/_/frame/status/${this._requestId}`, {
      method: 'GET',
    }).then(jsonOrThrow).then((json) => {
      if (json.state === 'Running') {
        this._percent.textContent = `${Math.floor(json.percent * 100)}%`;
        window.setTimeout(() => this._checkFrameRequestStatus(cb), 300);
      } else {
        fetch(`/_/frame/results/${this._requestId}`, {
          method: 'GET',
        }).then(jsonOrThrow).then((json) => {
          cb(json);
          this._catch(json.msg);
        }).catch((msg) => this._catch(msg));
      }
    }).catch((msg) => this._catch(msg));
  }

  // Download all the displayed data as a CSV file.
  _csv() {
    if (this._csvBlob) {
      URL.revokeObjectURL(this._csvBlob);
      this._csvBlob = null;
    }
    const csv = [];
    let line = ['id'];
    this._dataframe.header.forEach((_, i) => {
      // TODO(jcgregorio) Look up the git hash and use that as the header.
      line.push(i);
    });
    csv.push(line.join(','));
    Object.keys(this._dataframe.traceset).forEach((traceId) => {
      if (traceId === ZERO_NAME) {
        return;
      }
      line = [`"${traceId}"`];
      this._dataframe.traceset[traceId].forEach((f) => {
        if (f !== MISSING_DATA_SENTINEL) {
          line.push(f);
        } else {
          line.push('');
        }
      });
      csv.push(line.join(','));
    });
    this._csvBlob = new Blob([csv.join('\n')], { type: 'text/csv' });
    this._csv_download.href = URL.createObjectURL(this._csvBlob);
    this._csv_download.click();
  }

  _isZero(n) {
    return n === 0;
  }
});
