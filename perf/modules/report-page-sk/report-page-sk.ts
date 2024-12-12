/**
 * @module modules/report-page-sk
 * @description <h2><code>report-page-sk</code></h2>
 *
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { State, ExploreSimpleSk } from '../explore-simple-sk/explore-simple-sk';
import { Anomaly, QueryConfig, Timerange } from '../json';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { ChromeTraceFormatter } from '../trace-details-formatter/traceformatter';
import { SpinnerSk } from '../../../elements-sk/modules/spinner-sk/spinner-sk';
import { errorMessage } from '../errorMessage';
import { AnomaliesTableSk } from '../anomalies-table-sk/anomalies-table-sk';
import '../../../elements-sk/modules/spinner-sk';
import '../anomalies-table-sk/anomalies-table-sk';

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

  // Anomaly list
  private anomalyList: Anomaly[] = [];

  // Maps anomaly ID to Begin and End ranges.
  private timerangeMap: { [key: number]: Timerange } = {};

  // Keep track of which anomalies are graphed.
  private anomalyMap: { [key: number]: ExploreSimpleSk } = {};

  private graphDiv: Element | null = null;

  private _spinner: SpinnerSk | null = null;

  private traceFormatter: ChromeTraceFormatter | null = null;

  private defaults: QueryConfig | null = null;

  constructor() {
    super(ReportPageSk.template);
    this.traceFormatter = new ChromeTraceFormatter();
  }

  async connectedCallback() {
    super.connectedCallback();
    this._render();

    this._spinner = this.querySelector('#loading-spinner');
    this.anomaliesTable = this.querySelector('#anomaly-table');
    this.graphDiv = this.querySelector('#graph-container');
    await this.initializeDefaults();
    // Parse the URL Params.
    const params = new URLSearchParams(window.location.search);
    this.params.rev = params.get('rev') || '';
    this.params.anomalyIDs = params.get('anomalyIDs') || '';
    this.params.bugID = params.get('bugID') || '';
    this.params.anomalyGroupID = params.get('anomalyGroupID') || '';
    this.params.sid = params.get('sid') || '';
    this.addEventListener('anomalies_checked', (e) => {
      const detail = (e as CustomEvent).detail;
      this.updateGraphs(detail.anomaly, detail.checked);
    });
    await this.fetchAnomalies();
  }

  private static template = () => html`
    <div>
      <spinner-sk id="loading-spinner"></spinner-sk>
    </div>
    <anomalies-table-sk id="anomaly-table"></anomalies-table-sk>
    <div id="graph-container"></div>
  `;

  async fetchAnomalies() {
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
      .then((json) => {
        this.anomalyList = json.anomaly_list || [];
        this.timerangeMap = json.timerange_map;
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
    this.anomaliesTable!.populateTable(this.anomalyList);
    if (this.params.anomalyIDs) {
      const anomaly = this.initialReportPageCheckedAnomalies(
        this.params.anomalyIDs,
        this.anomalyList
      );
      if (anomaly) {
        this.anomaliesTable!.checkAnomaly(anomaly!);
      }
    }
  }

  private async initializeDefaults() {
    await fetch(`/_/defaults/`, {
      method: 'GET',
    })
      .then(jsonOrThrow)
      .then((json) => {
        this.defaults = json;
      });
  }

  private getQueryFromAnomaly(anomaly: Anomaly) {
    return this.traceFormatter!.formatQuery(anomaly.test_path);
  }

  private addGraph(anomaly: Anomaly) {
    const explore: ExploreSimpleSk = new ExploreSimpleSk(true, false);
    explore.defaults = this.defaults;
    explore.openQueryByDefault = false;
    explore.navOpen = false;

    this.graphDiv!.prepend(explore);

    const query = this.getQueryFromAnomaly(anomaly);
    const state = new State();
    const timerange = this.timerangeMap[anomaly.id];
    explore.state = {
      ...state,
      queries: [query],
      highlight_anomalies: [String(anomaly.id)],
      begin: timerange.begin,
      end: timerange.end,
    };
    this._render();

    return explore;
  }

  private updateGraphs(anomaly: Anomaly, checked: boolean) {
    const explore = this.anomalyMap[anomaly.id];
    if (checked && !explore) {
      // Add a new graph if checked and it doesn't exist
      this.anomalyMap[anomaly.id] = this.addGraph(anomaly);
    } else if (!checked && explore) {
      // Remove the graph if unchecked and it exists
      delete this.anomalyMap[anomaly.id];
      this.graphDiv!.removeChild(explore);
    }
  }

  private initialReportPageCheckedAnomalies(
    anomalyIds: string,
    anomalies: Anomaly[]
  ): Anomaly | null {
    const firstAnomalyId = anomalyIds.split(',').at(0);
    for (let i = 0; i < anomalies.length; i++) {
      const anomaly = anomalies.at(i);
      if (anomaly !== undefined && String(anomaly.id) === firstAnomalyId) {
        return anomaly;
      }
    }
    return null;
  }
}

define('report-page-sk', ReportPageSk);
