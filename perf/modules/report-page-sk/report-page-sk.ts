/**
 * @module modules/report-page-sk
 * @description <h2><code>report-page-sk</code></h2>
 *
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { State, ExploreSimpleSk } from '../explore-simple-sk/explore-simple-sk';
import '../explore-multi-v2-sk/explore-multi-v2-sk';
import { Anomaly, Commit, CommitNumber, QueryConfig, Timerange } from '../json';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { ChromeTraceFormatter } from '../trace-details-formatter/traceformatter';
import { errorMessage } from '../errorMessage';
import { AnomaliesTableSk } from '../anomalies-table-sk/anomalies-table-sk';
import '../../../elements-sk/modules/spinner-sk';
import '../anomalies-table-sk/anomalies-table-sk';
import '../graph-list-sk/graph-list-sk';
import { GraphListSk } from '../graph-list-sk/graph-list-sk';
import { lookupCids } from '../cid/cid';
import { upgradeProperty } from '../../../elements-sk/modules/upgradeProperty';
import '../../../elements-sk/modules/icons/camera-roll-icon-sk';
import '../../../elements-sk/modules/icons/help-icon-sk';
import { CountMetric, SummaryMetric, telemetry } from '../telemetry/telemetry';
import { TrimHash } from '../common/commit';
import { UNSET_TIME } from '../const/const';

const weekInSeconds = 7 * 24 * 60 * 60;

// Data point for anomalies tracking the actual anomaly object.
// Inclusive of:
// * whether it's been checked.
// * the graph generated for it.
// * beginning and end time ranges.
interface AnomalyDataPoint {
  // The anomaly object that this tracker is maintaining
  anomaly: Anomaly;
  // Boolean field to track whether the given anomaly has been selected
  checked: boolean;
  // Begin and end time ranges for the current Anomaly.
  timerange: Timerange;
}

class AnomalyTracker {
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
        // When selectedKeys is null, includes() returns undefined.
        checked: Boolean(selectedKeys?.includes(anomaly.id)),
        timerange: timerangeMap[anomaly.id],
      };
    });
  }

  getAnomaly(id: string): AnomalyDataPoint | null {
    return this.tracker[id];
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
  /**
   * Factory for creating ExploreSimpleSk instances. This allows for dependency
   * injection in tests.
   */
  public exploreSimpleSkFactory = () => new ExploreSimpleSk(false);

  // An anomaly tracker for the report page.
  private anomalyTracker = new AnomalyTracker();

  // Reference to anomalies table element.
  anomaliesTable: AnomaliesTableSk | null = null;

  private graphList: GraphListSk | null = null;

  private _currentlyLoading: string = 'Loading configuration...';

  private _initialized = false;

  private _allGraphsLoaded: boolean = false;

  private pageLoadStart: number = 0;

  private traceFormatter: ChromeTraceFormatter | null = null;

  private defaults: QueryConfig | null = null;

  private commitMap: Map<Commit, boolean> = new Map();

  private requestAnomalies: string[] = [];

  private commitUrlprefix = window.perf.git_repo_url + '/+show/';

  private commitBodyPrefix = 'Body ';

  private _queriesV2: Record<string, string[]>[] = [{}];

  private _viewportMinXV2: number | null = null;

  private _viewportMaxXV2: number | null = null;

  private _splitKeysV2: Set<string> = new Set();

  private _beginV2: number = UNSET_TIME;

  private _endV2: number = UNSET_TIME;

  private _highlightAnomaliesV2: string[] = [];

  private _onlyRegressionsV2 = true;

  private _splitAllV2 = true;

  private _showSparklinesV2 = true;

  private _pageSizeV2 = 250;

  private get isV2Enabled(): boolean {
    const urlParams = new URLSearchParams(window.location.search);
    // URL parameters take precedence to allow forcing a specific version (V1 vs V2) via links.
    if (urlParams.has('v2')) {
      return urlParams.get('v2') === 'true';
    }
    if (urlParams.has('explore')) {
      return urlParams.get('explore') === 'v2';
    }

    // Fall back to the user's local preference, and then the instance default.
    const localPref = localStorage.getItem('perf:use-explore-v2');
    return localPref !== null ? localPref === 'true' : !!window.perf.default_to_explore_v2;
  }

  private static template = (ele: ReportPageSk) => html`
    <div class="title-container">
      <div class="title-left">
        <h1>Anomaly Report</h1>
        <a
          href="https://skia.googlesource.com/buildbot/+/refs/heads/main/perf/report-page-guide.md"
          target="_blank"
          rel="noopener"
          title="Report Page Guide">
          <help-icon-sk></help-icon-sk>
        </a>
      </div>
    </div>
    ${ele._currentlyLoading
      ? html`
          <div class="loading-status">
            <spinner-sk id="loading-spinner" active></spinner-sk>
            <span class="loading-message">${ele._currentlyLoading}</span>
          </div>
        `
      : ''}
    <anomalies-table-sk
      id="anomaly-table"
      show-requested-groups-first
      .loading=${!!ele._currentlyLoading}>
    </anomalies-table-sk>
    ${ele.showAllCommitsTemplate()}
    ${ele._currentlyLoading
      ? ''
      : html`
          <div
            class="v2-toggle-container"
            style="display: flex; align-items: center; justify-content: flex-end; padding: 8px 16px; gap: 12px; border-radius: 8px; margin: 12px 0; border: 1px solid var(--outline, rgba(255,255,255,0.1)); background-color: rgba(128,128,128,0.05);">
            <span style="font-size: 12px; font-weight: 600; color: var(--on-background, #cbd5e1);"
              >Multi-Trace Anomaly Dashboard V2 (Prototype):</span
            >
            <button
              @click=${ele.toggleV2Mode}
              style="background: ${ele.isV2Enabled
                ? 'var(--primary, #1a73e8)'
                : '#475569'}; color: white; border: none; padding: 4px 16px; border-radius: 12px; font-size: 11px; font-weight: bold; cursor: pointer; transition: all 0.2s;">
              ${ele.isV2Enabled ? 'ACTIVE (Click to Disable)' : 'OPT-IN (Click to Enable)'}
            </button>
          </div>
        `}
    ${ele.isV2Enabled
      ? ele._currentlyLoading
        ? ''
        : html`
            <explore-multi-v2-sk
              id="multi-explore"
              .queries=${ele._queriesV2}
              .splitKeys=${ele._splitKeysV2}
              .dateMode=${false}
              .viewportMinX=${ele._viewportMinXV2}
              .viewportMaxX=${ele._viewportMaxXV2}
              .begin=${ele._beginV2}
              .end=${ele._endV2}
              .highlightAnomalies=${ele._highlightAnomaliesV2}
              .onlyRegressions=${ele._onlyRegressionsV2}
              .splitAll=${ele._splitAllV2}
              .showSparklines=${ele._showSparklinesV2}
              .pageSize=${ele._pageSizeV2}
              @explore-state-change=${() => ele._syncFromMultiExplore()}
              .embedded=${true}>
            </explore-multi-v2-sk>
          `
      : html`<graph-list-sk id="graph-list"></graph-list-sk>`}
    <div id="bottom-spacer"></div>
  `;

  constructor() {
    super(ReportPageSk.template);
    this.traceFormatter = new ChromeTraceFormatter();
  }

  private setCurrentlyLoading(value: string) {
    this._currentlyLoading = value;
    this._render();
  }

  private stopTimerAndRecord(metric: SummaryMetric, tags: { [key: string]: string } = {}) {
    if (this.pageLoadStart) {
      const duration = (performance.now() - this.pageLoadStart) / 1000;
      telemetry.recordSummary(metric, duration, {
        ...tags,
        url: window.location.href,
      });
      telemetry.increaseCounter(CountMetric.ReportPageVisit);
      this.pageLoadStart = 0;
    }
  }

  async connectedCallback() {
    this.pageLoadStart = performance.now();
    super.connectedCallback();
    this.updatePageTitle();
    if (this._initialized || this._allGraphsLoaded) {
      return;
    }
    this._initialized = true;
    this._connected = true;
    upgradeProperty(this, 'commitList');
    this._render();

    this.anomaliesTable = this.querySelector('#anomaly-table');
    this.graphList = this.querySelector('#graph-list');
    if (this.graphList) {
      this.graphList.addEventListener('graphs-loaded', () => {
        this._allGraphsLoaded = true;
      });
    }

    this.setCurrentlyLoading('Loading configuration...');
    await this.initializeDefaults();

    this.setCurrentlyLoading('Loading anomalies...');
    await this.fetchAnomalies();

    this.addEventListener('anomalies_checked', (e) => {
      const detail = (e as CustomEvent).detail;
      const anomalies = detail.anomalies as Anomaly[];
      anomalies.forEach((anomaly) => {
        this.anomalyTracker.getAnomaly(anomaly.id)!.checked = detail.checked;
      });
      if (this.isV2Enabled) {
        this.updateMultiExploreStateV2(anomalies, detail.checked);
      } else {
        anomalies.forEach((anomaly) => {
          this.updateGraphs(anomaly, detail.checked);
        });
      }
    });
    this.stopTimerAndRecord(SummaryMetric.ReportPageLoadTime);
  }

  async fetchAnomalies() {
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
        this.updatePageTitle();
        const selectedKey: string[] = json.selected_keys;
        if (selectedKey && selectedKey.length > 0) {
          this.requestAnomalies.push(...selectedKey);
        }
        this.setCurrentlyLoading('Loading anomalies details and common commits...');
        const loadingPromises = [this.initializePage()];

        // Only attempt to fetch and list common commits if the instance is configured
        // for standard integer commit numbers.
        if (json.is_commit_number_based) {
          loadingPromises.push(this.listAllCommits(this.anomalyTracker.toAnomalyList()));
        }

        await Promise.all(loadingPromises);

        if (this.isV2Enabled) {
          this.updateMultiExploreStateV2();
        } else if (this.graphList) {
          this.graphList.items = this.anomalyTracker
            .getSelectedAnomalies()
            .map((anomaly, index) => ({
              id: anomaly.id,
              generateGraph: () => this.addGraph(anomaly, index),
            }));
        }
        this.setCurrentlyLoading('');
      })
      .catch((msg: any) => {
        errorMessage(msg);
        this.setCurrentlyLoading('');
        this._render();
      });
  }

  private async initializePage() {
    await this.anomaliesTable!.populateTable(this.anomalyTracker.toAnomalyList());

    const urlParams = new URLSearchParams(window.location.search);
    // This statement is for when anomalyIDs is set, e.g. anomalyIDs=123,124.
    // Only those anomalies are selected for initial graphing.
    const selected = this.findRequestedAnomalies();
    if (selected.length > 0) {
      this.anomaliesTable!.checkSelectedAnomalies(selected);
      selected.forEach((anomaly) => {
        this.anomalyTracker.getAnomaly(anomaly.id)!.checked = true;
      });
    } else if (urlParams.has('sid')) {
      telemetry.increaseCounter(CountMetric.SIDRequiringActionTaken, {
        module: 'report-page-sk',
        function: 'initializePage',
      });
      // If 'sid' is requested, user has explicitly specified a set of anomalies from the
      // 'Graph Selected' button. It's okay to assume user wants all of them graphed.
      this.anomaliesTable!.initialCheckAllCheckbox();
      this.anomalyTracker.toAnomalyList().forEach((anomaly) => {
        this.anomalyTracker.getAnomaly(anomaly.id)!.checked = true;
      });
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
    if (anomalies !== undefined && anomalies.length > 0) {
      const commits: CommitNumber[] = [];
      let start = anomalies[0]!.start_revision;
      let end = anomalies[0]!.end_revision;
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

  private updatePageTitle(): void {
    const urlParams = new URLSearchParams(window.location.search);
    const bugID = urlParams.get('bugID');
    const anomalyIDs = urlParams.get('anomalyIDs');
    const anomalyGroupID = urlParams.get('anomalyGroupID');
    const sid = urlParams.get('sid');

    let title = 'Report';
    if (bugID) {
      title = `Report for bug: ${bugID}`;
    } else if (anomalyIDs) {
      title = `Report for anomalies: ${anomalyIDs}`;
    } else if (anomalyGroupID) {
      title = `Report for anomaly group: ${anomalyGroupID}`;
    } else if (sid) {
      title = `Report for selected: ${sid}`;
    }

    document.title = title;
  }

  public toggleV2Mode = () => {
    const urlParams = new URLSearchParams(window.location.search);
    const newUrl = new URL(window.location.href);
    if (this.isV2Enabled) {
      // Opting out of V2: keep only standard report-page parameters
      localStorage.setItem('perf:use-explore-v2', 'false');
      const reportParams = ['sid', 'bugID', 'anomalyIDs', 'anomalyGroupID', 'rev'];
      const newParams = new URLSearchParams();
      reportParams.forEach((param) => {
        if (urlParams.has(param)) {
          newParams.set(param, urlParams.get(param)!);
        }
      });
      if (window.perf.default_to_explore_v2) {
        newParams.set('v2', 'false');
      }
      newUrl.search = newParams.toString();
    } else {
      // Opting in to V2: keep existing params and set v2=true
      localStorage.setItem('perf:use-explore-v2', 'true');
      urlParams.set('v2', 'true');
      newUrl.search = urlParams.toString();
    }
    this.redirect(newUrl.toString());
  };

  // Visible for testing
  public redirect(url: string) {
    window.location.href = url;
  }

  private getQueryFromAnomaly(anomaly: Anomaly) {
    return this.traceFormatter!.formatQuery(anomaly.test_path);
  }

  private addGraph(anomaly: Anomaly, index = -1) {
    const explore: ExploreSimpleSk = this.exploreSimpleSkFactory();
    explore.defaults = this.defaults;
    explore.openQueryByDefault = false;
    explore.navOpen = false;
    explore.enableRemoveButton = false;
    explore.is_chart_split = true;
    explore.state.plotSummary = true;
    explore.tracesRendered = true;
    explore.isReportPage = true;

    const query = this.getQueryFromAnomaly(anomaly);
    const state = new State();
    state.plotSummary = true;
    const timerange = this.anomalyTracker.getAnomaly(anomaly.id)!.timerange;
    explore.state = {
      ...state,
      queries: [query],
      highlight_anomalies: [anomaly.id],
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
      graph_index: index > -1 ? index : this.graphList ? this.graphList.items.length : 0,
    };

    return explore;
  }

  private updateGraphs(anomaly: Anomaly, checked: boolean) {
    if (!this._allGraphsLoaded || !this.graphList) {
      return;
    }

    if (checked) {
      this.graphList.addGraph({
        id: anomaly.id,
        generateGraph: () => this.addGraph(anomaly),
      });
    } else {
      this.graphList.removeGraph(anomaly.id);
    }
  }

  private parseAnomalyPathToParamSet(path: string): Record<string, string[]> {
    return this.traceFormatter!.formatParamSet(path);
  }

  private applyDefaultsToParamSet(query: Record<string, string[]>): Record<string, string[]> {
    const defaultParams = this.defaults?.default_param_selections;
    if (!defaultParams) {
      return query;
    }

    const result = { ...query };
    for (const [key, val] of Object.entries(defaultParams)) {
      if (window.perf?.remove_default_stat_value && key === 'stat') {
        continue;
      }
      if (!Object.keys(result).includes(key) && val !== null) {
        result[key] = [...val];
      }
    }
    return result;
  }

  private getSortedQueryString(query: Record<string, string[]>): string {
    const sortedQuery: Record<string, string[]> = {};
    Object.keys(query)
      .sort()
      .forEach((key) => {
        sortedQuery[key] = [...query[key]].sort();
      });
    return JSON.stringify(sortedQuery);
  }

  private isEmptyWithDefaults(
    query: Record<string, string[]>,
    defaultQuery?: Record<string, string[]>
  ): boolean {
    const defaults = defaultQuery || this.applyDefaultsToParamSet({});
    return this.getSortedQueryString(query) === this.getSortedQueryString(defaults);
  }

  private getCommonPathGroupKey(testPath: string): string {
    if (testPath.includes('/')) {
      const trimmed = testPath.replace(/\/+$/, '');
      const lastSlash = trimmed.lastIndexOf('/');
      return lastSlash !== -1 ? trimmed.substring(0, lastSlash) : trimmed;
    }
    if (testPath.startsWith(',') || testPath.includes('=')) {
      const parts = testPath.split(',').filter(Boolean);
      const prefixParts = parts.filter(
        (p) => !p.startsWith('test=') && !p.startsWith('subtest_') && !p.startsWith('stat=')
      );
      if (prefixParts.length > 0) {
        return ',' + prefixParts.join(',') + ',';
      }
      if (parts.length > 1) {
        return ',' + parts.slice(0, -1).join(',') + ',';
      }
    }
    return testPath;
  }

  private mergeAnomalyQueries(anomalies: Anomaly[]): Record<string, string[]> {
    const merged: Record<string, string[]> = {};
    anomalies.forEach((anomaly) => {
      const parsed = this.applyDefaultsToParamSet(
        this.parseAnomalyPathToParamSet(anomaly.test_path)
      );
      for (const [key, values] of Object.entries(parsed)) {
        if (!merged[key]) {
          merged[key] = [];
        }
        for (const val of values) {
          if (!merged[key].includes(val)) {
            merged[key].push(val);
          }
        }
      }
    });
    return merged;
  }

  private updateMultiExploreStateV2(_toggledAnomalies?: Anomaly[], _checked?: boolean): void {
    if (!this.isV2Enabled) return;

    const selectedAnomalies = this.anomalyTracker.getSelectedAnomalies();
    this._highlightAnomaliesV2 = selectedAnomalies.map((a) => a.id);
    if (selectedAnomalies.length === 0) {
      this._queriesV2 = [this.applyDefaultsToParamSet({})];
      this._splitKeysV2 = new Set();
      this._viewportMinXV2 = null;
      this._viewportMaxXV2 = null;
      this._beginV2 = UNSET_TIME;
      this._endV2 = UNSET_TIME;
      this._render();
      return;
    }

    const multiExplore = this.querySelector<any>('#multi-explore');

    // Group selected anomalies by common path
    const groupsMap = new Map<string, Anomaly[]>();
    selectedAnomalies.forEach((anomaly) => {
      const groupKey = this.getCommonPathGroupKey(anomaly.test_path);
      if (!groupsMap.has(groupKey)) {
        groupsMap.set(groupKey, []);
      }
      groupsMap.get(groupKey)!.push(anomaly);
    });

    const mergedQueries: Record<string, string[]>[] = [];
    groupsMap.forEach((anomaliesInGroup) => {
      mergedQueries.push(this.mergeAnomalyQueries(anomaliesInGroup));
    });

    this._queriesV2 = mergedQueries.length > 0 ? mergedQueries : [this.applyDefaultsToParamSet({})];

    // --- Update _splitKeysV2, _viewportMinXV2, _viewportMaxXV2, _beginV2, _endV2 ---
    const shouldPreserveMultiExploreState =
      multiExplore && multiExplore.splitKeys && multiExplore.viewportMinX !== null;

    if (shouldPreserveMultiExploreState) {
      this._splitKeysV2 = multiExplore.splitKeys;
      this._viewportMinXV2 = multiExplore.viewportMinX;
      this._viewportMaxXV2 = multiExplore.viewportMaxX;
      this._beginV2 = multiExplore.begin;
      this._endV2 = multiExplore.end;
      if (multiExplore.onlyRegressions !== undefined)
        this._onlyRegressionsV2 = multiExplore.onlyRegressions;
      if (multiExplore.splitAll !== undefined) this._splitAllV2 = multiExplore.splitAll;
      if (multiExplore.showSparklines !== undefined)
        this._showSparklinesV2 = multiExplore.showSparklines;
      if (multiExplore.pageSize !== undefined) this._pageSizeV2 = multiExplore.pageSize;
    } else {
      let minCommit = Infinity;
      let maxCommit = -Infinity;
      let minBegin = Infinity;
      let maxEnd = -Infinity;

      selectedAnomalies.forEach((anomaly) => {
        if (anomaly.start_revision < minCommit) minCommit = anomaly.start_revision;
        if (anomaly.end_revision > maxCommit) maxCommit = anomaly.end_revision;

        const timerange = this.anomalyTracker.getAnomaly(anomaly.id)!.timerange;
        if (timerange) {
          if (timerange.begin < minBegin) minBegin = timerange.begin;
          if (timerange.end > maxEnd) maxEnd = timerange.end;
        }
      });

      this._splitKeysV2 = new Set();
      this._viewportMinXV2 = Math.max(0, minCommit - 100);
      this._viewportMaxXV2 = maxCommit + 100;

      if (minBegin !== Infinity) {
        this._beginV2 = minBegin - weekInSeconds;
      } else {
        this._beginV2 = UNSET_TIME;
      }
      if (maxEnd !== -Infinity) {
        this._endV2 = maxEnd + weekInSeconds;
      } else {
        this._endV2 = UNSET_TIME;
      }
    }

    this._render();
  }

  private _syncFromMultiExplore(): void {
    const multiExplore = this.querySelector<any>('#multi-explore');
    if (multiExplore) {
      if (multiExplore.onlyRegressions !== undefined)
        this._onlyRegressionsV2 = multiExplore.onlyRegressions;
      if (multiExplore.splitAll !== undefined) this._splitAllV2 = multiExplore.splitAll;
      if (multiExplore.showSparklines !== undefined)
        this._showSparklinesV2 = multiExplore.showSparklines;
      if (multiExplore.pageSize !== undefined) this._pageSizeV2 = multiExplore.pageSize;
    }
  }

  // findRequestedAnomalies returns a list of requested anomaly objects .
  // This is for only loading selected untriaged anomaly graphs in the first place.
  findRequestedAnomalies(): Anomaly[] {
    const ret: Anomaly[] = [];
    this.anomalyTracker.toAnomalyList().forEach((anomaly) => {
      if (this.requestAnomalies.includes(anomaly.id)) {
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
                    <a href="${commit.url}" target="_blank">${TrimHash(commit.hash)}</a>
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
