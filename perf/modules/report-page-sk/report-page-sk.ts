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
import { errorMessage } from '../errorMessage';
import { AnomaliesTableSk } from '../anomalies-table-sk/anomalies-table-sk';
import '../../../elements-sk/modules/spinner-sk';
import '../anomalies-table-sk/anomalies-table-sk';
import { lookupCids } from '../cid/cid';
import { upgradeProperty } from '../../../elements-sk/modules/upgradeProperty';
import '../../../elements-sk/modules/icons/camera-roll-icon-sk';

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
  private tracker: { [key: string]: AnomalyDataPoint };

  constructor() {
    this.tracker = {};
  }

  // Load the tracker with the necessary information from the provided anomaly list.
  // It's assumed that there's a 1:1 mapping between the information in anomalyList
  // and timerangeMap, but there's nothing that enforces nor checks it. This means
  // that any missing data will simply be unset.
  load(
    anomalyList: Anomaly[],
    timerangeMap: { [key: string]: Timerange },
    selectedKeys: string[]
  ): void {
    anomalyList.forEach((anomaly) => {
      this.tracker[anomaly.id] = {
        anomaly: anomaly,
        // anomaly id is number type, but selectedKey in string and needs type
        // match to check whether it's in.
        // When selectedKeys is null, includes() returns undefined.
        checked: Boolean(selectedKeys?.includes(anomaly.id)),
        graph: null,
        timerange: timerangeMap[anomaly.id],
      };
    });
  }

  getAnomaly(id: string): AnomalyDataPoint | null {
    return this.tracker[id];
  }

  setGraph(id: string, graph: ExploreSimpleSk): void {
    this.tracker[id].graph = graph;
  }

  unsetGraph(id: string): void {
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

  getTimerangeMap(): { [key: string]: Timerange } {
    const map: { [key: string]: Timerange } = {};
    for (const anomalyId in this.tracker) {
      map[anomalyId] = this.tracker[anomalyId].timerange;
    }
    return map;
  }
}

export class ReportPageSk extends ElementSk {
  // An anomaly tracker for the report page.
  private anomalyTracker = new AnomalyTracker();

  // Reference to anomalies table element.
  private anomaliesTable: AnomaliesTableSk | null = null;

  private graphDiv: Element | null = null;

  private _currentlyLoading: string = '';

  private traceFormatter: ChromeTraceFormatter | null = null;

  private defaults: QueryConfig | null = null;

  private commitMap: Map<Commit, boolean> = new Map();

  private requestAnomalies: string[] = [];

  private commitUrlprefix = window.perf.git_repo_url + '/+show/';

  private commitBodyPrefix = 'Body ';

  private static template = (ele: ReportPageSk) => html`
    ${ele._currentlyLoading
      ? html`
          <div class="loading-status">
            <spinner-sk id="loading-spinner" active></spinner-sk>
            <span class="loading-message">${ele._currentlyLoading}</span>
          </div>
        `
      : ''}
    <anomalies-table-sk id="anomaly-table"></anomalies-table-sk>
    ${ele.showAllCommitsTemplate()}
    <div id="graph-container" @x-axis-toggled=${ele.syncXAxisLabel}></div>
  `;

  constructor() {
    super(ReportPageSk.template);
    this.traceFormatter = new ChromeTraceFormatter();
  }

  private setCurrentlyLoading(value: string) {
    this._currentlyLoading = value;
    this._render();
  }

  async connectedCallback() {
    super.connectedCallback();
    upgradeProperty(this, 'commitList');
    this._render();

    this.anomaliesTable = this.querySelector('#anomaly-table');
    this.graphDiv = this.querySelector('#graph-container');

    this.setCurrentlyLoading('Loading configuration...');
    await this.initializeDefaults();
    this.addEventListener('anomalies_checked', (e) => {
      const detail = (e as CustomEvent).detail;
      this.updateGraphs(detail.anomaly, detail.checked);
    });

    await this.fetchAnomalies();
  }

  async fetchAnomalies() {
    this.setCurrentlyLoading('Loading anomalies...');
    this._render();

    const urlParams = new URLSearchParams(window.location.search);
    this.requestAnomalies =
      urlParams.get('anomalyIDs') === null ? [] : urlParams.get('anomalyIDs')!.split(',');
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
        const selectedKey: string[] = json.selected_keys;
        if (selectedKey && selectedKey.length > 0) {
          this.requestAnomalies.push(...selectedKey);
        }
        this.setCurrentlyLoading('Loading anomalies details and common commits...');
        await Promise.all([
          this.initializePage(),
          this.listAllCommits(this.anomalyTracker.toAnomalyList()),
        ]);
        this.setCurrentlyLoading('');
        this._render();
      })
      .catch((msg: any) => {
        errorMessage(msg);
        this.setCurrentlyLoading('');
        this._render();
      });
  }

  private async initializePage() {
    await this.anomaliesTable!.populateTable(
      this.anomalyTracker.toAnomalyList(),
      this.anomalyTracker.getTimerangeMap()
    );

    this.anomaliesTable!.checkSelectedAnomalies(this.findRequestedAnomalies());
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
    if (anomalies !== undefined && anomalies.length > 0) {
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
      const commitSlice: Commit[] = json.commitSlice!;
      const commitUrlSet = new Set<string>();
      commitSlice.forEach((commit) => {
        if (!commitUrlSet.has(commit.url)) {
          this.checkCommitIsRollout(commit);
        }
        commitUrlSet.add(commit.url);
      });
    }
  }

  private getQueryFromAnomaly(anomaly: Anomaly) {
    return this.traceFormatter!.formatQuery(anomaly.test_path);
  }

  private addGraph(anomaly: Anomaly) {
    const explore: ExploreSimpleSk = new ExploreSimpleSk(true);
    explore.defaults = this.defaults;
    explore.openQueryByDefault = false;
    explore.navOpen = false;
    explore.enableRemoveButton = false;
    explore.is_chart_split = true;
    const graphIndex = this.graphDiv!.children.length;
    this.graphDiv!.prepend(explore);

    const query = this.getQueryFromAnomaly(anomaly);
    const state = new State();
    const timerange = this.anomalyTracker.getAnomaly(anomaly.id)!.timerange;
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
      graph_index: graphIndex,
    };
    this.updateChartHeights();
    this._render();

    return explore;
  }

  private updateGraphs(anomaly: Anomaly, checked: boolean) {
    const graph = this.anomalyTracker.getAnomaly(anomaly.id)!.graph;
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

  private syncXAxisLabel(e: CustomEvent): void {
    const graphs = this.graphDiv!.querySelectorAll('explore-simple-sk');
    graphs.forEach((graph) => {
      (graph as ExploreSimpleSk).switchXAxis(e.detail);
    });
  }

  // findRequestedAnomalies returns a list of requested anomaly objects .
  // This is for only loading selected untriaged anomaly graphs in the first place.
  findRequestedAnomalies(): Anomaly[] {
    const ret: Anomaly[] = [];
    this.anomalyTracker.toAnomalyList().forEach((anomaly) => {
      if (this.requestAnomalies.includes(String(anomaly.id))) {
        ret.push(this.anomalyTracker.getAnomaly(anomaly.id)!.anomaly);
      }
    });
    return ret;
  }

  private showAllCommitsTemplate() {
    if (this.commitMap.size !== 0) {
      return html`
        <div class="common-commits">
          <h3>Common Commits</h3>
          <div class="scroll-commits">
            <ul class="table" id="all-commits-scroll">
              ${Array.from(this.commitMap.keys())
                .slice(0, 10)
                .map((commit) => {
                  return html` <li>
                    <a href="${commit.url}" target="_blank">${commit.hash.substring(0, 7)}</a>
                    <span id="commit-message">${commit.message}</span>
                    ${this.addIconForRollCommit(commit)}
                  </li>`;
                })}
            </ul>
          </div>
        </div>
      `;
    }
  }

  private checkCommitIsRollout(commit: Commit) {
    if (commit.message.startsWith('Roll') || commit.message.startsWith('Manual roll')) {
      this.commitMap.set(commit, true);
    } else {
      this.commitMap.set(commit, false);
    }
  }

  private addIconForRollCommit(commit: Commit) {
    if (this.commitMap.get(commit)) {
      return html`
        <button
          id="roll-commits-link"
          @click=${() => {
            this.openUnderlyingCommitUrl(commit);
          }}>
          <camera-roll-icon-sk></camera-roll-icon-sk>
        </button>
      `;
    } else {
      return ``;
    }
  }

  private async openUnderlyingCommitUrl(commit: Commit) {
    const json = await lookupCids([commit.offset]);
    const logEntry = json.logEntry;
    let url = '';
    if (this.checkIfCommitMessageFollowsRollPattern(commit.message)) {
      url = this.findInternalCommitUrl(logEntry);
    } else {
      url = this.findParentUrl(logEntry);
    }
    if (url !== '') {
      window.open(url, '_blank');
    } else {
      window.open(commit.url, '_blank');
    }
  }

  // Using Regular Expressions to check whether the commit message
  // follows the pattern, e.g: "Roll repo from hash to hash"
  private checkIfCommitMessageFollowsRollPattern(message: string) {
    const regex = /^.+? from .+? to .+? \(.+?\)$/;
    // Execute the regex against the input string
    const match = regex.exec(message);
    return match ? true : false;
  }

  private findInternalCommitUrl(log: string) {
    // If a match is found, match[1] contains the captured group
    // split the log to each line, find the url right after "Body ")
    const bodyLine = log.split('\n').find((line) => line.startsWith(this.commitBodyPrefix));
    return bodyLine ? bodyLine.substring(this.commitBodyPrefix.length) : '';
  }

  // When the initial commit message does not follow "Roll"
  private findParentUrl(log: string) {
    const regex = /^Parent (.*)$/m;
    const match = log.match(regex);
    // If a match is found, match[1] contains the captured group (the text after "Parent ")
    if (match && match[1]) {
      // Return parent url with window perf git url prefix
      // Parent url starts after 'Parent '
      // e,g: 'db2e77d1decc3cae2172a7b72c931aa20b4b1d37'
      return this.commitUrlprefix + match[1];
    }
    return '';
  }
}

define('report-page-sk', ReportPageSk);
