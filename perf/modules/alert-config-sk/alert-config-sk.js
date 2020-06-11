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
import { errorMessage } from 'elements-sk/errorMessage';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import '../algo-select-sk';
import '../query-chooser-sk';

const _groupByChoices = (ele) => {
  const groups = ele._config.group_by.split(',');
  return ele._paramkeys.map((p) => html`<div ?selected=${groups.indexOf(p) !== -1}>${p}</div>`);
};

const template = (ele) => html`
  <h3>Display Name</h3>
  <label for=display-name>Display Name</label>
  <input id=display-name type=text .value=${ele._config.display_name} @change=${(e) => ele._config.display_name = e.target.value}>

  <h3>Category</h3>
  <label for=category>Alerts will be grouped by category.</label>
  <input id=category type=text .value=${ele._config.category} @input=${(e) => ele._config.category = e.target.value}>

  <h3>Which traces should be monitored</h3>
  <query-chooser-sk id=querychooser .paramset=${ele.paramset} .key_order=${ele.key_order} current_query=${ele._config.query} count_url='/_/count/' @query-change=${(e) => ele._config.query = e.detail.q}></query-chooser-sk>

  <div>
    <a href="/e/?queries=${encodeURIComponent(ele._config.query)}" target=_blank>Preview traces that match the query</a>
  </div>

  <h3>What triggers an alert</h3>
  <h4>Grouping</h4>
  <label for=grouping>Are the traces k-means clustered and Step Detection  done on
    the centroid, or is Step Detection done on each trace individually.
  </label>
<algo-select-sk id=grouping algo=${ele._config.algo} @algo-change=${(e) => ele._config.algo = e.detail.algo}></algo-select-sk>

  <h4>Step Detection</h4>
  <label for=step>Choose the algorithm used to determine if a regression has occurred.
    The actual value is set in <em>Threshold</em>.
  </label>
  <select-sk id=step @selection-changed=${(e) => ele._config.step = e.target.children[e.detail.selection].getAttribute('value')}>
    <div value="" ?selected=${ele._config.step === ''} >Regression = Step Size/Variance. This is the original regression factor.</div>
    <div value=absolute ?selected=${ele._config.step === 'absolute'}>A change in absolute magnitude. Threshold is the minimum difference to trigger an alert.</div>
    <div value=percent ?selected=${ele._config.step === 'percent'}>A change by percent. Threshold is a value in [0.0, 1.0] and the minimum difference to trigger an alert.</div>
    <div value=cohen ?selected=${ele._config.step === 'cohen'}>Use Cohen's d method to detect a change. Threshold is the standard deviations that the mean must move to trigger an alert.</div>
  </select-sk>
  <h4>Threshold</h4>
  <label for=threshold>The threshold for Step Detection to trigger an alert. The meaning of the value and meaningful range depends on the algorithm chosen for <em>Step Detection</em>. </label>
  <input id=threshold type=number min=1 max=500 .value=${ele._config.interesting} @input=${(e) => ele._config.interesting = +e.target.value}>

  <h4>K</h4>
  <label for=k>The number of clusters. Only used when Grouping is K-Means. 0 = use a server chosen value.</label>
  <input id=k type=number min=0 .value=${ele._config.k} @input=${(e) => ele._config.k = +e.target.value}>

  <h4>Radius</h4>
  <label for=radius>Number of commits on either side to consider. 0 = use a server chosen value. </label>
  <input id=radius type=number min=0 .value=${ele._config.radius} @input=${(e) => ele._config.radius = +e.target.value}>

  <h4>Step Direction</h4>
  <select-sk @selection-changed=${(e) => ele._config.direction = e.target.children[e.detail.selection].getAttribute('value')}>
    <div value=BOTH ?selected=${ele._config.direction === 'BOTH'} >Either step up or step down trigger an alert.</div>
    <div value=UP ?selected=${ele._config.direction === 'UP'}>Step up triggers an alert.</div>
    <div value=DOWN ?selected=${ele._config.direction === 'DOWN'}>Step down triggers an alert.</div>
  </select-sk>

  <h4>Minimum</h4>
  <label for=min>Minimum number of interesting traces to trigger an alert.</label>
  <input id=min type=number .value=${ele._config.minimum_num} @input=${(e) => ele._config.minimum_num = +e.target.value}>

  <h4>Sparse</h4>
  <checkbox-sk ?checked=${ele._config.sparse} @input=${(e) => ele._config.sparse = e.target.checked} label='Data is sparse, so only include commits that have data.'></checkbox-sk>

  <h3>Where are alerts sent</h3>
  <label for=sent>Alert Destination: Comma separated list of email addresses.</label>
  <input id=sent .value=${ele._config.alert} @input=${(e) => ele._config.alert = e.target.value}>
  <button @click=${ele._testAlert}>Test</button>
  <spinner-sk id=alertSpinner></spinner-sk>

  <h3>Where are bugs filed</h3>
  <label for=template>Bug URI Template: {cluster_url}, {commit_url}, and {message}.</label>
  <input id=template .value=${ele._config.bug_uri_template} @input=${(e) => ele._config.bug_uri_template = e.target.value}>
  <button @click=${ele._testBugTemplate}>Test</button>
  <spinner-sk id=bugSpinner></spinner-sk>

  <h3>Who owns this alert</h3>
  <label for=owner>Email address of owner.</label>
  <input id=owner .value=${ele._config.owner} @input=${(e) => ele._config.owner = e.target.value}>

  <h3>Group By</h3>
  <label for=groupby>Group clusters by these parameters. (Multiselect)</label>
  <multi-select-sk
    @selection-changed=${(e) => ele._config.group_by = e.detail.selection.map((i) => ele._paramkeys[i]).join(',')}
    id=groupby
    >
    ${_groupByChoices(ele)}
  </multi-select-sk>

  <h3>Status</h3>
  <select-sk .selection=${ele._config.state === 'ACTIVE' ? 0 : 1} @selection-changed=${(e) => ele._config.state = e.target.children[e.detail.selection].getAttribute('value')}>
    <div value=ACTIVE title='Clusters that match this will generate alerts.'>Active</div>
    <div value=DELETED title='Currently inactive.'>Deleted</div>
  </select-sk>
  `;

define('alert-config-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._paramset = {};
    this._paramkeys = [];
    this._config = {
      id: -1,
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
    };
    this._key_order = window.sk.perf.key_order;
  }

  connectedCallback() {
    super.connectedCallback();
    this._upgradeProperty('config');
    this._upgradeProperty('paramset');
    this._render();
    this._bugSpinner = this.querySelector('#bugSpinner');
    this._alertSpinner = this.querySelector('#alertSpinner');
  }

  _testBugTemplate() {
    this._bugSpinner.active = true;
    const body = {
      bug_uri_template: this.config.bug_uri_template,
    };
    fetch('/_/alert/bug/try', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    }).then(jsonOrThrow).then((json) => {
      this._bugSpinner.active = false;
      if (json.url) {
        // Open the bug reporting page in a new window.
        window.open(json.url, '_blank');
      }
    }).catch((msg) => {
      this._bugSpinner.active = false;
      errorMessage(msg);
    });
  }

  _testAlert() {
    this._alertSpinner.active = true;
    const body = {
      alert: this.config.alert,
    };
    fetch('/_/alert/notify/try', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    }).then(jsonOrThrow).then(() => {
      this._alertSpinner.active = false;
    }).catch((msg) => {
      this._alertSpinner.active = false;
      errorMessage(msg);
    });
  }

  /** @prop paramset {string} A serialized paramtools.ParamSet. */
  get paramset() { return this._paramset; }

  set paramset(val) {
    if (val === undefined) {
      return;
    }
    this._paramset = val;
    this._paramkeys = Object.keys(val);
    this._paramkeys.sort();
    this._render();
  }

  /** @prop config {Object} A serialized alerts.Config. */
  get config() { return this._config; }

  set config(val) {
    if (val === undefined || Object.keys(val).length === 0) {
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
  get key_order() { return this._key_order; }

  set key_order(val) {
    if (val === undefined) {
      return;
    }
    this._key_order = val;
    this._render();
  }
});
