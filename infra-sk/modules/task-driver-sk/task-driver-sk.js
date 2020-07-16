/**
 * @module infra-sk/modules/task-driver-sk
 * @description <h2><code>task-driver-sk</code></h2>
 *
 * <p>
 * This element displays information about a Task Driver.
 * </p>
 *
 */
import { define } from 'elements-sk/define'
import { escapeAndLinkify } from '../linkify'
import { html, render } from 'lit-html'
import { localeTime, strDuration } from 'common-sk/modules/human'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'
import { errorMessage } from 'elements-sk/errorMessage'
import { upgradeProperty } from 'elements-sk/upgradeProperty'
import 'elements-sk/collapse-sk'
import 'elements-sk/icon/launch-icon-sk'
import 'elements-sk/styles/buttons'


const tr = (contents) => html`<tr>${contents}</tr>`;

const td = (contents) => html`<td>${contents}</td>`;

const propLine = (k, v) => html`
  ${tr(html`${td(k)}${td(v)}`)}
`;

function stepData(ele, s, d) {
  switch(d.type) {
    case "command":
      return propLine("Command", d.data.command.join(" "));
    case "httpRequest":
      return propLine("HTTP Request", d.data.url);
    case "httpResponse":
      return propLine("HTTP Response", d.data.status);
    case "text":
      return propLine(d.data.label, escapeAndLinkify(d.data.value));
    case "log":
      return propLine("Log (" + d.data.name + ")", html`
          <a href="${ele._logLink(s.id, d.data.id)}" target="_blank">${d.data.name}</a>
      `);
  }
  return "";
}

const stepError = (ele, s, e, idx) => propLine(html`
  <a href="${ele._errLink(s.id, idx)}" target="_blank">
    Error
  </a>`, html`
  <pre>${e}</pre>
`);

const stepProperties = (ele, s) => html`
  <table class="properties">
    ${s.properties
      ? html`
        ${s.properties.swarmingServer && s.properties.swarmingTask
          ? propLine("Swarming Task", html`
            <a href="${s.properties.swarmingServer + "/task?id=" + s.properties.swarmingTask + "&show_raw=1"}" target="_blank">${s.properties.swarmingTask}</a>
          `)
          : ""
        }
        ${s.properties.swarmingServer && s.properties.swarmingBot
          ? propLine("Swarming Bot", html`
            <a href="${s.properties.swarmingServer + "/bot?id=" + s.properties.swarmingBot}" target="_blank">${s.properties.swarmingBot}</a>
          `)
          : ""
        }
        ${!s.properties.local
          ? propLine("Task Scheduler", html`
            <a href="https://task-scheduler.skia.org/task/${s.id}" target="_blank">${s.id}</a>
          `)
          : ""
        }
      `
      : ""
    }
    ${s.isInfra ? propLine("Infra", s.isInfra) : ""}
    ${propLine("Started", ele._displayTime(s.started))}
    ${propLine("Finished", ele._displayTime(s.finished))}
    ${s.environment
        ? propLine("Environment", html`
            <a id="button_env_${s.id}" @click=${ev => ele._toggleEnv(s)}>
              ${expando(s.expandEnv)}
            </a>
            <collapse-sk id="env_${s.id}" ?closed="${!s.expandEnv}">
              ${s.environment.map(env => tr(td(env)))}
            </collapse-sk>
          `)
        : ""
    }
    ${s.data ? s.data.map((d) => stepData(ele, s, d)) : ""}
    ${propLine("Log (combined)", html`
        <a href="${ele._logLink(s.id)}" target="_blank">all logs</a>
    `)}
    ${s.errors ? s.errors.map((e, idx) => stepError(ele, s, e, idx)) : ""}
  </div>
`;

const stepChildren = (ele, s) => html`
  <div class="vert children_link">
    <a id="button_children_${s.id}" @click=${(ev) => ele._toggleChildren(s)}>
      ${expando(s.expandChildren)}
    </a>
    ${s.steps.length} Children
  </div>
  <collapse-sk id="children_${s.id}" ?closed="${!s.expandChildren}">
    ${s.steps.map((s) => step(ele, s))}
  </collapse-sk>
`;

const stepInner = (ele, s) => html`
    <collapse-sk id="props_${s.id}" ?closed="${!s.expandProps}">
      ${stepProperties(ele, s)}
    </collapse-sk>
    ${s.steps && s.steps.length > 0 ? stepChildren(ele, s) : ""}
`;

const expando = (expanded) => html`<span class="expando">[${expanded ? "-" : "+"}]</span>`;

const step = (ele, s) => html`
  <div class="${ele._stepClass(s)}">
    <div class="vert">
      <a class="horiz" id="button_props_${s.id}" @click=${(ev) => ele._toggleProps(s)}>
        ${expando(s.expandProps)}
      </a>
      <div class="${ele._stepNameClass(s)}">${s.name}</div>
      <div class="horiz duration">${ele._duration(s.started, s.finished)}</div>
      ${!s.parent && ele.hasAttribute('embedded') ? html`
        <div class="horiz">
          <a href="https://task-driver.skia.org/td/${s.id}" target="_blank">
            <launch-icon-sk></launch-icon-sk>
          </a>
        </div>
      ` : ""}
    </div>
    ${stepInner(ele, s)}
  </div>
`;

const template = (ele) => step(ele, ele.data);

define('task-driver-sk', class extends HTMLElement {
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

  _displayTime(ts) {
    let d = this._parseDate(ts);
    if (!d) {
      return "-";
    }
    return localeTime(d);
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

  _toggleChildren(step) {
    let collapse = document.getElementById("children_" + step.id);
    collapse.closed = !collapse.closed;
    step.expandChildren = !collapse.closed;
    this._render();
  }

  _toggleEnv(step) {
    let collapse = document.getElementById("env_" + step.id);
    collapse.closed = !collapse.closed;
    step.expandEnv = !collapse.closed;
    this._render();
  }

  _toggleProps(step) {
    let collapse = document.getElementById("props_" + step.id);
    collapse.closed = !collapse.closed;
    step.expandProps = !collapse.closed;
    this._render();
  }

  _errLink(stepId, idx) {
    let link = "/errors/" + this._data.id;
    if (stepId !== this._data.id) {
      link += "/" + stepId;
    }
    return link + "/" + idx;
  }

  _logLink(stepId, logId) {
    let link = "/logs/" + this._data.id;
    if (stepId !== this._data.id) {
      link += "/" + stepId;
    }
    if (logId) {
      link += "/" + logId;
    }
    return link
  }

  // Return true if the step is interesting, ie. it has a result other than
  // SUCCESS (including not yet finished). The root step (which has no parent)
  // is interesting by default.
  _stepIsInteresting(step) {
    if (!step.parent) {
      return true
    }
    return step.result != "SUCCESS";
  }

  // Process the step data. Return true if the current step is interesting.
  _process(step) {
    // Sort the step data, so that the properties end up in a predictable order.
    if (step.data) {
      step.data.sort(function(a, b) {
        if (a.type < b.type) {
          return -1;
        } else if (a.type > b.type) {
          return 1;
        }
        if (a.data.name < b.data.name) {
          return -1;
        }
        return 1;
      });
    }

    // We expand the children of this step if this step is interesting AND if
    // any of the children are interesting. Note that parent steps which do not
    // inherit the failure of one of their children will not be considered
    // interesting unless they fail for another reason.
    let anyChildInteresting = false;
    for (var i = 0; i < (step.steps || []).length; i++) {
      if (this._process(step.steps[i])) {
        anyChildInteresting = true;
      }
    }
    let isInteresting = this._stepIsInteresting(step);
    step.expandChildren = false;
    if (isInteresting && anyChildInteresting) {
      step.expandChildren = true;
    }
    step.expandEnv = false;

    // Step properties take up a lot of space on the screen. Only display them
    // if the step is interesting AND it has no interesting children.
    // Unsuccessful steps which have unsuccessful children are most likely to
    // have inherited the result of their children and so their properties are
    // not as important of those of the failed child step.
    step.expandProps = isInteresting && !anyChildInteresting;

    return isInteresting;
  }

  get data() { return this._data; }
  set data(val) {
    this._process(val);
    this._data = val;
    this._render();
  }

  get embedded() { return this.hasAttribute('embedded'); }
  set embedded(isEmbedded) {
    if (isEmbedded) {
      this.setAttribute('embedded', '');
    } else {
      this.removeAttribute('embedded');
    }
    this._render();
  }

  _render() {
    render(template(this), this, {eventContext: this});
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
    if (!res) {
      res = "IN_PROGRESS";
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
