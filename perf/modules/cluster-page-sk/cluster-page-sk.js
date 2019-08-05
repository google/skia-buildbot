/**
 * @module module/cluster-page-sk
 * @description <h2><code>cluster-page-sk</code></h2>
 *
 *   The top level element for clustering traces.
 *
 */
import { html, render } from 'lit-html'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'

const _summaryRows = (ele) => {
  const ret = ele._summaries.map((summary) => {
    return html`<cluster-summary2-sk .full_summary=${super} notriage></cluster-summary2-sk>`;
  });
  if (!ret.length) {
    ret.push(html`
      <p class=info>
        No clusters found.
      </p>
    `);
  }
  return ret;
}

const template = (ele) => html`
  <h2>Commit</h2>
  <h3>Appears in Date Range</h3>
  <div class=day-range-with-spinner>
    <day-range-sk id=range @day-range-change=${ele._rangeChange}></day-range-sk>
    <spinner-sk id=day-range-spinner></spinner-sk>
  </div>
  <h3>Commit</h3>
  <div>
    <commit-detail-picker-sk @commit-selected=${ele._commitSelected} id=commit></commit-picker-sk>
  </div>

  <h2>Algorithm</h2>
  <algo-select-sk @algo-change=${_algoChange} algo=${ele.state.algo}></algo-select-sk>

  <h2>Query</h2>
  <div class="query-action">
    <query-sk id=query @query-change=${_queryChange} @query-change-delayed=${ele._queryChangeDelayed}></query-sk>
    <div id=selections>
      <h3>Selections</h3>
      <query-summary-sk id=summary></query-summary-sk>
      <query-count-sk></query-count-sk>
      <button on-tap="_start" class=action id=start>Run</button>
      <div>
        <spinner-sk id=clusterSpinner></spinner-sk>
        <span id=status></span>
      </div>
    </div>
  </div>

  <details-sk>
    <summary-sk id=advanced>
      <h2>Advanced</h2>
    </summary-sk>
    <div id=inputs>
      <label>
        K (A value of 0 means the server chooses).
        <input type=number min=0 max=100  .value=${ele.state.k}>
      </label>
      <label>
        Number of commits to include on either side.
        <input type=number min=1 max=25   .value=${ele.state.radius}>
      </label>
      <label>
        Clusters are interesting if regression score &gt;= this.
        <input type=number min=0 max=500  .value=${ele.state.interesting}>
      </label>
      <checkbox-sk
        checked=${ele.state.sparse}
        label='Data is sparse, so only include commits that have data.'>
      </checkbox-sk>
    </div>
  </details-sk>

  <h2>Results</h2>
  <sort-sk target=clusters node_name="CLUSTER-SUMMARY2-SK">
    <button data-key="clustersize">Cluster Size </button>
    <button data-key="stepregression" data-default=up>Regression </button>
    <button data-key="stepsize">Step Size </button>
    <button data-key="steplse">Least Squares</button>
  </sort-sk>
  <div id=clusters>
    ${_summaryRows(ele)}
  </div>
`;

window.customElements.define('cluster-page-sk', class extends ElementSk {
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
