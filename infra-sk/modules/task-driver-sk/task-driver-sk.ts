/**
 * @module infra-sk/modules/task-driver-sk
 * @description <h2><code>task-driver-sk</code></h2>
 *
 * <p>
 * This element displays information about a Task Driver.
 * </p>
 *
 */
import { define } from 'elements-sk/define';
import { html, render } from 'lit-html';
import { localeTime, strDuration } from 'common-sk/modules/human';
import { upgradeProperty } from 'elements-sk/upgradeProperty';
import { CollapseSk } from 'elements-sk/collapse-sk/collapse-sk';
import { escapeAndLinkify } from '../linkify';
import { StepData, StepDisplay, TaskDriverRunDisplay } from '../../../task_driver/modules/json';
import 'elements-sk/collapse-sk';
import 'elements-sk/icon/launch-icon-sk';
import 'elements-sk/styles/buttons';

/**
 * Describes the extra fields that this custom element adds to the JSON-encoded
 * TaskDriverRunDisplay and StepDisplay structs returned by the Go backend.
 */
interface UIInfo {
  expandProps?: boolean;
  expandEnv?: boolean;
  expandChildren?: boolean;
}

/**
 * Convenience type for use in templates that take both a TaskDriverRunDisplay
 * or a StepDisplay struct as their input.
 */
type RunOrStepDisplay = (TaskDriverRunDisplay & UIInfo) | (StepDisplay & UIInfo);

/** Type guard to differentiate between the TaskDriverRunDisplay and StepDisplay types. */
function isTaskDriverRunDisplay(d: RunOrStepDisplay): d is TaskDriverRunDisplay & UIInfo {
  return (d as TaskDriverRunDisplay).properties !== undefined;
}

const tr = (contents: unknown) => html`<tr>${contents}</tr>`;

const td = (contents: unknown) => html`<td>${contents}</td>`;

const propLine = (k: unknown, v: unknown) => html`
  ${tr(html`${td(k)}${td(v)}`)}
`;

const expando = (expanded = false) => html`<span class="expando">[${expanded ? '-' : '+'}]</span>`;

// Return true if the step is interesting, ie. it has a result other than
// SUCCESS (including not yet finished). The root step (which has no parent)
// is interesting by default.
const stepIsInteresting = (step: StepDisplay): boolean => {
  if (!step.parent) {
    return true;
  }
  return step.result !== 'SUCCESS';
}

export class TaskDriverSk extends HTMLElement {
  private static stepData = (ele: TaskDriverSk, s: RunOrStepDisplay, d: StepData) => {
    switch (d.type) {
      case 'command':
        return propLine('Command', d.data.command.join(' '));
      case 'httpRequest':
        return propLine('HTTP Request', d.data.url);
      case 'httpResponse':
        return propLine('HTTP Response', d.data.status);
      case 'text':
        return propLine(d.data.label, escapeAndLinkify(d.data.value));
      case 'log':
        return propLine(`Log (${d.data.name})`, html`
          <a href="${ele.logLink(s.id!, d.data.id)}" target="_blank">${d.data.name}</a>
      `);
    }
    return '';
  }

  private static stepError = (ele: TaskDriverSk, s: RunOrStepDisplay, e: string, idx: number) => propLine(html`
    <a href="${ele.errLink(s.id!, idx)}" target="_blank">
      Error
    </a>`, html`
    <pre>${e}</pre>
  `);

  private static stepProperties = (ele: TaskDriverSk, s: RunOrStepDisplay) => html`
    <table class="properties">
      ${isTaskDriverRunDisplay(s) && s.properties
    ? html`
          ${s.properties.swarmingServer && s.properties.swarmingTask
      ? propLine('Swarming Task', html`
              <a href="${`${s.properties.swarmingServer}/task?id=${s.properties.swarmingTask}&show_raw=1`}" target="_blank">${s.properties.swarmingTask}</a>
            `)
      : ''
        }
          ${s.properties.swarmingServer && s.properties.swarmingBot
          ? propLine('Swarming Bot', html`
              <a href="${`${s.properties.swarmingServer}/bot?id=${s.properties.swarmingBot}`}" target="_blank">${s.properties.swarmingBot}</a>
            `)
          : ''
        }
          ${!s.properties.local
          ? propLine('Task Scheduler', html`
              <a href="https://task-scheduler.skia.org/task/${s.id}" target="_blank">${s.id}</a>
            `)
          : ''
        }
        `
    : ''
      }
      ${s.isInfra ? propLine('Infra', s.isInfra) : ''}
      ${propLine('Started', ele.displayTime(s.started))}
      ${propLine('Finished', ele.displayTime(s.finished))}
      ${s.environment
        ? propLine('Environment', html`
                <a id="button_env_${s.id}" @click=${() => ele.toggleEnv(s)}>
                  ${expando(s.expandEnv)}
                </a>
                <collapse-sk id="env_${s.id}" ?closed="${!s.expandEnv}">
                  ${s.environment.map((env) => tr(td(env)))}
                </collapse-sk>
              `)
        : ''
      }
      ${s.data ? s.data.map((d) => TaskDriverSk.stepData(ele, s, d)) : ''}
      ${propLine('Log (combined)', html`
          <a href="${ele.logLink(s.id!)}" target="_blank">all logs</a>
      `)}
      ${s.errors ? s.errors.map((e, idx) => TaskDriverSk.stepError(ele, s, e, idx)) : ''}
    </div>
  `;

  private static stepChildren = (ele: TaskDriverSk, s: RunOrStepDisplay) => html`
    <div class="vert children_link">
      <a id="button_children_${s.id}" @click=${() => ele.toggleChildren(s)}>
        ${expando(s.expandChildren)}
      </a>
      ${s.steps!.length} Children
    </div>
    <collapse-sk id="children_${s.id}" ?closed="${!s.expandChildren}">
      ${s.steps!.map((s) => TaskDriverSk.step(ele, s))}
    </collapse-sk>
  `;

  private static stepInner = (ele: TaskDriverSk, s: RunOrStepDisplay) => html`
    <collapse-sk id="props_${s.id}" ?closed="${!s.expandProps}">
      ${TaskDriverSk.stepProperties(ele, s)}
    </collapse-sk>
    ${s.steps && s.steps.length > 0 ? TaskDriverSk.stepChildren(ele, s) : ''}
  `;

  private static step = (ele: TaskDriverSk, s: RunOrStepDisplay): unknown => html`
    <div class="${ele.stepClass(s)}">
      <div class="vert">
        <a class="horiz" id="button_props_${s.id}" @click=${() => ele.toggleProps(s)}>
          ${expando(s.expandProps)}
        </a>
        <div class="${ele.stepNameClass(s)}">${s.name}</div>
        <div class="horiz duration">${ele.duration(s.started, s.finished)}</div>
        ${!s.parent && ele.hasAttribute('embedded') ? html`
          <div class="horiz">
            <a href="https://task-driver.skia.org/td/${s.id}" target="_blank">
              <launch-icon-sk></launch-icon-sk>
            </a>
          </div>
        ` : ''}
      </div>
      ${TaskDriverSk.stepInner(ele, s)}
    </div>
  `;

  private static template = (ele: TaskDriverSk) => TaskDriverSk.step(ele, ele.data);

  private _data: Partial<TaskDriverRunDisplay> = {};

  connectedCallback(): void {
    upgradeProperty(this, 'data');
    this.render();
  }

  private parseDate(ts: string): Date | null {
    if (!ts) {
      return null;
    }
    try {
      const d = new Date(ts);
      if (d.getFullYear() < 1970) {
        return null;
      }
      return d;
    } catch (e) {
      return null;
    }
  }

  private displayTime(ts = ''): string {
    const d = this.parseDate(ts);
    if (!d) {
      return '-';
    }
    return localeTime(d);
  }

  private duration(started = '', finished = ''): string {
    const startedDate = this.parseDate(started);
    if (!startedDate) {
      // PubSub messages may arrive out of order, so it's possible that we don't
      // have a start timestemp for a step. Don't try to compute a duration in
      // that case.
      return '(no start time)';
    }
    let finishedDate = this.parseDate(finished);
    if (!finishedDate) {
      // If we don't have a finished timestamp for the step, we can assume that
      // the step simply hasn't finished yet. Compute the duration of the step
      // so far.
      finishedDate = new Date();
    }
    // TODO(borenet): strDuration only gets down to seconds. It'd be nice to
    // give millisecond precision.
    return strDuration((finishedDate.getTime() - startedDate.getTime()) / 1000);
  }

  private toggleChildren(step: RunOrStepDisplay) {
    const collapse = this.querySelector<CollapseSk>(`#children_${step.id}`)!;
    collapse.closed = !collapse.closed;
    step.expandChildren = !collapse.closed;
    this.render();
  }

  private toggleEnv(step: RunOrStepDisplay) {
    const collapse = this.querySelector<CollapseSk>(`#env_${step.id}`)!;
    collapse.closed = !collapse.closed;
    step.expandEnv = !collapse.closed;
    this.render();
  }

  private toggleProps(step: RunOrStepDisplay) {
    const collapse = this.querySelector<CollapseSk>(`#props_${step.id}`)!;
    collapse.closed = !collapse.closed;
    step.expandProps = !collapse.closed;
    this.render();
  }

  private errLink(stepId: string, idx: number): string {
    let link = `/errors/${this._data.id}`;
    if (stepId !== this._data.id) {
      link += `/${stepId}`;
    }
    return `${link}/${idx}`;
  }

  private logLink(stepId: string, logId?: string): string {
    let link = `/logs/${this._data.id}`;
    if (stepId !== this._data.id) {
      link += `/${stepId}`;
    }
    if (logId) {
      link += `/${logId}`;
    }
    return link;
  }

  // Process the step data. Return true if the current step is interesting.
  private process(step: RunOrStepDisplay): boolean {
    // Sort the step data, so that the properties end up in a predictable order.
    if (step.data) {
      step.data.sort((a, b) => {
        if (a.type < b.type) {
          return -1;
        } if (a.type > b.type) {
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
    for (let i = 0; i < (step.steps || []).length; i++) {
      if (this.process(step.steps![i])) {
        anyChildInteresting = true;
      }
    }
    const isInteresting = stepIsInteresting(step);
    step.expandChildren = isInteresting && anyChildInteresting;
    // Always hide the environment by default - this is rarely useful by users.
    step.expandEnv = false;

    // Step properties take up a lot of space on the screen. Only display them
    // if the step is interesting AND it has no interesting children.
    // Unsuccessful steps which have unsuccessful children are most likely to
    // have inherited the result of their children and so their properties are
    // not as important of those of the failed child step.
    step.expandProps = isInteresting && !anyChildInteresting;
    // Always expand the root, which contains any errors that happened.
    if (!step.parent) {
      step.expandProps = true;
    }
    return isInteresting;
  }

  get data(): TaskDriverRunDisplay { return this._data as TaskDriverRunDisplay; }

  set data(val: TaskDriverRunDisplay) {
    this.process(val);
    this._data = val;
    this.render();
  }

  get embedded(): boolean { return this.hasAttribute('embedded'); }

  set embedded(isEmbedded: boolean) {
    if (isEmbedded) {
      this.setAttribute('embedded', '');
    } else {
      this.removeAttribute('embedded');
    }
    this.render();
  }

  private render() {
    render(TaskDriverSk.template(this), this, { eventContext: this });
  }

  private stepClass(s: StepDisplay): string {
    let res = s.result;
    if (s.isInfra && s.result === 'FAILURE') {
      res = 'EXCEPTION';
    }
    if (!res) {
      res = 'IN_PROGRESS';
    }
    return `step ${res}`;
  }

  private stepNameClass(s: StepDisplay): string {
    if (s.parent) {
      return 'horiz h4';
    }
    return 'horiz h2';
  }
}

define('task-driver-sk', TaskDriverSk);
