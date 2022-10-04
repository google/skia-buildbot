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
import { FullSummary, TriageStatus, Alert } from '../json';

export type ClusterType = 'high' | 'low';

export interface TriageStatusSkStartTriageEventDetails {
  triage: TriageStatus;
  full_summary: FullSummary | null;
  alert: Alert | null;
  cluster_type: ClusterType;
  // eslint-disable-next-line no-use-before-define
  element: TriageStatusSk;
}

export class TriageStatusSk extends ElementSk {
  private _triage: TriageStatus;

  private _full_summary: FullSummary | null;

  private _alert: Alert | null;

  private _cluster_type: ClusterType;

  constructor() {
    super(TriageStatusSk.template);
    this._triage = {
      status: 'untriaged',
      message: '(none)',
    };
    this._full_summary = null;
    this._alert = null;
    this._cluster_type = 'low';
  }

  private static template = (ele: TriageStatusSk) => html`
  <button
    title=${ele.triage.message}
    @click=${ele._start_triage}
    class=${ele.triage.status}
  >
    <tricon2-sk class="inside_status" value=${ele.triage.status}></tricon2-sk>
  </button>
`;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this._upgradeProperty('alert');
    this._upgradeProperty('cluster_type');
    this._upgradeProperty('full_summary');
    this._upgradeProperty('triage');
  }

  private _start_triage() {
    const detail = {
      full_summary: this.full_summary,
      triage: this.triage,
      alert: this.alert,
      cluster_type: this.cluster_type,
      element: this,
    };
    this.dispatchEvent(
      new CustomEvent<TriageStatusSkStartTriageEventDetails>('start-triage', {
        detail,
        bubbles: true,
      }),
    );
  }

  /** The config this cluster is associated with. */
  get alert(): Alert | null {
    return this._alert;
  }

  set alert(val: Alert | null) {
    this._alert = val;
  }

  /** The type of cluster. */
  get cluster_type(): ClusterType {
    return this._cluster_type;
  }

  set cluster_type(val: ClusterType) {
    this._cluster_type = val;
  }

  /** A serialized ClusterSummary and FrameResponse. */
  get full_summary(): FullSummary | null {
    return this._full_summary;
  }

  set full_summary(val: FullSummary | null) {
    this._full_summary = val;
  }

  /** The triage status of the cluster. */
  get triage(): TriageStatus {
    return this._triage;
  }

  set triage(val: TriageStatus) {
    this._triage = val;
    this._render();
  }
}

define('triage-status-sk', TriageStatusSk);
