/**
 * @module task-driver-sk
 * @description <h2><code>task-driver-sk</code></h2>
 *
 * <p>
 * This element displays information about a Task Driver.
 * </p>
 *
 */
import { html, render } from 'lit-html/lib/lit-extended'
import { $$ } from 'common-sk/modules/dom'
import { localeTime, strDuration } from 'common-sk/modules/human'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'
import { errorMessage } from 'elements-sk/errorMessage'
import { upgradeProperty } from 'elements-sk/upgradeProperty'
import 'elements-sk/collapse-sk'
import 'elements-sk/styles/buttons'


const template = (ele) => step(ele, ele.data);

const step = (ele, s) => html`
  <div class$="${ele._stepClass(s)}">
    <div class="vert">
      <div class$="${ele._stepNameClass(s)}">${s.name}</div>
      <div class="horiz duration">${ele._duration(s.started, s.finished)}</div>
      <button class="horiz" id="button_props_${s.id}" on-click=${(ev) => ele._toggleProps(s.id)}>
        ${s.expandProps ? "-" : "+"}
      </button>
    </div>
    ${stepInner(ele, s)}
  </div>
`;

const stepInner = (ele, s) => html`
    <collapse-sk id="props_${s.id}" closed?="${!s.expandProps}">
      ${stepProperties(ele, s)}
    </collapse-sk>
    ${s.steps && s.steps.length > 0 ? stepChildren(ele, s) : ""}
`;

const stepChildren = (ele, s) => html`
  <button id="button_children_${s.id}" on-click=${(ev) => ele._toggleChildren(s.id)}>
    ${s.expandChildren ? "-" : "+"}
  </button>
  ${s.steps.length} Children
  <collapse-sk id="children_${s.id}" closed?="${!s.expandChildren}">
    ${s.steps.map((s) => step(ele, s))}
  </collapse-sk>
`;

const stepProperties = (ele, s) => html`
  <div class="table">
    <div class="tr"><div class="td">ID</div><div class="td">${s.id}</div></div>
    <div class="tr"><div class="td">Infra</div><div class="td">${s.isInfra}</div></div>
    <div class="tr"><div class="td">Started</div><div class="td">${localeTime(s.started)}</div></div>
    <div class="tr"><div class="td">Finished</div><div class="td">${localeTime(s.finished)}</div></div>
    <div class="tr">
      <div class="td">Logs</div>
      <div class="td">
        <a href="${ele._logLink(s)}" target="_blank">Cloud Logs</a>
        <!-- TODO(borenet): Each step might have additional logs emitted via
             STEP_DATA. We should add separate links here. -->
      </div>
    </div>
  </div>
`;

window.customElements.define('task-driver-sk', class extends HTMLElement {
  constructor() {
    super();
    this._data = {};
  }

  connectedCallback() {
    upgradeProperty(this, 'data');
    this._render();
  }

  _parseDate(ts) {
    if (!ts) {
      return null;
    }
    try {
      let d = new Date(ts);
      if (d.getFullYear() < 1970) {
        return null;
      }
      return d;
    } catch(e) {
      return null;
    }
  }

  _duration(started, finished) {
    let startedDate = this._parseDate(started);
    if (!startedDate) {
      // PubSub messages may arrive out of order, so it's possible that we don't
      // have a start timestemp for a step. Don't try to compute a duration in
      // that case.
      return "(no start time)";
    }
    let finishedDate = this._parseDate(finished);
    if (!finishedDate) {
      // If we don't have a finished timestamp for the step, we can assume that
      // the step simply hasn't finished yet. Compute the duration of the step
      // so far.
      finishedDate = new Date();
    }
    // TODO(borenet): strDuration only gets down to seconds. It'd be nice to
    // give millisecond precision.
    let duration = strDuration((finishedDate.getTime() - startedDate.getTime()) / 1000);
    return duration;
  }

  _toggleChildren(id) {
    let collapse = document.getElementById("children_" + id);
    collapse.closed = !collapse.closed;
    let button = document.getElementById("button_children_" + id);
    if (collapse.closed) {
      button.innerHTML = "+";
    } else {
      button.innerHTML = "-";
    }
  }

  _toggleProps(id) {
    let collapse = document.getElementById("props_" + id);
    collapse.closed = !collapse.closed;
    let button = document.getElementById("button_props_" + id);
    if (collapse.closed) {
      button.innerHTML = "+";
    } else {
      button.innerHTML = "-";
    }
  }

  _logLink(step) {
    // Build the logs filter.
    let project = "skia-swarming-bots";
    let taskId = this._data.id;
    let logName = `projects/${project}/logs/task-driver`;
    let filter = {
      "logName": logName,
      "labels.taskId": taskId,
      "textPayload": "*",
    };
    if (step.parent) {
      filter["labels.stepId"] = step.id;
    }

    // Stringify the filter.
    let filterStr = "";
    for (var key in filter) {
      if (filterStr) {
        filterStr += "\n";
      }
      filterStr += key + "=\"" + filter[key] + "\"";
    }

    // Gather the remaining URL params.
    let params = {
      "project": project,
      "logName": logName,
      "minLogLevel": 1,
      "dateRangeUnbound": "backwardInTime",
      "advancedFilter": filterStr,
    };

    // Build the URL.
    let rv = "https://pantheon.corp.google.com/logs/viewer";
    let first = true;
    for (var key in params) {
      if (first) {
        rv += "?";
        first = false;
      } else {
        rv += "&"
      }
      rv += key + "=" + encodeURIComponent(params[key]);
    }
    return rv;
  }

  _setExpanded(step) {
    // We display the properties of the step by default if the step failed or
    // if it is the root step.
    step.expandProps = false;
    if (step.result != "SUCCESS") {
      step.expandProps = true;
    } else if (!step.parent) {
      step.expandProps = true;
    }

    // We expand the children of this step if this step failed AND if any of
    // the children failed.
    step.expandChildren = false;
    if (step.result != "SUCCESS") {
      for (var i = 0; i < (step.steps || []).length; i++) {
        if (this._setExpanded(step.steps[i])) {
          step.expandChildren = true;
        }
      }
      return true;
    }
    return false;
  }

  get data() { return this._data; }
  set data(val) {
    this._setExpanded(val);
    this._data = val;
    this._render();
  }

  _render() {
    console.log("_render");
    render(template(this), this);
  }

  _reload() {
    fetch(`/json/task`)
      .then(jsonOrThrow)
      .then((json) => {
        this._data = json;
        this._render();
      }
    ).catch((e) => {
      errorMessage('Failed to load task driver', 10000);
      this.data = {};
      this._render();
    });
  }

  _stepClass(s) {
    let res = s.result;
    if (s.isInfra && s.result == "FAILURE") {
      res = "EXCEPTION";
    }
    return "step " + res;
  }

  _stepNameClass(s) {
    if (s.parent) {
      return "horiz h4";
    }
    return "horiz h2";
  }
});
