/**
 * @module modules/triage-status-sk
 * @description <h2><code>triage-status-sk</code></h2>
 *
 * Displays a button that shows the triage status of a cluster.  When the
 * button is pushed a dialog opens that allows the user to see the cluster
 * details and to change the triage status.
 *
 * @evt start-triage - Contains the new triage status. The detail contains the
 *    alert, cluster_type, full_summary, and triage.
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import '../tricon2-sk';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

const template = (ele) => html`
  <button title=${ele.triage.message} @click=${ele._start_triage}>
    <tricon2-sk value=${ele.triage.status}></tricon2-sk>
  </button>
`;

define('triage-status-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._triage = {
      status: 'untriaged',
      message: '(none)',
    };
    this._full_summary = {};
    this._alert = {};
    this._cluster_type = 'low';
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this._upgradeProperty('alert');
    this._upgradeProperty('cluster_type');
    this._upgradeProperty('full_summary');
    this._upgradeProperty('triage');
  }

  _start_triage() {
    const detail = {
      full_summary: this.full_summary,
      triage: this.triage,
      alert: this.alert,
      cluster_type: this.cluster_type,
      element: this,
    };
    this.dispatchEvent(new CustomEvent('start-triage', { detail: detail, bubbles: true }));
  }

  /** @prop alert {alerts.Config} The config this cluster is associated with. */
  get alert() { return this._alert; }

  set alert(val) { this._alert = val; }

  /** @prop cluster_type {string} The type of cluster, either "high" or "low". */
  get cluster_type() { return this._cluster_type; }

  set cluster_type(val) { this._cluster_type = val; }

  /** @prop full_summary {string} A serialized
   *
   *    {
   *      summary: cluster2.ClusterSummary,
   *      frame: dataframe.FrameResponse,
   *    }
   */
  get full_summary() { return this._full_summary; }

  set full_summary(val) { this._full_summary = val; }

  /** @prop triage {Object} The triage status of the cluster. Something of
   * the form:
   *
   *   {
   *     status: "untriaged",
   *     message: "This is a regression.",
   *   }
   *
   */
  get triage() { return this._triage; }

  set triage(val) {
    this._triage = val;
    this._render();
  }
});
