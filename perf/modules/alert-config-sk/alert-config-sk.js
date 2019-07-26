/**
 * @module module/alert-config-sk
 * @description <h2><code>alert-config-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { html, render } from 'lit-html'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'

import 'elements-sk/checkbox-sk'
import 'elements-sk/multi-select-sk'
import 'elements-sk/select-sk'
import 'elements-sk/spinner-sk'
import 'elements-sk/styles/buttons'

import '../algo-select-sk'
import '../query-chooser-sk'

const STATUS_MAP = {
  'ACTIVE': 0,
  'DELETED': 0,
}

const _groupByChoices = (ele) => ele._paramkeys.map((p) => html`<div>${p}</div>`);

const template = (ele) => html`
  <h3>Display Name</h3>
  <label for=display-name>Display Name</label>
  <input id=display-name type=text value=${ele.config.display_name} @input=${(e) => ele.config.display_name=e.target.value}>
  <h3>Category</h3>
  <label for=category>Alerts will be grouped by category.</label>
  <input id=category type=text value=${ele.config.category} @input=${(e) => ele.config.category=e.target.value}>
  <h3>Which traces should be monitored</h3>
  <query-chooser-sk id=querychooser paramset=${paramset} current_query=${ele.config.query} count_url='/_/count/' @query-change=_queryChange></query-chooser-sk>
  <h3>What triggers an alert</h3>
  <h4>Algorithm</h4>
  <algo-select-sk @algo-change=${ele._onAlgoChange} algo=${ele.config.algo}></algo-select-sk>
  <h4>K</h4>
  <label for=k>The number of clusters. Only used in kmeans. 0 = use a server chosen value. (For Tail algorithm, K is the jump percentage.)</label>
  <input id=k type=number min=0 value=${ele.config.k}>
  <h4>Radius</h4>
  <label for=radius>Number of commits on either side to consider. 0 = use a server chosen value. (For Tail algorithm, we only consider 2*Radius commits on the left side.)</label>
  <input id=radius type=number min=0 value=${ele.config.radius}>
  <h4>Step Direction</h4>
  <select-sk .selection=${config.direction} @selection-changed=${(e) => ele.config.direction=e.detail.selection}>
    <div value=BOTH>Either step up or step down trigger an alert.</div>
    <div value=UP>Step up triggers an alert.</div>
    <div value=DOWN>Step down triggers an alert.</div>
  </select-sk>
  <h4>Threshold</h4>
  <label for=threshold>Interesting Threshold for clusters to be interesting. (Tail algorithm use this 1/Threshold as the min/max quantile.)</label>
  <input id=threshold type=number min=1 max=500  value=${ele.config.interesting} @input=${(e) => ele.config.interesting=+e.target.value}>
  <h4>Minimum</h4>
  <label for=min>Minimum number of interesting traces to trigger an alert.</label>
  <input id=min type=number value=${ele.config.minimum_num} @input=${(e) => ele.config.minimum_num=+e.target.value}>
  <h4>Sparse</h4>
  <checkbox-sk checked=${ele.config.sparse} @input=${(e) => ele.config.sparse = e.target.checked} label='Data is sparse, so only include commits that have data.'</checkbox-sk>
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
  <multi-select-sk id=groupby .selection=${ele._groupsFrom()} @selection-changed=${ele._groupByChanged}>
    <div title="No grouping.">(none)</div>
    ${_groupByChoices(ele)}
  </multi-select-sk>
  <h3>Status</h3>
  <select-sk selection=${STATUS_MAP[ele.config.state]} @selection-chenged=${(e) => ele.config.state=e.target.getAttribute('value')}>
    <div value=ACTIVE title="Clusters that match this will generate alerts.">Active</div>
    <div value=DELETED title="Currently inactive.">Deleted</div>
  </select-sk>
  `;

window.customElements.define('alert-config-sk', class extends ElementSk {
  constructor() {
    super(template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  disconnectedCallback() {
  }

});
