/**
 * @module module/explore-sk
 * @description <h2><code>explore-sk</code></h2>
 *
 * Main page of Perf, for exploring data.
 */
import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import { errorMessage } from 'elements-sk/errorMessage'
import { html, render } from 'lit-html'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'
import { stateReflector } from 'common-sk/modules/stateReflector'

import 'elements-sk/checkbox-sk'
import 'elements-sk/icon/help-icon-sk'
import 'elements-sk/spinner-sk'
import 'elements-sk/styles/buttons'
import 'elements-sk/tabs-panel-sk'
import 'elements-sk/tabs-sk'

import '../commit-detail-panel-sk'
import '../domain-picker-sk'
import '../json-source-sk'
import '../paramset-sk'
import '../plot-simple-sk'
import '../query-count-sk'
import '../query-sk'
import '../query-summary-sk'

// MISSING_DATA_SENTINEL signifies a missing sample value.
//
// JSON doesn't support NaN or +/- Inf, so we need a valid float32 to signal
// missing data that also has a compact JSON representation.
//
// The mirror Go definition is in infra/go/vec32.
const MISSING_DATA_SENTINEL = 1e32;

const ZERO_NAME = "special_zero";

const REFRESH_TIMEOUT = 30*1000; // milliseconds

const DEFAULT_RANGE_S = 24*60*60 // 2 days in seconds.

// TODO(jcgregorio) Move to a 'key' module.
// Returns true if paramName=paramValue appears in the given structured key.
function _matches(key, paramName, paramValue) {
  return key.indexOf("," + paramName + "=" + paramValue + ",") >= 0;
};

// TODO(jcgregorio) Move to a 'key' module.
// Parses the structured key and returns a populated object with all
// the param names and values.
function toObject(key) {
  let ret = {};
  key.split(",").forEach(function(s, i) {
    if (i == 0 ) {
      return
    }
    if (s === "") {
      return;
    }
    let parts = s.split("=");
    if (parts.length != 2) {
      return
    }
    ret[parts[0]] = parts[1];
  });
  return ret;
};

const template = (ele) => html`
  <div id=buttons>
    <domain-picker-sk id=range .state=${ele.state} @domain-changed=${ele._rangeChange}></domain-picker-sk>
    <button @click=${ele._removeHighlighted} title="Remove all the highlighted traces.">Remove Highlighted</button>
    <button @click=${ele._removeAll} title="Remove all the traces.">Remove All</button>
    <button @click=${ele._highlightedOnly} title="Remove all but the highlighted traces.">Highlighted Only</button>
    <button @click=${ele._clearHighlights} title="Remove highlights from all traces.">Clear Highlights</button>
    <button @click=${ele._resetAxes} title="Reset back to the original zoom level.">Reset Axes</button>
    <button @click=${ele._shiftLeft} title="Move ${ele._numShift} commits in the past.">&lt;&lt; ${ele._numShift}</button>
    <button @click=${ele._shiftBoth} title="Expand the display ${ele._numShift} commits in both directions.">&lt;&lt; ${+ele._numShift} &gt;&gt;</button>
    <button @click=${ele._shiftRight} title="Move ${ele._numShift} commits in the future.">${+ele._numShift} &gt;&gt;</button>
    <button @click=${ele._zoomToRange} id=zoom_range disabled title="Fit the time range to the current zoom window.">Zoom Range</button>
    <span title="Number of commits skipped between each point displayed." ?hidden=${ele._isZero(ele._dataframe.skip)} id=skip>${ele._dataframe.skip}</span>
    <checkbox-sk name=zero @change=${ele._zeroChangeHandler} ?checked=${ele.state.show_zero} label="Zero" title="Toggle the presence of the zero line.">Zero</checkbox-sk>
    <checkbox-sk name=auto @change=${ele._autoRefreshHandler} ?checked=${ele.state.auto_refresh} label="Auto-refresh" title="Auto-refresh the data displayed in the graph.">Auto-Refresh</checkbox-sk>
    <button @click=${ele._normalize} title="Apply norm() to all the traces.">Normalize</button>
    <button @click=${ele._scale_by_ave} title="Apply scale_by_ave() to all the traces.">Scale By Ave</button>
    <button @click=${ele._csv} title="Download all displayed data as a CSV file.">CSV</button>
    <a href="" target=_blank download="traces.csv" id=csv_download></a>
    <div id=spin-container>
      <spinner-sk id=spinner></spinner-sk>
      <span id=percent></span>
    </div>
  </div>

  <plot-simple-sk
    width=1024 height=256 id=plot
    @trace_selected=${ele._traceSelected}
    @zoom=${ele._plotZoom}
    @trace_focused=${ele._plotTraceFocused}
    ></plot-simple-sk>

    <div id=tabs>
      <tabs-sk id=detailTab>
        <button>Query</button>
        <button>Params</button>
        <button id=commitsTab disabled>Details</button>
      </tabs-sk>
      <tabs-panel-sk>
        <div id=queryTab>
          <div class=query-parts>
            <query-sk
              id=query
              @query-change=${ele._queryChangeHandler}
              @query-change-delayed=${ele._queryChangeDelayedHandler}
              ></query-sk>
              <div id=selections>
                <h3>Selections</h3>
                <query-summary-sk id=summary></query-summary-sk>
                <div class=query-counts>
                  Matches: <query-count-sk url='/_/count/' @paramset-changed=${ele._paramsetChanged}>
                  </query-count-sk>
                </div>
                <button @click=${ele._add} class=action>Plot</button>
              </div>
          </div>
          <h3>Calculated Traces</h3>
          <div class=formulas>
            <textarea id=formula rows=3 cols=80></textarea>
            <button @click=${ele._addCalculated} class=action>Add</button>
            <a href=/help/ target=_blank>
              <help-icon-sk></help-icon-sk>
            </a>
          </div>
        </div>
        <div>
          <p>
            <b>Trace ID</b>: <span title="Trace ID" id=trace_id></span>
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

window.customElements.define('explore-sk', class extends ElementSk {
  constructor() {
    super(template);
    // Keep track of the data sent to plot.
    this._lines = {};

    this._dataframe = {
      traceset: {},
    };

    // The state that goes into the URL.
    this.state = {
      begin: Math.floor(Date.now()/1000 - DEFAULT_RANGE_S),
      end: Math.floor(Date.now()/1000),
      formulas: [],
      queries: [],
      keys: "",  // The id of the shortcut to a list of trace keys.
      xbaroffset: -1, // The offset of the commit in the repo.
      show_zero: true,
      auto_refresh: false,
      num_commits: 50,
      request_type: 1,
    };

    // The [begin, end] timestamps of the region that the user is zoomed in
    // on.
    this._zoomRange = [];

    // The id of the current frame request. Will be the empty string if there
    // is no pending request.
    this._requestId = '';

    this._numShift = sk.perf.num_shift;
    this._refreshId = -1;
    this._csvBlob = null;

    this._stateHasChanged = () => {};
  }

  connectedCallback() {
    super.connectedCallback();
    if (this._initialized) {
      return;
    }
    this._initialized = true;
    this._render();

    // TODO(jcgregorio) This is the first time I've missed Polymer's this.$.id notation.
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
    this._zoomRange = this.querySelector('#zoomRange');
    this._zoom_range = this.querySelector('#zoom_range');
    this._csv_download = this.querySelector('#csv_download');

    // Populate the query element.
    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
    fetch('/_/initpage/?tz=' + tz, {
      method: 'GET',
    }).then(jsonOrThrow).then((json) => {
      const now = Math.floor(Date.now()/1000);
      this.state.begin = now - 60*60*24;
      this.state.end   = now;
      this._range.state = this.state;

      this._query.key_order = sk.perf.key_order;
      this._query.paramset = json.dataframe.paramset;

      // Remove the paramset so it doesn't get displayed in the Params tab.
      json.dataframe.paramset = {};

      // From this point on reflect the state to the URL.
      this._startStateReflector();
    }).catch(errorMessage);
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
      this._summary.selection = query;
      const formula = this._formula.value;
      if (formula == "") {
        this._formula.value = 'filter("' + query + '")';
      } else if (2 == (formula.match(/\"/g) || []).length) {
        // Only update the filter query if there's one string in the formula.
        this._formula.value = formula.replace(/".*"/, '"' + query + '"');
      }
  }

  // Reflect the focused trace in the paramset.
  _plotTraceFocused(e) {
    if (e.detail.id === ZERO_NAME) {
      return;
    }
    this._paramset.highlight = toObject(e.detail.id);
    this._trace_id.textContent= e.detail.id;
  }

  // User has zoomed in on the graph.
  _plotZoom(e) {
    this._zoomRange = [Math.floor(e.detail.xMin/1000), Math.floor(e.detail.xMax/1000)];
    this._zoom_range.disabled = false;
  }

  // Highlight a trace when it is clicked on.
  _traceSelected(e) {
    this._plot.clearHighlight();
    this._plot.setHighlight(e.detail.id);
    this._commits.details = [];

    const x = +e.detail.pt[0]|0;
    // loop backwards from x until you get the next
    // non MISSING_DATA_SENTINEL point.
    const commits = [this._dataframe.header[x]];
    const trace = this._dataframe.traceset[e.detail.id];
    for (let i = x-1; i >= 0; i--) {
      if (trace[i] != MISSING_DATA_SENTINEL) {
        break;
      }
      commits.push(this._dataframe.header[i]);
    }
    // Convert the trace id into a paramset to display.
    let params = toObject(e.detail.id);
    let paramset = {}
    Object.keys(params).forEach((key) => {
      paramset[key] = [params[key]];
    });

    // Request populated commits from the server.
    fetch('/_/cid/', {
      method: 'POST',
      body: JSON.stringify(commits),
      headers:{
        'Content-Type': 'application/json'
      }
    }).then(jsonOrThrow).then(json => {
      this._commits.details = json;
      this._commitsTab.disabled = false;
      this._simple_paramset.paramsets = {
        paramsets: [paramset],
      };
      this._detailTab.selected = 2;
      this._jsonsource.cid = commits[0];
      this._jsonsource.traceid = e.detail.id;
    }).catch(errorMessage);
  }

  _startStateReflector() {
    this._stateHasChanged = stateReflector(() => this.state, (state) => {
      state = this._rationalizeTimeRange(state);
      this.state = state;
      this._range.state = this.state;
      this._render();
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
      } else {
        // If 'begin' was set in the URL.
        if (this.state.begin != state.begin) {
          state.end = state.begin + DEFAULT_RANGE_S;
        } else { // They set 'end' in the URL.
          state.begin = states.end - DEFAULT_RANGE_S;
        }
      }
    }
    return state;
  }

  _paramsetKeyValueClick(e) {
    const keys = [];
    Object.keys(this._lines).forEach((key) => {
      if (_matches(key, e.detail.key, e.detail.value)) {
        keys.push(key);
      }
    });
    // Additively highlight if the ctrl key is pressed.
    if (!e.detail.ctrl) {
      this._plot.clearHighlight();
    }
    this._plot.setHighlight(keys);
  }

  // Fit the time range to the zoom being displayed.
  // Reload all the queries/formulas on the new time range.
  _zoomToRange() {
    this.state.begin = this._zoomRange[0];
    this.state.end = this._zoomRange[1];
    this._rangeChangeImpl();
  }

  // Called when the domain-picker-sk control has changed, causes all the
  // queries/formulas to be reloaded for the new time range.
  _rangeChange(e) {
    this.state.begin = e.detail.state.begin;
    this.state.end = e.detail.state.end;
    this.state.num_commits = e.detail.state.num_commits;
    this.state.request_type = e.detail.state.request_type;
    this._rangeChangeImpl();
  }

  _shiftBoth(e) {
    this._shiftImpl(-this._numShift, this._numShift);
  }

  _shiftRight(e) {
    this._shiftImpl(this._numShift, this._numShift);
  }

  _shiftLeft(e) {
    this._shiftImpl(-this._numShift, -this._numShift);
  }

  // Change the current range by the following +/- offsets.
  _shiftImpl(beginOffset, endOffset) {
    const body = {
      begin:         this.state.begin,
      begin_offset:  beginOffset,
      end:           this.state.end,
      end_offset:    endOffset,
      num_commits:   this.state.num_commits,
      request_type:  this.state.request_type,
    }
    fetch('/_/shift/', {
      method: 'POST',
      body: JSON.stringify(body),
      headers:{
        'Content-Type': 'application/json'
      }
    }).then(jsonOrThrow).then((json) => {
      this.state.begin = json.begin;
      this.state.end = json.end;
      this.state.num_commits = json.num_commits;
      this._rangeChangeImpl();
    }).catch(errorMessage);
  }

  // Fill in the basic data needed for a FrameRequest that will be common
  // to all situations.
  _requestFrameBodyFromState() {
    return {
      begin: this.state.begin,
      end: this.state.end,
      num_commits: this.state.num_commits,
      request_type: this.state.request_type,
    };
  }

  // Create a FrameRequest that will re-create the current state of the page.
  _requestFrameBodyFullFromState() {
    let body = this._requestFrameBodyFromState();
    return Object.assign(body, {
      formulas: this.state.formulas,
      queries: this.state.queries,
      keys: this.state.keys,
    });
  }

  // Reload all the queries/formulas on the given time range.
  _rangeChangeImpl() {
    if (!this.state) {
      return;
    }
    if (this.state.formulas.length == 0 && this.state.queries.length == 0 && this.state.keys == "") {
      return;
    }

    if (this._trace_id) {
      this._trace_id.textContent = '';
    }
    const body = this._requestFrameBodyFullFromState();
    const switchToTab = body.formulas.length > 0 || body.queries.length > 0 || body.keys != "";
    this._requestFrame(body, (json) => {
      if (json == null) {
        errorMessage('Failed to find any matching traces.');
        return;
      }
      this._plot.removeAll();
      this._lines = [];
      this._addTraces(json, false, switchToTab);
      this._plot.resetAxes();
      this._zoom_range.disabled = true;
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
      const line = [];
      for (let i = 0; i < this._dataframe.header.length; i++) {
        line.push([i, 0]);
      }
      const lines = {};
      lines[ZERO_NAME] = line;
      this._plot.addLines(lines);
    } else {
      this._plot.deleteLine(ZERO_NAME);
    }
    this._plot.resetAxes();
  }

  _autoRefreshHandler(e) {
    this.state.auto_refresh = e.target.checked;
    this._stateHasChanged();
    this._autoRefreshChanged();
  }

  _autoRefreshChanged() {
    if (!this.state.auto_refresh) {
      if (this._refreshId != -1) {
        clearInterval(this._refreshId);
      }
    } else {
      this._refreshId = setInterval(() => this._autoRefresh(), REFRESH_TIMEOUT);
    }
  }

  _autoRefresh() {
    // Update end to be now.
    this.state.end = Math.floor(Date.now()/1000);
    const body = this._requestFrameBodyFullFromState();
    const switchToTab = body.formulas.length > 0 || body.queries.length > 0 || body.keys != "";
    this._requestFrame(body, (json) => {
      this._plot.removeAll();
      this._lines = [];
      this._addTraces(json, false, switchToTab);
      this._zoom_range.disabled = true;
    });
  }

  _addTraces(json, incremental, tab) {
    const dataframe = json.dataframe;
    const lines = {};

    if (dataframe.traceset === null) {
      return;
    }

    // Add in the 0-trace.
    if (this.state.show_zero) {
      dataframe.traceset[ZERO_NAME] = Array(dataframe.header.length).fill(0);
    }

    // Convert the dataframe into a form suitable for the plot element.
    const keys = Object.keys(dataframe.traceset);
    keys.forEach((key) => {
      const values = [];
      dataframe.traceset[key].forEach(function(y, x) {
        if (y != MISSING_DATA_SENTINEL) {
          values.push([x, y]);
        } else {
          values.push([x, null]);
        }
      });
      lines[key] = values;
    });

    if (incremental) {
      Object.keys(lines).forEach((key) => {
        this._lines[key] = lines[key];
      });
      if (this._dataframe.header === undefined) {
        this._dataframe = dataframe;
      } else {
        Object.keys(dataframe.traceset).forEach((key) => {
          this._dataframe.traceset[key] = dataframe.traceset[key];
        });
      }
    } else {
      this._lines = lines;
      this._dataframe = dataframe;
    }
    if (!incremental) {
      this._plot.removeAll();
    }
    const labels = [];
    dataframe.header.forEach((header) => {
      labels.push(new Date(header.timestamp * 1000));
    });

    this._plot.addLines(this._lines, labels);

    // Always add in the last index so we draw a band if there's an odd
    // number of skp updates.
    json.skps.push(json.dataframe.header.length-1);

    const bands = [];
    for (let i = 1; i < json.skps.length; i+=2) {
      bands.push([json.skps[i-1], json.skps[i]]);
    }
    this._plot.setBanding(bands);

    // Populate the xbar if present.
    if (this.state.xbaroffset != -1) {
      let xbaroffset = this.state.xbaroffset;
      let xbar = -1;
      this._dataframe.header.forEach((h, i) => {
        if (h.offset == xbaroffset) {
          xbar = i;
        }
      });
      if (xbar != -1) {
        this._plot.setXBar(xbar);
      } else {
        this._plot.clearXBar();
      }
    } else {
      this._plot.clearXBar();
    }

    // Populate the paramset element.
    this._paramset.paramsets = {
      paramsets: [dataframe.paramset],
    };
    if (tab) {
      this._detailTab.selected = 1;
    }
  }

  _add() {
    const q = this._query.current_query.trim();
    if (q == "") {
      return
    }
    if (this.state.queries.indexOf(q) == -1) {
      this.state.queries.push(q);
    }
    let body = this._requestFrameBodyFromState();
    Object.assign(body, {
      queries: [q],
    });
    this._requestFrame(body, (json) => {
      this._addTraces(json, true, true);
    });
  }

  _removeAll() {
    this.state.formulas = [];
    this.state.queries = [];
    this.state.keys = "";
    this._plot.removeAll();
    this._lines = [];
    this._stateHasChanged();
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
    if (keys.length == 0) {
      this.state.keys = "";
      this.state.queries = [];
      return undefined;
    }
    const state = {
      keys: keys,
    };
    return fetch('/_/keys/', {
      method: 'POST',
      body: JSON.stringify(state),
      headers:{
        'Content-Type': 'application/json'
      }
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
      if (key[0] == ",") {
        toShortcut.push(key);
      }
    });

    return this._reShortCut(toShortcut);
  }

  // Apply norm() to all the traces currently being displayed.
  _normalize() {
    let promise = this._shortcutAll();
    if (!promise) {
      errorMessage("No traces to normalize.");
      return;
    }
    promise.then(() => {
      let f = `norm(shortcut("${this.state.keys}"))`
      this._removeAll();
      let body = this._requestFrameBodyFromState();
      Object.assign(body, {
        formulas: [f],
      });
      this.state.formulas.push(f);
      this._requestFrame(body, (json) => {
        this._addTraces(json, true, false);
      });
    });
  }

  // Apply scale_by_ave() to all the traces currently being displayed.
  _scale_by_ave() {
    let promise = this._shortcutAll();
    if (!promise) {
      errorMessage("No traces to scale.");
      return;
    }
    promise.then(() => {
      let f = `scale_by_ave(shortcut("${this.state.keys}"))`
      this._removeAll();
      var body = this._requestFrameBodyFromState();
      Object.assign(body, {
        formulas: [f],
      });
      this.state.formulas.push(f);
      this._requestFrame(body, (json) => {
        this._addTraces(json, true, false);
      });
    });
  }

  _removeHighlighted() {
    const ids = this._plot.highlighted();
    const toadd = {};
    const toShortcut = [];

    Object.keys(this._dataframe.traceset).forEach((key) => {
      if (ids.indexOf(key) != -1) {
        // Detect if it is a formula being removed.
        if (this.state.formulas.indexOf(key) != -1) {
          this.state.formulas.splice(this.state.formulas.indexOf(key), 1)
        }
        return;
      }
      if (key[0] == ",") {
        toShortcut.push(key);
      }
      const values = [];
      this._dataframe.traceset[key].forEach((y, x) => {
        if (y != MISSING_DATA_SENTINEL) {
          values.push([x, y]);
        } else {
          values.push([x, null]);
        }
      });
      toadd[key] = values;
    });

    // Remove the traces from the traceset so they don't reappear.
    ids.forEach((key) => {
      if (this._dataframe.traceset[key] != undefined) {
        delete this._dataframe.traceset[key];
      }
    });

    this._lines = toadd;
    this._plot.removeAll();
    this._plot.addLines(toadd);
    this._reShortCut(toShortcut);

  }

  _highlightedOnly() {
    const ids = this._plot.highlighted();
    const toadd = {};
    const toremove = [];
    const toShortcut = [];

    Object.keys(this._dataframe.traceset).forEach((key) => {
      if (ids.indexOf(key) == -1 && !key.startsWith("special")) {
        // Detect if it is a formula being removed.
        if (this.state.formulas.indexOf(key) != -1) {
          this.state.formulas.splice(this.state.formulas.indexOf(key), 1)
        } else {
          toremove.push(key);
        }
        return;
      }
      if (key[0] == ",") {
        toShortcut.push(key);
      }
      const values = [];
      this._dataframe.traceset[key].forEach((y, x) => {
        if (y != MISSING_DATA_SENTINEL) {
          values.push([x, y]);
        } else {
          values.push([x, null]);
        }
      });
      toadd[key] = values;
    });

    // Remove the traces from the traceset so they don't reappear.
    Object.keys(this._dataframe.traceset).forEach((key) => {
      if (key in toremove) {
        delete this._dataframe.traceset[key];
      }
    });

    this._lines = toadd;
    this._plot.removeAll();
    this._plot.addLines(toadd);
    this._reShortCut(toShortcut);
  }

  _clearHighlights() {
    this._plot.clearHighlight();
  }

  _resetAxes() {
    this._plot.resetAxes();
    this._zoom_range.disabled = true;
  }

  _addCalculated() {
    const f = this._formula.value;
    if (f.trim() === '') {
      return;
    }
    if (this.state.formulas.indexOf(f) === -1) {
      this.state.formulas.push(f);
    }
    let body = this._requestFrameBodyFromState();
    Object.assign(body, {
      formulas: [f],
    });
    this._requestFrame(body, function(json) {
      // TODO(jcgregorio) Remove all returned trace ids from hidden.
      this._addTraces(json, true, false);
    }.bind(this));
  }

  // Common catch function for _requestFrame and _checkFrameRequestStatus.
  _catch(msg) {
    this._requestId = '';
    if (msg) {
      errorMessage(msg, 10000);
    }
    this._percent.textContent = '';
    this._spinner.active = false;
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
    if (this._requestId != '') {
      errorMessage('There is a pending query already running.');
      return;
    } else {
      this._requestId = 'About to make request';
    }
    this._spinner.active = true;

    this._stateHasChanged();
    fetch('/_/frame/start', {
      method: 'POST',
      body: JSON.stringify(body),
      headers:{
        'Content-Type': 'application/json'
      }
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
        this._percent.textContent = Math.floor(json.percent*100) + '%';
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
    let csv = [];
    let line = ['id'];
    this._dataframe.header.forEach((_,i) => {
      // TODO(jcgregorio) Look up the git hash and use that as the header.
      line.push(i);
    });
    csv.push(line.join(','));
    for (const traceId in this._dataframe.traceset) {
      if (traceId === ZERO_NAME) {
        continue
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
    }
    this._csvBlob = new Blob([csv.join('\n')], {type: 'text/csv'});
    this._csv_download.href = URL.createObjectURL(this._csvBlob);
    this._csv_download.click();
  }

  _isZero(n) {
    return n === 0;
  }

});
