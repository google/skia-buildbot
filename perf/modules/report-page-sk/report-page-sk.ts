/**
 * @module modules/report-page-sk
 * @description <h2><code>report-page-sk</code></h2>
 *
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { errorMessage } from '../errorMessage';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { SpinnerSk } from '../../../elements-sk/modules/spinner-sk/spinner-sk';
import { AnomaliesTableSk } from '../anomalies-table-sk/anomalies-table-sk';
import '../../../elements-sk/modules/spinner-sk';

class ReportPageParams {
  // A revision number.
  rev: string = '';

  // Comma-separated list of Anomaly keys.
  anomalyIDs: string = '';

  // A Buganizer bug number ID.
  bugID: string = '';

  // An Anomaly Group ID
  anomalyGroupID: string = '';

  // A hash of a group of anomaly keys.
  sid: string = '';
}

export class ReportPageSk extends ElementSk {
  private params: ReportPageParams = new ReportPageParams();

  // Anomalies table
  private anomaliesTable: AnomaliesTableSk | null = null;

  private _spinner: SpinnerSk | null = null;

  constructor() {
    super(ReportPageSk.template);
    this.anomaliesTable = new AnomaliesTableSk();
  }

  async connectedCallback() {
    super.connectedCallback();
    this._render();

    this._spinner = this.querySelector('#loading-spinner');

    // Parse the URL Params.
    const params = new URLSearchParams(window.location.search);
    this.params.rev = params.get('rev') || '';
    this.params.anomalyIDs = params.get('anomalyIDs') || '';
    this.params.bugID = params.get('bugID') || '';
    this.params.anomalyGroupID = params.get('anomalyGroupID') || '';
    this.params.sid = params.get('sid') || '';

    await this.fetchAnomalies();
  }

  private static template = () => html`
    <div>
      <spinner-sk id="loading-spinner"></spinner-sk>
    </div>
    <anomalies-table-sk id="anomaly-table"></anomalies-table-sk>
  `;

  private async fetchAnomalies() {
    this._spinner!.active = true;
    this._render();

    await fetch('/_/anomalies/group_report', {
      method: 'POST',
      body: JSON.stringify(this.params),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((_) => {
        this.initializePage();
        this._spinner!.active = false;
        this._render();
      })
      .catch((msg: any) => {
        errorMessage(msg);
        this._spinner!.active = false;
        this._render();
      });
  }

  private initializePage() {
    // TODO(eduardoyap): Initialize table and graphs.
  }
}

define('report-page-sk', ReportPageSk);
