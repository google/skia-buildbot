/**
 * @module modules/cluster-summary2-sk
 * @description <h2><code>cluster-summary2-sk</code></h2>
 *
 * @evt open-keys - An event that is fired when the user clicks the "View on
 *     dashboard" button that contains the shortcut id, and the timestamp range of
 *     traces in the details that should be opened in the explorer, and the xbar
 *     location specified as a serialized cid.CommitID, for example:
 *
 *     {
 *       shortcut: "X1129832198312",
 *       begin: 1476982874,
 *       end: 1476987166,
 *       xbar: {"source":"master","offset":24750,"timestamp":1476985844},
 *     }
 *
 * @evt triaged - An event generated when the 'Update' button is pressed, which
 *     contains the new triage status. The detail contains the cid and triage
 *     status, for example:
 *
 *     {
 *       cid: {
 *         source: "master",
 *         offset: 25004,
 *       },
 *       triage: {
 *         status: "negative",
 *         messge: "This is a regression in ...",
 *       },
 *     }
 *
 * @attr {Boolean} fade - A boolean, fade out an issue if its status isn't New.
 *
 * @example
 */
import { html, render } from 'lit-html'
import { ElementSk } from '../../infra-sk/ElementSk'

function _trunc(value) {
  return (+value).toPrecision(3);
}

const template = (ele) => html`
  <div class='regression ${ele._statusClass()}'>
    Regression: <span>${_trunc(_summary.step_fit.regression)}</span>
  </div>
  <div class=stats>
    <div class=labelled>Cluster Size: <span>${ele._summary.num}</span></div>
    <div class=labelled>Least Squares Error: <span>${_trunc(ele._summary.step_fit.least_squares)}</span></div>
    <div class=labelled>Step Size: <span>${_trunc(ele._summary.step_fit.step_size)}</span></div>
  </div>
  <div class=plot>
    <plot-simple-sk specialevents @trace_selected=${ele._traceSelected} id=graph width=400 height=150></plot-simple-sk>
    <div id=status class=${ele._hiddenClass()}>
      <p class="disabledMessage">You must be logged in to change the status.</p>
      <triage2-sk value=${ele.triage.status} @change=${ele._triageChange}></triage2-sk>
      <input type=text value=${ele.triage.message} label=Message>
      <button class=action @click=${ele._update}>Update</button>
    </div>
  </div>
  <commit-detail-panel-sk id=commits></commit-detail-panel-sk>
  <div>
    <button id=shortcut @click=${ele._openShortcut}>View on dashboard</button>
    <a id=permalink class=${_hiddenClass()} href=${ele._permaLink()}>Permlink</a>
    <a id=rangelink href='' target=_blank></a>
    <button @click=${ele._toggleWordCloud}>Word Cloud</button>
  </div>
  <collapse-sk id=wordCloudCollapse closed>
    <word-cloud-sk id=wordcloud items=${ele._summary.param_summaries}></word-cloud-sk>
  </collapse-sk>
  `;

window.customElements.define('cluster-summary2-sk', class extends ElementSk {
  constructor() {
    super(template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  disconnectedCallback() {
  }

  _hiddenClass() {
    return this.triage.status === '' ? 'hidden' : '';
  }

  _permaLink() {
    // Bounce to the triage page, but with the time range narrowed to
    // contain just the step_point commit.
    if (!this._summary || !this._summary.step_point) {
      return '';
    }
    const begin = this._summary.step_point.timestamp;
    const end = begin+1;
    return `/t/?begin=${begin}&end=${end}&subset=all`;
  }

  _statusClass() {
    const status = ele_summary.step_fit.status || '';
    return status.toLowerCase();
  }


  /** @prop full_summary {string} A serialized:
   *
   *  {
   *    summary: cluster2.ClusterSummary,
   *    frame: dataframe.FrameResponse,
   *  }
   *
   */
  get full_summary() { return this._full_summary }
  set full_summary(val) {
    this._full_summary = val;
    this._render();
  }

  /** @prop triage {string} The triage status of the cluster.
   *     Something of the form:
   *
   *    {
   *      status: "untriaged",
   *      message: "This is a regression.",
   *    }
   *
   */
  get triage() { return this._triage }
  set triage(val) {
    this._triage = val;
    this._render();
  }
});
