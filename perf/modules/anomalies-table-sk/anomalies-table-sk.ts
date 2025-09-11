/**
 * @module modules/anomalies-table-sk
 * @description <h2><code>anomalies-table-skr: </code></h2>
 *
 * Display table of anomalies
 */

import { html, TemplateResult } from 'lit/html.js';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { define } from '../../../elements-sk/modules/define';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import '../../../infra-sk/modules/sort-sk';
import { Anomaly, GetGroupReportResponse, Timerange } from '../json';
import { GraphConfig } from '../explore-simple-sk/explore-simple-sk';
import { AnomalySk } from '../anomaly-sk/anomaly-sk';
import '../window/window';
import { TriageMenuSk } from '../triage-menu-sk/triage-menu-sk';
import '../triage-menu-sk/triage-menu-sk';
import '@material/web/button/outlined-button.js';
import { errorMessage } from '../errorMessage';
import { updateShortcut } from '../explore-simple-sk/explore-simple-sk';
import { ChromeTraceFormatter } from '../trace-details-formatter/traceformatter';
import '../../../elements-sk/modules/spinner-sk';

const weekInSeconds = 7 * 24 * 60 * 60;
class AnomalyGroup {
  anomalies: Anomaly[] = [];

  expanded: boolean = false;
}

export class AnomaliesTableSk extends ElementSk {
  anomalyList: Anomaly[] = [];

  anomalyGroups: AnomalyGroup[] = [];

  showPopup: boolean = false;

  private checkedAnomaliesSet: Set<Anomaly> = new Set<Anomaly>();

  private triageMenu: TriageMenuSk | null = null;

  private headerCheckbox: HTMLInputElement | null = null;

  private traceFormatter: ChromeTraceFormatter | null = null;

  private shortcutUrl: string = '';

  getGroupReportResponse: GetGroupReportResponse | null = null;

  private loadingGraphForAnomaly: Map<string, boolean> = new Map<string, boolean>();

  multiChartUrlToAnomalyMap: Map<string, string> = new Map<string, string>();

  private regressionsPageHost = '/a/';

  private reportPageHost = '/u/';

  private isParentRow = false;

  constructor() {
    super(AnomaliesTableSk.template);
  }

  public openAnomalyChartListener = (e: Event) => {
    const anomaly = (e as CustomEvent<Anomaly>).detail;
    if (anomaly) {
      this.openMultiGraphUrl(anomaly);
    }
  };

  async connectedCallback() {
    super.connectedCallback();
    this._render();

    this.triageMenu = this.querySelector('#triage-menu');
    this.triageMenu!.disableNudge();
    this.triageMenu!.toggleButtons(this.checkedAnomaliesSet.size > 0);
    this.headerCheckbox = this.querySelector('#header-checkbox') as HTMLInputElement;
    this.traceFormatter = new ChromeTraceFormatter();
    this.addEventListener('click', (e: Event) => {
      const triageButton = this.querySelector('#triage-button');
      const popup = this.querySelector('.popup');
      if (this.showPopup && !popup!.contains(e.target as Node) && e.target !== triageButton) {
        this.showPopup = false;
        this._render();
      }
    });
    this.addEventListener('open-anomaly-chart', this.openAnomalyChartListener);
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    this.removeEventListener('open-anomaly-chart', this.openAnomalyChartListener);
  }

  private static template = (ele: AnomaliesTableSk) => html`
    <div class="filter-buttons" ?hidden="${ele.anomalyList.length === 0}">
      <button
        id="triage-button"
        @click="${ele.togglePopup}"
        ?disabled="${ele.checkedAnomaliesSet.size === 0}">
        Triage Selected
      </button>
      <button
        id="graph-button"
        @click="${ele.openReport}"
        ?disabled="${ele.checkedAnomaliesSet.size === 0}">
        Graph Selected
      </button>
      <button
        id="open-group-button"
        @click="${ele.openAnomalyGroupReportPage}"
        ?disabled="${ele.checkedAnomaliesSet.size === 0}">
        Graph Selected by Group
      </button>
    </div>
    <div class="popup-container" ?hidden="${!ele.showPopup}">
      <div class="popup">
        <triage-menu-sk id="triage-menu"></triage-menu-sk>
      </div>
    </div>
    ${ele.generateTable()}
    <h1 id="clear-msg" hidden>All anomalies are triaged!</h1>
  `;

  async openReport() {
    const idList = [...this.checkedAnomaliesSet].map((a) => a.id);

    // If only one anomaly is selected, open the report page using
    // the anomaly id directly.
    // TODO(b/384952008): offload the handling to backend.
    if (idList.length === 1) {
      const key = idList[0];
      window.open(`/u/?anomalyIDs=${key}`, '_blank');
      return;
    }

    const idString = idList.join(',');
    // TODO(wenbinzhang): ideally, we should open the url:
    //   /u/?keys=idString.
    // Then from the report-page-sk.ts, we can call
    //   /_anomalies/group_report?keys=idString.
    // From the response, we can use the .anomaly_list to
    // populate the tablem and use the .sid to update the url.
    // As the report-page-sk.ts is not finalized yet, I'm puting
    // the logic here to make the implementation more clear.
    // Though, this will cause one extra call to Chromeperf, which
    // will slow down the repsonse time.
    // I will move this to report-page-sk when the page is ready.
    await this.fetchGroupReportApi(idString);

    const sid: string = this.getGroupReportResponse!.sid || '';
    const url = `/u/?sid=${sid}`;
    window.open(url, '_blank');
  }

  async openAnomalyGroupReportPage() {
    for (const group of this.anomalyGroups) {
      const summaryRowCheckboxId = this.getGroupId(group);
      const groupCheckbox = this.querySelector<HTMLInputElement>(
        `input[id="anomaly-row-${summaryRowCheckboxId}"]`
      );
      if (groupCheckbox && groupCheckbox!.checked) {
        const idList = [...group.anomalies].map((a) => a.id);
        const idString = idList.join(',');
        await this.fetchGroupReportApi(idString);

        const sid: string = this.getGroupReportResponse!.sid || '';
        const url = `/u/?sid=${sid}`;
        window.open(url, '_blank');
      }
    }
  }

  togglePopup() {
    this.showPopup = !this.showPopup;
    if (this.showPopup) {
      const triageMenu = this.querySelector('#triage-menu') as TriageMenuSk;
      triageMenu.setAnomalies(Array.from(this.checkedAnomaliesSet), [], []);
    }
    this._render();
  }

  doRangesOverlap(a: Anomaly, b: Anomaly): boolean {
    if (a.start_revision > b.start_revision) {
      [a, b] = [b, a];
    }

    if (
      a.start_revision === null ||
      a.end_revision === null ||
      b.start_revision === null ||
      b.end_revision === null
    ) {
      return false;
    }
    return a.start_revision <= b.end_revision && a.end_revision >= b.start_revision;
  }

  /**
   * Helper method to group anomalies based on a predicate.
   *
   * It takes a list of anomalies, groups them, and then partitions the result
   * into groups containing multiple items and a flat list of anomalies that
   * remained in single-item groups.
   *
   * @param anomalies - The list of anomalies to group.
   * @param predicate - A function that returns true if two anomalies belong in the same group.
   * @returns An object containing the grouped anomalies and the remaining single anomalies.
   */
  private groupAndPartition(
    anomalies: Anomaly[],
    predicate: (a: Anomaly, b: Anomaly) => boolean
  ): { multiItemGroups: AnomalyGroup[]; singleAnomalies: Anomaly[] } {
    if (!anomalies.length) {
      return { multiItemGroups: [], singleAnomalies: [] };
    }

    // Use reduce to iterate once and create all groups.
    const allGroups = anomalies.reduce((groups: AnomalyGroup[], anomaly) => {
      const existingGroup = groups.find((g) =>
        g.anomalies.every((other) => predicate(anomaly, other))
      );

      if (existingGroup) {
        existingGroup.anomalies.push(anomaly);
      } else {
        groups.push({ anomalies: [anomaly], expanded: false });
      }
      return groups;
    }, []);

    // Now, partition the results into multi-item groups and singles.
    const multiItemGroups: AnomalyGroup[] = [];
    const singleAnomalies: Anomaly[] = [];
    for (const group of allGroups) {
      if (group.anomalies.length > 1) {
        multiItemGroups.push(group);
      } else {
        singleAnomalies.push(group.anomalies[0]);
      }
    }

    return { multiItemGroups, singleAnomalies };
  }

  /**
   * Groups anomalies based on a hierarchy of criteria:
   * 1. By shared bug_id.
   * 2. By overlapping revision range.
   * 3. By the exact same revision.
   * 4. By the same benchmark.
   * Any remaining anomalies are left in their own individual groups.
   */
  groupAnomalies() {
    // First, separate anomalies that have a bug_id from those that don't.
    const withBugId: Anomaly[] = [];
    const withoutBugId: Anomaly[] = [];
    for (const anomaly of this.anomalyList) {
      if (anomaly.bug_id && anomaly.bug_id > 0) {
        withBugId.push(anomaly);
      } else {
        withoutBugId.push(anomaly);
      }
    }

    // Second, create groups for anomalies sharing a bug_id.
    const bugIdGroupMap = withBugId.reduce((map, anomaly) => {
      const bugId = anomaly.bug_id!;
      const group = map.get(bugId) || [];
      map.set(bugId, [...group, anomaly]);
      return map;
    }, new Map<number, Anomaly[]>());

    const bugIdGroups: AnomalyGroup[] = Array.from(bugIdGroupMap.values()).map((anomalies) => ({
      anomalies,
      expanded: false,
    }));

    // Third, sequentially group the remaining anomalies using the helper.
    const { multiItemGroups: revisionGroups, singleAnomalies: remainingAfterRevision } =
      this.groupAndPartition(withoutBugId, (a, b) => this.isSameRevision(a, b));

    const { multiItemGroups: sameRevisionGroups, singleAnomalies: remainingAfterSameRevision } =
      this.groupAndPartition(remainingAfterRevision, (a, b) => this.doRangesOverlap(a, b));

    const { multiItemGroups: sameBenchmarkGroups, singleAnomalies: finalSingles } =
      this.groupAndPartition(remainingAfterSameRevision, (a, b) => this.isSameBenchmark(a, b));

    // Fourth, any anomalies that were never grouped become their own single-item groups.
    const singleAnomalyGroups: AnomalyGroup[] = finalSingles.map((anomaly) => ({
      anomalies: [anomaly],
      expanded: false,
    }));

    // Last, combine all groups into the final list.
    this.anomalyGroups = [
      ...bugIdGroups,
      ...revisionGroups,
      ...sameRevisionGroups,
      ...sameBenchmarkGroups,
      ...singleAnomalyGroups,
    ];
  }

  isSameBenchmark(a: Anomaly, b: Anomaly) {
    const testSuiteA = a.test_path.split('/').length > 2 ? a.test_path.split('/')[2] : '';
    const testSuiteB = b.test_path.split('/').length > 2 ? b.test_path.split('/')[2] : '';
    return testSuiteA === testSuiteB;
  }

  isSameRevision(a: Anomaly, b: Anomaly) {
    return a.start_revision === b.start_revision && a.end_revision === b.end_revision;
  }

  private generateTable() {
    return html`
      <sort-sk id="as_table" target="rows">
        <table id="anomalies-table" hidden>
          <tr class="headers">
            <th id="group"></th>
            <th id="checkbox">
              <label for="header-checkbox"
                ><input type="checkbox" id="header-checkbox" @change=${this.toggleAllCheckboxes}
              /></label>
            </th>
            <th id="graph_header">Chart</th>
            <th id="bug_id" data-key="bugid">Bug ID</th>
            <th id="revision_range" data-key="revisions" data-default="down">Revisions</th>
            <th id="bot" data-key="bot" data-sort-type="alpha">Bot</th>
            <th id="testsuite" data-key="testsuite" data-sort-type="alpha">Test Suite</th>
            <th id="test" data-key="test" data-sort-type="alpha">Test</th>
            <th id="percent_changed" data-key="delta">Delta %</th>
          </tr>
          <tbody id="rows">
            ${this.generateGroups()}
          </tbody>
        </table>
      </sort-sk>
    `;
  }

  private generateGroups() {
    const groups: TemplateResult[][] = [];
    for (let i = 0; i < this.anomalyGroups.length; i++) {
      const anomalyGroup = this.anomalyGroups[i];
      groups.push(this.generateRows(anomalyGroup) as TemplateResult[]);
    }
    return groups;
  }

  private async preGenerateMultiGraphUrl(timerangeMap: {
    [key: string]: Timerange;
  }): Promise<void> {
    await this.generateMultiGraphUrl(this.anomalyList, timerangeMap);
  }

  private findGroupForAnomaly(anomaly: Anomaly): AnomalyGroup | null {
    for (const group of this.anomalyGroups) {
      if (group.anomalies.find((a) => a.id === anomaly.id)) {
        return group;
      }
    }
    return null;
  }

  private _updateCheckedState(
    chkbox: HTMLInputElement,
    a: Anomaly,
    anomalyGroup: AnomalyGroup | null
  ) {
    if (chkbox.checked) {
      this.checkedAnomaliesSet.add(a);
    } else {
      this.checkedAnomaliesSet.delete(a);
    }

    const group = anomalyGroup || this.findGroupForAnomaly(a);

    // Update summary checkbox state.
    if (group && group.anomalies.length > 1) {
      const summaryRowCheckboxId = this.getGroupId(group);
      const summaryCheckbox = this.querySelector<HTMLInputElement>(
        `input[id="anomaly-row-${summaryRowCheckboxId}"]`
      );
      if (summaryCheckbox) {
        let checkedCount = 0;
        for (const anomaly of group.anomalies) {
          if (this.checkedAnomaliesSet.has(anomaly)) {
            checkedCount++;
          }
        }

        if (checkedCount === 0) {
          summaryCheckbox.indeterminate = false;
          summaryCheckbox.checked = false;
        } else if (checkedCount === group.anomalies.length) {
          summaryCheckbox.checked = true;
          summaryCheckbox.indeterminate = false;
        } else {
          summaryCheckbox.checked = false;
          summaryCheckbox.indeterminate = true;
        }
      }
    }

    if (this.checkedAnomaliesSet.size === 0) {
      this.headerCheckbox!.indeterminate = false;
      this.headerCheckbox!.checked = false;
    } else if (this.checkedAnomaliesSet.size === this.anomalyList.length) {
      this.headerCheckbox!.indeterminate = false;
      this.headerCheckbox!.checked = true;
    } else {
      this.headerCheckbox!.checked = false;
      this.headerCheckbox!.indeterminate = true;
    }

    this.dispatchEvent(
      new CustomEvent('anomalies_checked', {
        detail: {
          anomaly: a,
          checked: chkbox.checked,
        },
        bubbles: true,
      })
    );

    this.triageMenu!.toggleButtons(this.checkedAnomaliesSet.size > 0);
  }

  private anomalyChecked(chkbox: HTMLInputElement, a: Anomaly, anomalyGroup: AnomalyGroup | null) {
    this._updateCheckedState(chkbox, a, anomalyGroup);
    this._render();
  }

  private getProcessedAnomaly(anomaly: Anomaly) {
    const bugId = anomaly.bug_id;
    const testPathPieces = anomaly.test_path.split('/');
    const bot = testPathPieces[1];
    const testsuite = testPathPieces[2];
    const test = testPathPieces.slice(3, testPathPieces.length).join('/');
    const revision = anomaly.start_revision;
    const delta = AnomalySk.getPercentChange(
      anomaly.median_before_anomaly,
      anomaly.median_after_anomaly
    );
    return {
      bugId,
      revision,
      bot,
      testsuite,
      test,
      delta,
    };
  }

  private generateRows(anomalyGroup: AnomalyGroup): TemplateResult[] {
    const rows: TemplateResult[] | never = [];
    const length = anomalyGroup.anomalies.length;
    if (length > 1) {
      rows.push(this.generateSummaryRow(anomalyGroup));
    }

    for (let i = 0; i < anomalyGroup.anomalies.length; i++) {
      const anomalySortValues = this.getProcessedAnomaly(anomalyGroup.anomalies[i]);
      const anomaly = anomalyGroup.anomalies[i];
      const processedAnomaly = this.getProcessedAnomaly(anomaly);
      const anomalyClass = processedAnomaly.delta > 0 ? 'improvement' : 'regression';
      const isLoading = this.loadingGraphForAnomaly.get(anomaly.id) || false;
      rows.push(html`
        <tr
          data-bugid="${anomalySortValues.bugId}"
          data-revisions="${anomalySortValues.revision}"
          data-bot="${anomalySortValues.bot}"
          data-testsuite="${anomalySortValues.testsuite}"
          data-test="${anomalySortValues.test}"
          data-delta="${anomalySortValues.delta}"
          class=${this.getRowClass(i + 1, anomalyGroup)}
          ?hidden=${
            !anomalyGroup.expanded && !this.isParentRow && anomalyGroup.anomalies.length > 1
          }>
          <td>
          </td>
          <td>
            <label><input type="checkbox"
              @change=${(e: Event) => {
                // If we just need to check 1 anomaly, just mark it as checked.
                if (length === 1 || anomalyGroup.expanded) {
                  this.anomalyChecked(e.target as HTMLInputElement, anomaly, anomalyGroup);
                } else {
                  // If the the summary row gets checked, check all children anomalies.
                  this.toggleChildrenCheckboxes(anomalyGroup);
                }
              }}
              id="anomaly-row-${anomaly.id}"></label>
          </td>
          <td class="center-content">
            ${
              isLoading
                ? html`<spinner-sk active></spinner-sk>` // Show spinner if loading
                : html`
                    <button
                      id="trendingicon-link"
                      @click=${async () => {
                        this.loadingGraphForAnomaly.set(anomaly.id, true);
                        this._render();
                        await this.openMultiGraphUrl(anomaly);
                        this.loadingGraphForAnomaly.set(anomaly.id, false);
                        this._render();
                      }}>
                      <trending-up-icon-sk></trending-up-icon-sk>
                    </button>
                  `
            }
          </td>
          <td>
            ${this.getReportLinkForBugId(anomaly.bug_id)}
            <close-icon-sk
              id="btnUnassociate"
              @click=${() => {
                this.triageMenu!.makeEditAnomalyRequest([anomaly], [], 'RESET');
              }}
              ?hidden=${anomaly!.bug_id === 0}>
            </close-icon-sk>
          </td>
          <td>
              <span
                >${this.computeRevisionRange(anomaly.start_revision, anomaly.end_revision)}</span
              >
            </a>
          </td>
          <td>${processedAnomaly.bot}</td>
          <td>${processedAnomaly.testsuite}</td>
          <td>${processedAnomaly.test}</td>
          <td class=${anomalyClass}>${AnomalySk.formatPercentage(processedAnomaly.delta)}%</td>
        </tr>
      `);
    }
    return rows;
  }

  private generateSummaryRow(anomalyGroup: AnomalyGroup): TemplateResult {
    const firstAnomaly = anomalyGroup.anomalies[0];
    const summary = {
      bugId: 0,
      startRevision: firstAnomaly.start_revision,
      endRevision: firstAnomaly.end_revision,
      bot: '*',
      testsuite: '*',
      test: '*',
      delta: 0,
    };

    let sameBot = true;
    let sameTestSuite = true;
    const firstProcessed = this.getProcessedAnomaly(firstAnomaly);
    summary.delta = firstProcessed.delta;
    for (let i = 1; i < anomalyGroup.anomalies.length; i++) {
      const processed = this.getProcessedAnomaly(anomalyGroup.anomalies[i]);
      if (processed.bot !== firstProcessed.bot) {
        sameBot = false;
      }
      if (processed.testsuite !== firstProcessed.testsuite) {
        sameTestSuite = false;
      }
      if (summary.startRevision > anomalyGroup.anomalies[i].start_revision) {
        summary.startRevision = anomalyGroup.anomalies[i].start_revision;
      }
      if (summary.endRevision < anomalyGroup.anomalies[i].end_revision) {
        summary.endRevision = anomalyGroup.anomalies[i].end_revision;
      }
      const delta = AnomalySk.getPercentChange(
        anomalyGroup.anomalies[i].median_before_anomaly,
        anomalyGroup.anomalies[i].median_after_anomaly
      );
      if (summary.delta < delta) {
        summary.delta = delta;
      }
    }
    summary.test = this.findLongestSubTestPath(anomalyGroup.anomalies);

    if (sameBot) {
      summary.bot = firstProcessed.bot;
    }
    if (sameTestSuite) {
      summary.testsuite = firstProcessed.testsuite;
    }
    const anomalyForBugReportLink = this.getReportLinkForSummaryRowBugId(anomalyGroup);
    return html`
      <tr
        data-bugid="${anomalyForBugReportLink ? anomalyForBugReportLink.bug_id : 0}"
        data-revisions="${summary.endRevision}"
        data-bot="${summary.bot}"
        data-testsuite="${summary.testsuite}"
        data-test="${summary.test}"
        data-delta="${summary.delta}"
        class="${this.getRowClass(0, anomalyGroup)}}">
        <td>
          <button
            class="expand-button"
            @click=${() => this.expandGroup(anomalyGroup)}
            ?hidden=${anomalyGroup.anomalies.length === 1}>
            ${anomalyGroup.anomalies.length}
          </button>
        </td>
        <td>
          <label
            ><input
              type="checkbox"
              @change="${() => {
                // If the summary row checkbox gets checked and the
                // group is not expanded, check all children anomalies.
                this.toggleChildrenCheckboxes(anomalyGroup);
              }}"
              id="anomaly-row-${this.getGroupId(anomalyGroup)}" />
          </label>
        </td>
        <td class="center-content"></td>
        <td>
          ${this.getReportLinkForBugId(
            anomalyForBugReportLink ? anomalyForBugReportLink.bug_id : 0
          )}
          <close-icon-sk
            id="btnUnassociate"
            @click=${() => {
              this.triageMenu!.makeEditAnomalyRequest(
                [anomalyForBugReportLink ? anomalyForBugReportLink : firstAnomaly],
                [],
                'RESET'
              );
            }}
            ?hidden=${anomalyForBugReportLink === undefined}>
          </close-icon-sk>
        </td>
        <td>
          <span>${this.computeRevisionRange(summary.startRevision, summary.endRevision)}</span>
        </td>
        <td>${summary.bot}</td>
        <td>${summary.testsuite}</td>
        <td>${summary.test}</td>
        <td>${AnomalySk.formatPercentage(summary.delta)}%</td>
      </tr>
    `;
  }

  findLongestSubTestPath(anomalyList: Anomaly[]): string {
    // Check if this character exists at the same position in all other strings.
    let longestCommonTestPath = anomalyList.at(0)!.test_path;

    for (let i = 1; i < anomalyList.length; i++) {
      const currentString = anomalyList[i].test_path;
      while (currentString.indexOf(longestCommonTestPath) !== 0) {
        longestCommonTestPath = longestCommonTestPath.substring(
          0,
          longestCommonTestPath.length - 1
        );

        if (longestCommonTestPath === '') {
          return '*';
        }
      }
    }

    // Return the common test path plus '' if the paths in the grouped rows are not the same.
    // '*' indicates where the test names differ in the collapsed rows.
    if (longestCommonTestPath.length !== anomalyList.at(0)!.test_path.length) {
      const testPath = longestCommonTestPath.split('/');
      return testPath.slice(3, testPath.length).join('/') + '*';
    }
    // else return the original test path.
    return anomalyList.at(0)!.test_path;
  }

  getReportLinkForBugId(bug_id: number) {
    if (bug_id === 0) {
      return html``;
    }
    if (bug_id === -1) {
      return html`Invalid Alert`;
    }
    if (bug_id === -2) {
      return html`Ignored Alert`;
    }
    return html`<a href="http://b/${bug_id}" target="_blank">${bug_id}</a>`;
  }

  getReportLinkForSummaryRowBugId(anomalyGroup: AnomalyGroup): Anomaly | undefined {
    for (const anomaly of anomalyGroup.anomalies) {
      if (anomaly.bug_id !== null && anomaly.bug_id !== 0) {
        return anomaly;
      }
    }
    return undefined;
  }

  getRowClass(index: number, anomalyGroup: AnomalyGroup) {
    if (anomalyGroup.expanded) {
      if (index === 0) {
        this.isParentRow = true;
        return 'parent-expanded-row';
      } else {
        this.isParentRow = false;
        return 'child-expanded-row';
      }
    }
    return '';
  }

  expandGroup(anomalyGroup: AnomalyGroup) {
    anomalyGroup.expanded = !anomalyGroup.expanded;
    this._render();
  }

  computeRevisionRange(start: number | null, end: number | null): string {
    if (start === null || end === null) {
      return '';
    }
    if (start === end) {
      return '' + end;
    }
    return start + ' - ' + end;
  }

  async populateTable(
    anomalyList: Anomaly[],
    timerangeMap: { [key: string]: Timerange }
  ): Promise<void> {
    const msg = this.querySelector('#clear-msg') as HTMLHeadingElement;
    const table = this.querySelector('#anomalies-table') as HTMLTableElement;
    if (anomalyList.length > 0) {
      msg.hidden = true;
      table.hidden = false;
      this.anomalyList = anomalyList;
      if (![this.regressionsPageHost, this.reportPageHost].includes(window.location.pathname)) {
        await this.preGenerateMultiGraphUrl(timerangeMap);
      }

      this.groupAnomalies();
      this._render();
    } else {
      msg.hidden = false;
      table.hidden = true;
    }
  }

  /**
   * Set checkboxes to true for list of provided anomalies.
   * @param anomalyList
   */
  checkSelectedAnomalies(anomalyList: Anomaly[]): void {
    anomalyList.forEach((anomaly) => {
      this.checkAnomaly(anomaly);
    });

    this._render();
  }

  private checkAnomaly(checkedAnomaly: Anomaly) {
    const checkbox = this.querySelector(
      `input[id="anomaly-row-${checkedAnomaly.id}"]`
    ) as HTMLInputElement;
    if (checkbox !== null) {
      checkbox.checked = true;
      this.anomalyChecked(checkbox, checkedAnomaly, null);
    }
  }

  /**
   * Toggles the checked state of all child checkboxes within an anomaly group when the
   * group is collapsed. This allows the user to check/uncheck all children anomalies
   * at once by interacting with the parent checkbox.
   */
  toggleChildrenCheckboxes(anomalyGroup: AnomalyGroup) {
    const summaryRowCheckbox = this.querySelector<HTMLInputElement>(
      `input[id="anomaly-row-${this.getGroupId(anomalyGroup)}"]`
    ) as HTMLInputElement;
    const checked = summaryRowCheckbox.checked;

    anomalyGroup.anomalies.forEach((anomaly) => {
      const checkbox = this.querySelector<HTMLInputElement>(
        `input[id="anomaly-row-${anomaly.id}"]`
      ) as HTMLInputElement;
      checkbox.checked = checked;
      if (checked) {
        this.checkedAnomaliesSet.add(anomaly);
      } else {
        this.checkedAnomaliesSet.delete(anomaly);
      }
      this.dispatchEvent(
        new CustomEvent('anomalies_checked', {
          detail: {
            anomaly: anomaly,
            checked: checked,
          },
          bubbles: true,
        })
      );
    });

    // Update header checkbox state.
    if (this.checkedAnomaliesSet.size === 0) {
      this.headerCheckbox!.indeterminate = false;
      this.headerCheckbox!.checked = false;
    } else if (this.checkedAnomaliesSet.size === this.anomalyList.length) {
      this.headerCheckbox!.indeterminate = false;
      this.headerCheckbox!.checked = true;
    } else {
      this.headerCheckbox!.checked = false;
    }

    this.triageMenu!.toggleButtons(this.checkedAnomaliesSet.size > 0);
    this._render();
  }

  /**
   * Toggles the 'checked' state of all checkboxes in the table based on the state of
   * the header checkbox. This provides a convenient way to select or deselect all
   * anomalies at once.
   */
  toggleAllCheckboxes() {
    const checked = this.headerCheckbox!.checked;
    this.headerCheckbox!.indeterminate = false;

    this.anomalyGroups.forEach((group) => {
      if (group.anomalies.length > 1) {
        const summaryRowCheckbox = this.querySelector<HTMLInputElement>(
          `input[id=anomaly-row-${this.getGroupId(group)}]`
        );
        if (summaryRowCheckbox) {
          summaryRowCheckbox.indeterminate = false;
          summaryRowCheckbox.checked = checked;
        }
      }

      group.anomalies.forEach((anomaly) => {
        const checkbox = this.querySelector<HTMLInputElement>(
          `input[id="anomaly-row-${anomaly.id}"]`
        );
        if (checkbox) {
          checkbox.checked = checked;
        }
        if (checked) {
          this.checkedAnomaliesSet.add(anomaly);
        } else {
          this.checkedAnomaliesSet.delete(anomaly);
        }
      });
    });
    this.triageMenu!.toggleButtons(this.checkedAnomaliesSet.size > 0);
    this._render();
  }

  public async openMultiGraphUrl(anomaly: Anomaly) {
    // Skip pre-generating the multi-chart on the Regression page(/a/)
    // to prevent spikes in page loading time.
    // For example, there's a common scenario where more than 500 rows will be initially loaded
    // when the user chooses 'V8 Javascript Perf' on the Regressions page.
    // It would significantly increase the page loading time if it pre-generates each row's url.
    // To prevent this, we will only pre-generate the URLs on the Report page.
    if (window.location.pathname !== this.regressionsPageHost) {
      const url = this.multiChartUrlToAnomalyMap.get(anomaly.id);
      if (url) {
        return this.openAnomalyUrl(url);
      } else {
        console.warn('Anomaly not found, unable to open chart.');
      }
    } else {
      await this.fetchGroupReportApi(String(anomaly.id));
      const urlList = await this.generateMultiGraphUrl(
        [anomaly],
        this.getGroupReportResponse!.timerange_map!
      );
      return this.openAnomalyUrl(urlList.at(0));
    }
  }

  //helper method to handle the async multi chart url opening
  private async openAnomalyUrl(url: string | undefined): Promise<void> {
    if (url) {
      const resolvedUrl = url;
      window.open(resolvedUrl, '_blank');
    } else {
      console.warn('multi chart not found');
    }
  }

  getCheckedAnomalies(): Anomaly[] {
    return Array.from(this.checkedAnomaliesSet);
  }

  async fetchGroupReportApi(idString: string): Promise<any> {
    await fetch('/_/anomalies/group_report', {
      method: 'POST',
      body: JSON.stringify({
        anomalyIDs: idString,
      }),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .catch((msg) => {
        errorMessage(msg);
      })
      .then((response) => {
        const json: GetGroupReportResponse = response;
        this.getGroupReportResponse = json;
      });
  }

  // openMultiGraphLink generates a multi-graph url for the given parameters
  async generateMultiGraphUrl(
    anomalies: Anomaly[],
    timerangeMap: { [key: string]: Timerange }
  ): Promise<string[]> {
    const shortcutUrlList: string[] = [];
    for (let i = 0; i < anomalies.length; i++) {
      const timerange = this.calculateTimeRange(timerangeMap[anomalies.at(i)!.id]);
      const graphConfigs = [] as GraphConfig[];
      const config: GraphConfig = {
        keys: '',
        formulas: [],
        queries: [],
      };
      config.queries = [this.traceFormatter!.formatQuery(anomalies.at(i)!.test_path)];
      graphConfigs.push(config);
      await updateShortcut(graphConfigs)
        .then((shortcut) => {
          if (shortcut === '') {
            this.shortcutUrl = '';
            return;
          }
          this.shortcutUrl = shortcut;
        })
        .catch(errorMessage);

      // request_type=0 only selects data points for within the range
      // rather than show 250 data points by default
      const url =
        `${window.location.protocol}//${window.location.host}` +
        `/m/?begin=${timerange[0]}&end=${timerange[1]}` +
        `&request_type=0&shortcut=${this.shortcutUrl}&totalGraphs=1`;
      this.multiChartUrlToAnomalyMap.set(anomalies.at(i)!.id, url);
      shortcutUrlList.push(url);
    }

    return shortcutUrlList;
  }

  calculateTimeRange(timerange: Timerange): string[] {
    if (!timerange) {
      return ['', ''];
    }
    const timerangeBegin = timerange.begin;
    const timerangeEnd = timerange.end;

    // generate data one week ahead and one week behind to make it easier
    // for user to discern trends
    const newTimerangeBegin = timerangeBegin ? (timerangeBegin - weekInSeconds).toString() : '';
    const newTimerangeEnd = timerangeEnd ? (timerangeEnd + weekInSeconds).toString() : '';

    return [newTimerangeBegin, newTimerangeEnd];
  }

  initialCheckAllCheckbox() {
    this.headerCheckbox!.checked = true;
    this.toggleAllCheckboxes();
  }

  /**
   * Generates a deterministic ID for an anomaly group based on the sorted IDs of its anomalies.
   * This ensures a unique and consistent ID for each group, preventing clashes.
   * @param anomalyGroup The anomaly group.
   * @returns A string ID for the group.
   */
  getGroupId(anomalyGroup: AnomalyGroup): string {
    return `group-${anomalyGroup.anomalies
      .map((a) => a.id)
      .sort()
      .join('-')}`;
  }
}

define('anomalies-table-sk', AnomaliesTableSk);
