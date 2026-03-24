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
import { html, LitElement } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import '../tricon2-sk';
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

@customElement('triage-status-sk')
export class TriageStatusSk extends LitElement {
  @property({ type: Object })
  triage: TriageStatus = {
    status: 'untriaged',
    message: '(none)',
  };

  @property({ type: Object })
  full_summary: FullSummary | null = null;

  @property({ type: Object })
  alert: Alert | null = null;

  @property({ type: String })
  cluster_type: ClusterType = 'low';

  createRenderRoot() {
    return this;
  }

  render() {
    return html`
      <button title=${this.triage.message} @click=${this._start_triage} class=${this.triage.status}>
        <tricon2-sk class="inside_status" value=${this.triage.status}></tricon2-sk>
      </button>
    `;
  }

  private _start_triage() {
    const detail: TriageStatusSkStartTriageEventDetails = {
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
      })
    );
  }
}
