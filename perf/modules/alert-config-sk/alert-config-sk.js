/**
 * @module module/alert-config-sk
 * @description <h2><code>alert-config-sk</code></h2>
 *
 * Control that allows editing an alert.Config.
 *
 */
import { html, render } from 'lit-html'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'

import 'elements-sk/checkbox-sk'
import 'elements-sk/multi-select-sk'
import 'elements-sk/select-sk'
import 'elements-sk/spinner-sk'
import 'elements-sk/styles/buttons'
import { errorMessage } from 'elements-sk/errorMessage'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'

import '../algo-select-sk'
import '../query-chooser-sk'

const _groupByChoices = (ele) => {
  const groups = ele.config.group_by.split(',');
  return ele._paramkeys.map((p) => html`<div ?selected=${groups.indexOf(p) != -1}>${p}</div>`);
}

const template = (ele) => html`
  <h3>Display Name</h3>
  <label for=display-name>Display Name</label>
  <input id=display-name type=text value=${ele.config.display_name} @input=${(e) => ele.config.display_name=e.target.value}>
  <h3>Category</h3>
  <label for=category>Alerts will be grouped by category.</label>
  <input id=category type=text value=${ele.config.category} @input=${(e) => ele.config.category=e.target.value}>
  <h3>Which traces should be monitored</h3>
  <query-chooser-sk id=querychooser .paramset=${ele.paramset} .key_order=${ele.key_order} current_query=${ele.config.query} count_url='/_/count/' @query-change=${(e) => ele.config.query=e.detail.q}></query-chooser-sk>
  <h3>What triggers an alert</h3>
  <h4>Algorithm</h4>
  <algo-select-sk algo=${ele.config.algo} @algo-change=${(e) => ele.config.algo=e.detail.algo}></algo-select-sk>
  <h4>K</h4>
  <label for=k>The number of clusters. Only used in kmeans. 0 = use a server chosen value. (For Tail algorithm, K is the jump percentage.)</label>
  <input id=k type=number min=0 value=${ele.config.k} @input=${(e) => ele.config.k=+e.target.value}>
  <h4>Radius</h4>
  <label for=radius>Number of commits on either side to consider. 0 = use a server chosen value. (For Tail algorithm, we only consider 2*Radius commits on the left side.)</label>
  <input id=radius type=number min=0 value=${ele.config.radius} @input=${(e) => ele.config.radius=+e.target.value}>
  <h4>Step Direction</h4>
  <select-sk @selection-changed=${(e) => ele.config.direction=e.target.children[e.detail.selection].getAttribute('value')}>
    <!-- TODO(jcgregorio) Go back to using select-sk.selection once we've excised all Polymer. -->
    <div value=BOTH ?selected=${ele.config.direction === 'BOTH'} >Either step up or step down trigger an alert.</div>
    <div value=UP ?selected=${ele.config.direction === 'UP'}>Step up triggers an alert.</div>
    <div value=DOWN ?selected=${ele.config.direction === 'DOWN'}>Step down triggers an alert.</div>
  </select-sk>
  <h4>Threshold</h4>
  <label for=threshold>Interesting Threshold for clusters to be interesting. (Tail algorithm use this 1/Threshold as the min/max quantile.)</label>
  <input id=threshold type=number min=1 max=500  value=${ele.config.interesting} @input=${(e) => ele.config.interesting=+e.target.value}>
  <h4>Minimum</h4>
  <label for=min>Minimum number of interesting traces to trigger an alert.</label>
  <input id=min type=number value=${ele.config.minimum_num} @input=${(e) => ele.config.minimum_num=+e.target.value}>
  <h4>Sparse</h4>
  <checkbox-sk ?checked=${ele.config.sparse} @input=${(e) => ele.config.sparse = e.target.checked} label='Data is sparse, so only include commits that have data.'></checkbox-sk>
  <h3>Where are alerts sent</h3>
  <label for=sent>Alert Destination: Comma separated list of email addresses.</label>
  <input id=sent value=${ele.config.alert} @input=${(e) => ele.config.alert=e.target.value}>
  <button @click=${ele._testAlert}>Test</button>
  <spinner-sk id=alertSpinner></spinner-sk>
  <h3>Where are bugs filed</h3>
  <label for=template>Bug URI Template: {cluster_url}, {commit_url}, and {message}.</label>
  <input id=template value=${ele.config.bug_uri_template} @input=${(e) => ele.config.bug_uri_template=e.target.value}>
  <button @click=${ele._testBugTemplate}>Test</button>
  <spinner-sk id=bugSpinner></spinner-sk>
  <h3>Who owns this alert</h3>
  <label for=owner>Email address of owner.</label>
  <input id=owner value=${ele.config.owner} @input=${(e) => ele.config.owner=e.target.value}>
  <h3>Group By</h3>
  <label for=groupby>Group clusters by these parameters. (Multiselect)</label>
  <multi-select-sk
    @selection-changed=${(e) => ele.config.group_by = e.detail.selection.map((i) => ele._paramkeys[i]).join(',')}
    id=groupby
    >
    ${_groupByChoices(ele)}
  </multi-select-sk>
  <h3>Status</h3>
  <select-sk @selection-changed=${(e) => ele.config.state=e.target.children[e.detail.selection].getAttribute('value')}>
    <div value=ACTIVE ?selected=${ele.config.state === 'ACTIVE'} title='Clusters that match this will generate alerts.'>Active</div>
    <div value=DELETED ?selected=${ele.config.state === 'DELETED'} title='Currently inactive.'>Deleted</div>
  </select-sk>
  `;

window.customElements.define('alert-config-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._paramset = {};
    this._paramkeys = [];
    this._config = {
      query: '',
      alert: '',
      interesting: 25,
      bug_uri_template: '',
      algo: 'stepfit',
      owner: '',
      minimum_num: 2,
      category: '',
      group_by: '',
    };
    this._key_order = [];
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
      headers:{
        'Content-Type': 'application/json'
      }
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
      headers:{
        'Content-Type': 'application/json'
      }
    }).then(jsonOrThrow).then((json) => {
      this._alertSpinner.active = false;
    }).catch((msg) => {
      this._alertSpinner.active = false;
      errorMessage(msg);
    });
  }

  /** @prop paramset {string} A serialized paramtools.ParamSet. */
  get paramset() { return this._paramset }
  set paramset(val) {
    if (val === undefined) {
      return
    }
    this._paramset = val;
    this._paramkeys = Object.keys(val);
    this._paramkeys.sort();
    this._render();
  }

  /** @prop config {string} A serialized alerts.Config. */
  get config() { return this._config }
  set config(val) {
    if (val === undefined || Object.keys(val).length === 0) {
      return
    }
    this._config = val;
    this._render();
  }

  /** @prop key_order {string} The order of keys, passed to query-sk. */
  get key_order() { return this._key_order }
  set key_order(val) {
    if (val === undefined) {
      return
    }
    this._key_order = val;
    this._render();
  }

});
