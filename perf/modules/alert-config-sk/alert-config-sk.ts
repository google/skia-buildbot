/**
 * @module module/alert-config-sk
 * @description <h2><code>alert-config-sk</code></h2>
 *
 * Control that allows editing an alert.Config.
 *
 */
import { html, LitElement, TemplateResult } from 'lit';
import { customElement, property } from 'lit/decorators.js';

import '../../../elements-sk/modules/checkbox-sk';
import '../../../elements-sk/modules/multi-select-sk';
import '../../../elements-sk/modules/select-sk';
import '../../../elements-sk/modules/spinner-sk';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { SpinnerSk } from '../../../elements-sk/modules/spinner-sk/spinner-sk';
import {
  SelectSk,
  SelectSkSelectionChangedEventDetail,
} from '../../../elements-sk/modules/select-sk/select-sk';
import { MultiSelectSkSelectionChangedEventDetail } from '../../../elements-sk/modules/multi-select-sk/multi-select-sk';
import { errorMessage } from '../errorMessage';
import {
  ParamSet,
  Alert,
  Direction,
  StepDetection,
  ConfigState,
  TryBugRequest,
  TryBugResponse,
  SerializesToString,
  AlertAction,
} from '../json';
import { QuerySkQueryChangeEventDetail } from '../../../infra-sk/modules/query-sk/query-sk';
import { AlgoSelectAlgoChangeEventDetail } from '../algo-select-sk/algo-select-sk';

import '../algo-select-sk';
import '../query-chooser-sk';
import '../window/window';

const toDirection = (val: string | null): Direction => {
  if (val === 'UP') {
    return 'UP';
  }
  if (val === 'DOWN') {
    return 'DOWN';
  }
  return 'BOTH';
};

const toConfigState = (s: string | null): ConfigState => {
  if (s === 'ACTIVE') {
    return 'ACTIVE';
  }
  return 'DELETED';
};

/**
 * The labels and units for a single kind of threshold, which vary based on the
 * StepDetection chosed.
 */
interface ThresholdDescriptor {
  units: string;
  label: string;
}

/**
 * The labels and units for the Threshhold input, which vary
 * based on the StepDetection chosed.
 */
const thresholdDescriptors: Record<StepDetection, ThresholdDescriptor> = {
  '': {
    units: 'R',
    label: `Consider change significant if |(x-y)/σ²| > R.
     This is the original regression factor.
     Values in the range of 10-50 are suggested.`,
  },
  percent: {
    units: 'percent',
    label: `Consider change significant if |(x-y)/x| > percent.
      Values between 0.1 and 1.0 work well.`,
  },
  percent_median: {
    units: 'percent',
    label: `Consider change significant if |(x-y)/median| > percent.
      Values between 0.1 and 1.0 work well.`,
  },
  const: {
    units: 'magnitude',
    label: 'Consider change significant if |x| > magnitude',
  },
  absolute: {
    units: 'magnitude',
    label: 'Consider change significant if |(x-y)| > magnitude',
  },
  cohen: {
    units: 'standard deviations',
    label: `Consider change significant if the mean has changed by
        this many standard deviations.
        That is |(x-y)/σ| > standard deviations.
        Values from 2.0 to 3.0 work well.`,
  },
  mannwhitneyu: {
    units: 'alpha (α)',
    label: 'Consider change significant if p < α. A typical value is 0.05.',
  },
  stepiness: {
    units: 'stepiness',
    label: 'Consider change significant if stepiness > threshold. A typical value is 0.5.',
  },
};

@customElement('alert-config-sk')
export class AlertConfigSk extends LitElement {
  @property({ type: Object })
  paramset = ParamSet({});

  @property({ type: Object })
  config: Alert = AlertConfigSk.defaultConfig();

  @property({ type: Array })
  key_order: string[] | null = window.perf?.key_order || [];

  private paramkeys: string[] = [];

  private static defaultConfig(): Alert {
    return {
      id_as_string: '-1',
      display_name: 'Name',
      query: '',
      alert: '',
      issue_tracker_component: SerializesToString(''),
      interesting: 0,
      bug_uri_template: '',
      algo: 'kmeans',
      state: 'ACTIVE',
      owner: '',
      step_up_only: false,
      direction: 'BOTH',
      radius: 10,
      k: 50,
      group_by: '',
      sparse: false,
      minimum_num: 0,
      category: 'Experimental',
      step: '',
      action: 'noaction',
    };
  }

  createRenderRoot() {
    return this;
  }

  updated(changedProperties: Map<string | symbol, unknown>) {
    if (changedProperties.has('paramset')) {
      if (this.paramset) {
        this.paramkeys = Object.keys(this.paramset);
        this.paramkeys.sort();
      }
    }
    if (changedProperties.has('config')) {
      if (this.config) {
        this.config.interesting = this.config.interesting ?? window.perf?.interesting ?? 0;
        this.config.radius = this.config.radius ?? window.perf?.radius ?? 0;
      }
    }
  }

  render() {
    return html`
      <h3>Display Name</h3>
      <label for="display-name">Display Name</label>
      <input
        id="display-name"
        type="text"
        .value=${this.config.display_name}
        @change=${(e: InputEvent) => {
          this.config.display_name = (e.target! as HTMLInputElement).value;
        }} />

      <h3>Category</h3>
      <label for="category">Alerts will be grouped by category.</label>
      <input
        id="category"
        type="text"
        .value=${this.config.category}
        @input=${(e: InputEvent) => {
          this.config.category = (e.target! as HTMLInputElement).value;
        }} />

      <h3>Which traces should be monitored</h3>
      <query-chooser-sk
        id="querychooser"
        .paramset=${this.paramset}
        .key_order=${this.key_order}
        current_query=${this.config.query}
        count_url="/_/count"
        @query-change=${(e: CustomEvent<QuerySkQueryChangeEventDetail>) => {
          this.config.query = e.detail.q;
        }}></query-chooser-sk>

      <div>
        <a href="/e/?queries=${encodeURIComponent(this.config.query)}" target="_blank">
          Preview traces that match the query
        </a>
      </div>

      <h3>What triggers an alert</h3>
      <h4>Grouping</h4>
      <label for="grouping">
        Are the traces k-means clustered and Step Detection done on the centroid, or is Step
        Detection done on each trace individually.
      </label>
      <algo-select-sk
        id="grouping"
        algo=${this.config.algo}
        @algo-change=${(e: CustomEvent<AlgoSelectAlgoChangeEventDetail>) => {
          this.config.algo = e.detail.algo;
        }}></algo-select-sk>

      <h4>Step Detection</h4>
      <label for="step">
        Choose the algorithm used to determine if a regression has occurred. The
        <em>Threshold</em> units will change based on the algorithm selection. .
      </label>
      <select-sk
        id="step"
        @selection-changed=${this.stepSelectionChanged}
        .selection=${this.indexFromStep()}>
        <div value="">Original Regression Factor</div>
        <div value="absolute">Absolute</div>
        <div value="const">Const</div>
        <div value="percent">Percent</div>
        <div value="percent_median">Percent Median</div>
        <div value="cohen">Cohen's d</div>
        <div value="mannwhitneyu">Mann-Whitney U (Wilcoxon rank-sum)</div>
        <div value="stepiness">Stepiness</div>
      </select-sk>
      <h4>Threshold</h4>
      <label for="threshold"> ${thresholdDescriptors[this.config.step].label} </label>
      <input
        id="threshold"
        .value=${this.config.interesting.toString()}
        @input=${(e: InputEvent) => {
          this.config.interesting = +(e.target! as HTMLInputElement).value;
        }} />
      ${thresholdDescriptors[this.config.step].units}

      <h4>K</h4>
      <label for="k">
        The number of clusters. Only used when Grouping is K-Means. 0 = use a server chosen value.
      </label>
      <input
        id="k"
        type="number"
        min="0"
        .value=${this.config.k.toString()}
        @input=${(e: InputEvent) => {
          this.config.k = +(e.target! as HTMLInputElement).value;
        }} />

      <h4>Radius</h4>
      <label for="radius"> Number of commits on either side to consider. </label>
      <input
        id="radius"
        type="number"
        min="0"
        .value=${this.config.radius.toString()}
        @input=${(e: InputEvent) => {
          this.config.radius = +(e.target! as HTMLInputElement).value;
        }} />

      <h4>Step Direction</h4>
      <select-sk
        @selection-changed=${(e: CustomEvent<SelectSkSelectionChangedEventDetail>) => {
          this.config.direction = toDirection(
            (e.target! as HTMLDivElement).children[e.detail.selection].getAttribute('value')
          );
        }}>
        <div value="BOTH" ?selected=${this.config.direction === 'BOTH'}>
          Either step up or step down trigger an alert.
        </div>
        <div value="UP" ?selected=${this.config.direction === 'UP'}>Step up triggers an alert.</div>
        <div value="DOWN" ?selected=${this.config.direction === 'DOWN'}>
          Step down triggers an alert.
        </div>
      </select-sk>

      <h4>Minimum</h4>
      <label for="min"> Minimum number of interesting traces to trigger an alert. </label>
      <input
        id="min"
        type="number"
        .value=${this.config.minimum_num.toString()}
        @input=${(e: InputEvent) => {
          this.config.minimum_num = +(e.target! as HTMLInputElement).value;
        }} />

      <h4>Sparse</h4>
      <checkbox-sk
        id="sparse"
        ?checked=${this.config.sparse}
        @input=${(e: InputEvent) => {
          this.config.sparse = (e.target! as HTMLInputElement).checked;
        }}
        label="Data is sparse, so only include commits that have data."></checkbox-sk>

      ${window.perf.need_alert_action === true
        ? html`
            <h3>What action to take</h3>
            <label for="action"> Choose the action to take if a regression has occurred. </label>
            <select-sk
              id="action"
              @selection-changed=${this.alertActionChanged}
              .selection=${this.config.action === 'report'
                ? 1
                : this.config.action === 'bisect'
                  ? 2
                  : 0}>
              <div value="noaction" ?selected=${this.config.action === 'noaction'}>No action.</div>
              <div value="report" ?selected=${this.config.action === 'report'}>
                File an issue and assign to corresponding component.
              </div>
              <div value="bisect" ?selected=${this.config.action === 'bisect'}>
                Run Pinpoint Bisection and file an issue if culprit CL is found.
              </div>
            </select-sk>
          `
        : html``}
      ${window.perf.notifications === 'html_email'
        ? html`
            <h3>Where are alerts sent</h3>
            <label for="sent"> Alert Destination: Comma separated list of email addresses. </label>
            <input
              id="sent"
              .value=${this.config.alert}
              @input=${(e: InputEvent) => {
                this.config.alert = (e.target! as HTMLInputElement).value;
              }} />
            <button @click=${this.testAlert}>Test</button>
            <spinner-sk id="alertSpinner"></spinner-sk>
          `
        : html``}
      ${window.perf.notifications === 'markdown_issuetracker'
        ? html`
            <h3>Which component should receive issues for regressions that are found.</h3>
            <label for="component"> Issue Tracker Component ID. </label>
            <input
              type="text"
              id="component"
              inputmode="numeric"
              pattern="\\d+"
              .value=${this.config.issue_tracker_component}
              @input=${(e: InputEvent) => {
                const valMsg = (e.target! as HTMLInputElement).validationMessage;
                if (valMsg !== '') {
                  errorMessage(valMsg, 3000);
                  return;
                }
                this.config.issue_tracker_component = SerializesToString(
                  (e.target! as HTMLInputElement).value
                );
              }} />
            <button @click=${this.testAlert}>Test</button>
            <spinner-sk id="alertSpinner"></spinner-sk>
          `
        : html``}
      ${window.perf.notifications === 'markdown_issuetracker'
        ? html``
        : html`<h3>Where are bugs filed</h3>
            <label for="template">
              Bug URI Template: {cluster_url}, {commit_url}, and {message}.
            </label>
            <input
              id="template"
              .value=${this.config.bug_uri_template}
              @input=${(e: InputEvent) => {
                this.config.bug_uri_template = (e.target! as HTMLInputElement).value;
              }} />
            <button @click=${this.testBugTemplate}>Test</button>
            <spinner-sk id="bugSpinner"></spinner-sk> `}

      <h3>Who owns this alert</h3>
      <label for="owner">Email address of owner.</label>
      <input
        id="owner"
        .value=${this.config.owner}
        @input=${(e: InputEvent) => {
          this.config.owner = (e.target! as HTMLInputElement).value;
        }} />

      ${this._groupBy()}

      <h3>Status</h3>
      <select-sk
        .selection=${this.config.state === 'ACTIVE' ? 0 : 1}
        @selection-changed=${(e: CustomEvent<SelectSkSelectionChangedEventDetail>) => {
          this.config.state = toConfigState(
            (e.target! as HTMLDivElement).children[e.detail.selection].getAttribute('value')
          );
        }}>
        <div value="ACTIVE" title="Clusters that match this will generate alerts.">Active</div>
        <div value="DELETED" title="Currently inactive.">Deleted</div>
      </select-sk>
    `;
  }

  private _groupBy(): TemplateResult {
    if (!window.perf?.display_group_by) {
      return html``;
    }
    return html`
      <h3>Group By</h3>
      <label for="groupby"> Group clusters by these parameters. (Multiselect) </label>
      <multi-select-sk
        @selection-changed=${(e: CustomEvent<MultiSelectSkSelectionChangedEventDetail>) => {
          this.config.group_by = e.detail.selection.map((i) => this.paramkeys[i]).join(',');
        }}
        id="groupby">
        ${this._groupByChoices()}
      </multi-select-sk>
    `;
  }

  private _groupByChoices(): TemplateResult[] {
    const groups = this.config.group_by.split(',');
    return this.paramkeys.map((p) => html`<div ?selected=${groups.indexOf(p) !== -1}>${p}</div>`);
  }

  private indexFromStep(): number {
    const stepSelectSk = this.querySelector<SelectSk>('#step');
    if (!stepSelectSk) {
      return -1;
    }
    const children = stepSelectSk.children;
    for (let i = 0; i < children.length; i++) {
      if (children[i].getAttribute('value') === this.config.step) {
        return i;
      }
    }
    return -1;
  }

  private stepSelectionChanged(e: CustomEvent<SelectSkSelectionChangedEventDetail>) {
    const valueAsString = (e.target! as HTMLDivElement).children[e.detail.selection].getAttribute(
      'value'
    );
    this.config.step = valueAsString as StepDetection;
    this.requestUpdate();
  }

  private alertActionChanged(e: CustomEvent<SelectSkSelectionChangedEventDetail>) {
    const valueAsString = (e.target! as HTMLDivElement).children[e.detail.selection].getAttribute(
      'value'
    );
    this.config.action = valueAsString as AlertAction;
    this.requestUpdate();
  }

  private testBugTemplate() {
    const bugSpinner = this.querySelector<SpinnerSk>('#bugSpinner')!;
    bugSpinner.active = true;
    const body: TryBugRequest = {
      bug_uri_template: this.config.bug_uri_template,
    };
    fetch('/_/alert/bug/try', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((json: TryBugResponse) => {
        bugSpinner.active = false;
        if (json.url) {
          window.open(json.url, '_blank');
        }
      })
      .catch((msg) => {
        bugSpinner.active = false;
        errorMessage(msg);
      });
  }

  private async testAlert(): Promise<void> {
    const alertSpinner = this.querySelector<SpinnerSk>('#alertSpinner')!;
    alertSpinner.active = true;
    const body: Alert = this.config;

    try {
      const resp = await fetch('/_/alert/notify/try', {
        method: 'POST',
        body: JSON.stringify(body),
        headers: {
          'Content-Type': 'application/json',
        },
      });
      if (!resp.ok) {
        const msg = await resp.text();
        errorMessage(`${resp.statusText}: ${msg}`);
      }
    } finally {
      alertSpinner.active = false;
    }
  }
}
