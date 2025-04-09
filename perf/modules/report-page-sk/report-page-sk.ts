/**
 * @module modules/report-page-sk
 * @description <h2><code>report-page-sk</code></h2>
 *
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { State, ExploreSimpleSk } from '../explore-simple-sk/explore-simple-sk';
import { Anomaly, Commit, CommitNumber, QueryConfig, Timerange } from '../json';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { ChromeTraceFormatter } from '../trace-details-formatter/traceformatter';
import { SpinnerSk } from '../../../elements-sk/modules/spinner-sk/spinner-sk';
import { errorMessage } from '../errorMessage';
import { AnomaliesTableSk } from '../anomalies-table-sk/anomalies-table-sk';
import '../../../elements-sk/modules/spinner-sk';
import '../anomalies-table-sk/anomalies-table-sk';
import { lookupCids } from '../cid/cid';
import { upgradeProperty } from '../../../elements-sk/modules/upgradeProperty';

const weekInSeconds = 7 * 24 * 60 * 60;

// Data point for anomalies tracking the actual anomaly object.
// Inclusive of:
// * whether it's been checked.
// * the graph generated for it.
// * beginning and end time ranges.
export interface AnomalyDataPoint {
  // The anomaly object that this tracker is maintaining
  anomaly: Anomaly;
  // Boolean field to track whether the given anomaly has been selected
  checked: boolean;
  // The anomaly group this anomaly is associated with
  // group: AnomalyGroup;
  // The ExploreSimpleSk object this Anomaly has been graphed for
  graph: ExploreSimpleSk | null;
  // Begin and end time ranges for the current Anomaly.
  timerange: Timerange;
}

export class AnomalyTracker {
  // Internal map for anomalies
  private tracker: { [key: number]: AnomalyDataPoint };

  constructor() {
    this.tracker = {};
  }

  // Load the tracker with the necessary information from the provided anomaly list.
  // It's assumed that there's a 1:1 mapping between the information in anomalyList
  // and timerangeMap, but there's nothing that enforces nor checks it. This means
  // that any missing data will simply be unset.
  load(
    anomalyList: Anomaly[],
    timerangeMap: { [key: number]: Timerange },
    selectedKeys: string[] = []
  ): void {
    anomalyList.forEach((anomaly) => {
      this.tracker[anomaly.id] = {
        anomaly: anomaly,
        // anomaly id is number type, but selectedKey in string and needs type
        // match to check whether it's in.
        // When selectedKeys is null, includes() returns undefined.
        checked: Boolean(selectedKeys?.includes(anomaly.id.toString())),
        graph: null,
        timerange: timerangeMap[anomaly.id],
      };
    });
  }

  getAnomaly(id: number): AnomalyDataPoint {
    return this.tracker[id];
  }

  setGraph(id: number, graph: ExploreSimpleSk): void {
    this.tracker[id].graph = graph;
  }

  unsetGraph(id: number): void {
    this.tracker[id].graph = null;
  }

  // toAnomalyList returns a list of all anomaly objects being tracked.
  // This is mostly for backwards compatibility to anomalies-table-sk.
  toAnomalyList(): Anomaly[] {
    const ret = [];
    for (const anomalyId in this.tracker) {
      ret.push(this.tracker[anomalyId].anomaly);
    }
    return ret;
  }

  getSelectedAnomalies(): Anomaly[] {
    const ret = [];
    for (const anomalyId in this.tracker) {
      if (this.tracker[anomalyId].checked) {
        ret.push(this.tracker[anomalyId].anomaly);
      }
    }

    return ret;
  }
}

export class ReportPageSk extends ElementSk {
  // An anomaly tracker for the report page.
  private anomalyTracker = new AnomalyTracker();

  // Reference to anomalies table element.
  private anomaliesTable: AnomaliesTableSk | null = null;

  private graphDiv: Element | null = null;

  private _spinner: SpinnerSk | null = null;

  private traceFormatter: ChromeTraceFormatter | null = null;

  private defaults: QueryConfig | null = null;

  private commitList: Commit[] = [];

  private allCommitsDialog: HTMLDialogElement | null = null;

  constructor() {
    super(ReportPageSk.template);
    this.traceFormatter = new ChromeTraceFormatter();
  }

  async connectedCallback() {
    super.connectedCallback();
    upgradeProperty(this, 'commitList');
    this._render();

    this._spinner = this.querySelector('#loading-spinner');
    this.anomaliesTable = this.querySelector('#anomaly-table');
    this.graphDiv = this.querySelector('#graph-container');
    await this.initializeDefaults();

    this.addEventListener('anomalies_checked', (e) => {
      const detail = (e as CustomEvent).detail;
      this.updateGraphs(detail.anomaly, detail.checked);
    });
    await this.fetchAnomalies();
  }

  private static template = (ele: ReportPageSk) => html`
    <div>
      <spinner-sk id="loading-spinner"></spinner-sk>
    </div>
    <anomalies-table-sk id="anomaly-table"></anomalies-table-sk>
    ${ele.showAllCommitsTemplate()}
    <div id="graph-container"></div>
  `;

  async fetchAnomalies() {
    this._spinner!.active = true;
    this._render();

    const urlParams = new URLSearchParams(window.location.search);
    await fetch('/_/anomalies/group_report', {
      method: 'POST',
      body: JSON.stringify({
        rev: urlParams.get('rev') || '', // A revision number.
        anomalyIDs: urlParams.get('anomalyIDs') || '', // Comma delimited.
        bugID: urlParams.get('bugID') || '',
        anomalyGroupID: urlParams.get('anomalyGroupID') || '',
        sid: urlParams.get('sid') || '', // A hash of a group of anomaly keys.
      }),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then(async (json) => {
        this.anomalyTracker.load(json.anomaly_list, json.timerange_map, json.selected_keys);
        this.initializePage();
        await this.listAllCommits(this.anomalyTracker.toAnomalyList());
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
    this.anomaliesTable!.populateTable(this.anomalyTracker.toAnomalyList());

    const selected = this.anomalyTracker.getSelectedAnomalies();
    if (selected.length > 0) {
      this.anomaliesTable!.checkSelectedAnomalies(selected);
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

  private async listAllCommits(anomalies: Anomaly[] | undefined) {
    if (anomalies !== undefined) {
      const commits: CommitNumber[] = [];
      let start = anomalies.at(0)!.start_revision;
      let end = anomalies.at(0)!.end_revision;
      anomalies.forEach((anomaly) => {
        if (anomaly.start_revision > start && anomaly.start_revision < end) {
          start = anomaly.start_revision;
        }
        if (anomaly.end_revision < end && anomaly.end_revision > start) {
          end = anomaly.end_revision;
        }
      });

      for (let c = end; c >= start; c--) {
        commits.push(c as CommitNumber);
      }

      const json = await lookupCids(commits);
      const response: Commit[] = json.commitSlice!;
      const commitUrlSet = new Set<string>();
      response.forEach((commit) => {
        if (!commitUrlSet.has(commit.url)) {
          this.commitList!.push(commit);
        }
        commitUrlSet.add(commit.url);
      });
    }
  }

  private getQueryFromAnomaly(anomaly: Anomaly) {
    return this.traceFormatter!.formatQuery(anomaly.test_path);
  }

  private addGraph(anomaly: Anomaly) {
    const explore: ExploreSimpleSk = new ExploreSimpleSk(false);
    explore.defaults = this.defaults;
    explore.openQueryByDefault = false;
    explore.navOpen = false;

    this.graphDiv!.prepend(explore);

    const query = this.getQueryFromAnomaly(anomaly);
    const state = new State();
    const timerange = this.anomalyTracker.getAnomaly(anomaly.id).timerange;
    explore.state = {
      ...state,
      queries: [query],
      highlight_anomalies: [String(anomaly.id)],
      // show 1 week's worth of data before and after
      // showing more data helps users determine
      // if a regression has already been mitigated
      begin: timerange.begin - weekInSeconds,
      end: timerange.end + weekInSeconds,
      // the requestType controls how many data points to query.
      // 0 means to query data points between the begin and end timestamps
      // 1 means to query State().numCommits number of data points
      // Set to 0 to promote symmetry.
      requestType: 0,
    };
    this.updateChartHeights();
    this._render();

    return explore;
  }

  private updateGraphs(anomaly: Anomaly, checked: boolean) {
    const graph = this.anomalyTracker.getAnomaly(anomaly.id).graph;
    if (checked && !graph) {
      // Add a new graph if checked and it doesn't exist
      this.anomalyTracker.setGraph(anomaly.id, this.addGraph(anomaly));
    } else if (!checked && graph) {
      // Remove the graph if unchecked and it exists
      this.anomalyTracker.unsetGraph(anomaly.id);
      this.graphDiv!.removeChild(graph);
    }
    this.updateChartHeights();
  }

  private updateChartHeights(): void {
    const graphs = this.graphDiv!.querySelectorAll('explore-simple-sk');
    graphs.forEach((graph) => {
      const height = graphs.length === 1 ? '500px' : '250px';
      (graph as ExploreSimpleSk).updateChartHeight(height);
    });
  }

  private showAllCommitsTemplate() {
    if (this.commitList.length !== 0) {
      return html`
        <div class="common-commits">
          <h3>Common Commits</h3>
          <div class="scroll-commits">
            <ul class="table" id="all-commits-scroll">
              ${Array.from(this.commitList)
                .slice(0, 10)
                .map((commit) => {
                  return html` <li>
                    <a href="${commit.url}" target="_blank">${commit.hash.substring(0, 7)}</a>
                    <span id="commit-message">${commit.message}</span>
                  </li>`;
                })}
            </ul>
          </div>
        </div>
      `;
    }
  }
}

define('report-page-sk', ReportPageSk);
