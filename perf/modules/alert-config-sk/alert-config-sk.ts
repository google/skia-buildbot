/**
 * @module module/alert-config-sk
 * @description <h2><code>alert-config-sk</code></h2>
 *
 * Control that allows editing an alert.Config.
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';

import 'elements-sk/checkbox-sk';
import 'elements-sk/multi-select-sk';
import 'elements-sk/select-sk';
import 'elements-sk/spinner-sk';
import 'elements-sk/styles/buttons';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { SpinnerSk } from 'elements-sk/spinner-sk/spinner-sk';
import {
  SelectSk,
  SelectSkSelectionChangedEventDetail,
} from 'elements-sk/select-sk/select-sk';
import { MultiSelectSkSelectionChangedEventDetail } from 'elements-sk/multi-select-sk/multi-select-sk';
import { errorMessage } from '../errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
  ParamSet,
  Alert,
  Direction,
  StepDetection,
  ConfigState,
  TryBugRequest,
  TryBugResponse,
} from '../json';
import { QuerySkQueryChangeEventDetail } from '../../../infra-sk/modules/query-sk/query-sk';
import { AlgoSelectAlgoChangeEventDetail } from '../algo-select-sk/algo-select-sk';

import '../algo-select-sk';
import '../query-chooser-sk';

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
    label:
      'Consider change significant if p < α. A typical value is 0.05.',
  },
};

export class AlertConfigSk extends ElementSk {
  private _paramset: ParamSet = {};

  private _config: Alert;

  private _key_order: string[] | null = [];

  private paramkeys: string[] = [];

  private bugSpinner: SpinnerSk | null = null;

  private alertSpinner: SpinnerSk | null = null;

  private stepSelectSk: SelectSk | null = null;

  constructor() {
    super(AlertConfigSk.template);
    this._paramset = {};
    this.paramkeys = [];
    this._config = {
      id_as_string: '-1',
      display_name: 'Name',
      query: '',
      alert: '',
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
    };
  }

  private static template = (ele: AlertConfigSk) => html`
    <h3>Display Name</h3>
    <label for="display-name">Display Name</label>
    <input
      id="display-name"
      type="text"
      .value=${ele._config.display_name}
      @change=${(e: InputEvent) => (ele._config.display_name = (e.target! as HTMLInputElement).value)}
    />

    <h3>Category</h3>
    <label for="category">Alerts will be grouped by category.</label>
    <input
      id="category"
      type="text"
      .value=${ele._config.category}
      @input=${(e: InputEvent) => (ele._config.category = (e.target! as HTMLInputElement).value)}
    />

    <h3>Which traces should be monitored</h3>
    <query-chooser-sk
      id="querychooser"
      .paramset=${ele.paramset}
      .key_order=${ele.key_order}
      current_query=${ele._config.query}
      count_url="/_/count/"
      @query-change=${(e: CustomEvent<QuerySkQueryChangeEventDetail>) => (ele._config.query = e.detail.q)}
    ></query-chooser-sk>

    <div>
      <a
        href="/e/?queries=${encodeURIComponent(ele._config.query)}"
        target="_blank"
      >
        Preview traces that match the query
      </a>
    </div>

    <h3>What triggers an alert</h3>
    <h4>Grouping</h4>
    <label for="grouping">
      Are the traces k-means clustered and Step Detection done on the centroid,
      or is Step Detection done on each trace individually.
    </label>
    <algo-select-sk
      id="grouping"
      algo=${ele._config.algo}
      @algo-change=${(e: CustomEvent<AlgoSelectAlgoChangeEventDetail>) => (ele._config.algo = e.detail.algo)}
    ></algo-select-sk>

    <h4>Step Detection</h4>
    <label for="step">
      Choose the algorithm used to determine if a regression has occurred. The
      <em>Threshold</em> units will change based on the algorithm selection. .
    </label>
    <select-sk
      id="step"
      @selection-changed=${ele.stepSelectionChanged}
      .selection=${ele.indexFromStep()}
    >
      <div value="">Original Regression Factor</div>
      <div value="absolute">Absolute</div>
      <div value="percent">Percent</div>
      <div value="cohen">Cohen's d</div>
      <div value="mannwhitneyu">Mann-Whitney U (Wilcoxon rank-sum)</div>
    </select-sk>
    <h4>Threshold</h4>
    <label for="threshold">
      ${thresholdDescriptors[ele._config.step].label}
    </label>
    <input
      id="threshold"
      .value=${ele._config.interesting.toString()}
      @input=${(e: InputEvent) => (ele._config.interesting = +(e.target! as HTMLInputElement).value)}
    />
    ${thresholdDescriptors[ele._config.step].units}

    <h4>K</h4>
    <label for="k">
      The number of clusters. Only used when Grouping is K-Means. 0 = use a
      server chosen value.
    </label>
    <input
      id="k"
      type="number"
      min="0"
      .value=${ele._config.k.toString()}
      @input=${(e: InputEvent) => (ele._config.k = +(e.target! as HTMLInputElement).value)}
    />

    <h4>Radius</h4>
    <label for="radius">
      Number of commits on either side to consider.
    </label>
    <input
      id="radius"
      type="number"
      min="0"
      .value=${ele._config.radius.toString()}
      @input=${(e: InputEvent) => (ele._config.radius = +(e.target! as HTMLInputElement).value)}
    />

    <h4>Step Direction</h4>
    <select-sk
      @selection-changed=${(
    e: CustomEvent<SelectSkSelectionChangedEventDetail>,
  ) => (ele._config.direction = toDirection(
    (e.target! as HTMLDivElement).children[
      e.detail.selection
    ].getAttribute('value'),
  ))}
    >
      <div value="BOTH" ?selected=${ele._config.direction === 'BOTH'}>
        Either step up or step down trigger an alert.
      </div>
      <div value="UP" ?selected=${ele._config.direction === 'UP'}>
        Step up triggers an alert.
      </div>
      <div value="DOWN" ?selected=${ele._config.direction === 'DOWN'}>
        Step down triggers an alert.
      </div>
    </select-sk>

    <h4>Minimum</h4>
    <label for="min">
      Minimum number of interesting traces to trigger an alert.
    </label>
    <input
      id="min"
      type="number"
      .value=${ele._config.minimum_num.toString()}
      @input=${(e: InputEvent) => (ele._config.minimum_num = +(e.target! as HTMLInputElement).value)}
    />

    <h4>Sparse</h4>
    <checkbox-sk
      ?checked=${ele._config.sparse}
      @input=${(e: InputEvent) => (ele._config.sparse = (e.target! as HTMLInputElement).checked)}
      label="Data is sparse, so only include commits that have data."
    ></checkbox-sk>

    <h3>Where are alerts sent</h3>
    <label for="sent">
      Alert Destination: Comma separated list of email addresses.
    </label>
    <input
      id="sent"
      .value=${ele._config.alert}
      @input=${(e: InputEvent) => (ele._config.alert = (e.target! as HTMLInputElement).value)}
    />
    <button @click=${ele.testAlert}>Test</button>
    <spinner-sk id="alertSpinner"></spinner-sk>

    <h3>Where are bugs filed</h3>
    <label for="template">
      Bug URI Template: {cluster_url}, {commit_url}, and {message}.
    </label>
    <input
      id="template"
      .value=${ele._config.bug_uri_template}
      @input=${(e: InputEvent) => (ele._config.bug_uri_template = (e.target! as HTMLInputElement).value)}
    />
    <button @click=${ele.testBugTemplate}>Test</button>
    <spinner-sk id="bugSpinner"></spinner-sk>

    <h3>Who owns this alert</h3>
    <label for="owner">Email address of owner.</label>
    <input
      id="owner"
      .value=${ele._config.owner}
      @input=${(e: InputEvent) => (ele._config.owner = (e.target! as HTMLInputElement).value)}
    />

    <h3>Group By</h3>
    <label for="groupby">
      Group clusters by these parameters. (Multiselect)
    </label>
    <multi-select-sk
      @selection-changed=${(
    e: CustomEvent<MultiSelectSkSelectionChangedEventDetail>,
  ) => (ele._config.group_by = e.detail.selection
    .map((i) => ele.paramkeys[i])
    .join(','))}
      id="groupby"
    >
      ${AlertConfigSk._groupByChoices(ele)}
    </multi-select-sk>

    <h3>Status</h3>
    <select-sk
      .selection=${ele._config.state === 'ACTIVE' ? 0 : 1}
      @selection-changed=${(
      e: CustomEvent<SelectSkSelectionChangedEventDetail>,
    ) => (ele._config.state = toConfigState(
      (e.target! as HTMLDivElement).children[
        e.detail.selection
      ].getAttribute('value'),
    ))}
    >
      <div
        value="ACTIVE"
        title="Clusters that match this will generate alerts."
      >
        Active
      </div>
      <div value="DELETED" title="Currently inactive.">Deleted</div>
    </select-sk>
  `;

  private static _groupByChoices = (ele: AlertConfigSk) => {
    const groups = ele._config.group_by.split(',');
    return ele.paramkeys.map(
      (p) => html`<div ?selected=${groups.indexOf(p) !== -1}>${p}</div>`,
    );
  };

  connectedCallback(): void {
    super.connectedCallback();
    this._upgradeProperty('config');
    this._upgradeProperty('paramset');
    if (window.sk && window.sk.perf && window.sk.perf.key_order) {
      this._key_order = window.sk.perf.key_order;
    }
    this._render();
    this.bugSpinner = this.querySelector('#bugSpinner');
    this.alertSpinner = this.querySelector('#alertSpinner');
    this.stepSelectSk = this.querySelector('#step');
  }

  /**
   * Returns the index of the select-sk child that should be selected based on
   * the value of _config.step.
   */
  private indexFromStep(): number {
    if (!this.stepSelectSk) {
      return -1;
    }
    const children = this.stepSelectSk!.children;
    for (let i = 0; i < children.length; i++) {
      if (children[i].getAttribute('value') === this._config.step) {
        return i;
      }
    }
    return -1;
  }

  private stepSelectionChanged(
    e: CustomEvent<SelectSkSelectionChangedEventDetail>,
  ) {
    const valueAsString = (e.target! as HTMLDivElement).children[
      e.detail.selection
    ].getAttribute('value');
    this._config.step = valueAsString as StepDetection;
    this._render();
  }

  private testBugTemplate() {
    this.bugSpinner!.active = true;
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
        this.bugSpinner!.active = false;
        if (json.url) {
          // Open the bug reporting page in a new window.
          window.open(json.url, '_blank');
        }
      })
      .catch((msg) => {
        this.bugSpinner!.active = false;
        errorMessage(msg);
      });
  }

  private testAlert() {
    this.alertSpinner!.active = true;
    const body: Alert = this.config;
    fetch('/_/alert/notify/try', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(() => {
        this.alertSpinner!.active = false;
      })
      .catch((msg) => {
        this.alertSpinner!.active = false;
        errorMessage(msg);
      });
  }

  /** @prop paramset {string} A serialized paramtools.ParamSet. */
  get paramset(): ParamSet {
    return this._paramset;
  }

  set paramset(val: ParamSet) {
    if (val === undefined) {
      return;
    }
    this._paramset = val;
    this.paramkeys = Object.keys(val);
    this.paramkeys.sort();
    this._render();
  }

  /** @prop config {Object} A serialized alerts.Alert. */
  get config(): Alert {
    return this._config;
  }

  set config(val: Alert) {
    if (!val || Object.keys(val).length === 0) {
      return;
    }
    this._config = val;
    if (this._config.interesting === 0) {
      this._config.interesting = window.sk.perf.interesting;
    }
    if (this._config.radius === 0) {
      this._config.radius = window.sk.perf.radius;
    }
    this._render();
  }

  /** @prop key_order {string} The order of keys, passed to query-sk. */
  get key_order(): string[] | null {
    return this._key_order;
  }

  set key_order(val: string[] | null) {
    if (val === undefined) {
      return;
    }
    this._key_order = val;
    this._render();
  }
}

define('alert-config-sk', AlertConfigSk);
