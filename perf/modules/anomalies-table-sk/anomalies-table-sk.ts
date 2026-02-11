/**
 * @module modules/anomalies-table-sk
 * @description <h2><code>anomalies-table-skr: </code></h2>
 *
 * Display table of anomalies
 */

import { html, TemplateResult, LitElement } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';

import '../../../infra-sk/modules/sort-sk';
import { Anomaly, RegressionBug } from '../json';
import {
  AnomalyGroup,
  RevisionGroupingMode,
  GroupingCriteria,
  AnomalyGroupingConfig,
} from './grouping';

import { formatPercentage } from '../common/anomaly';
import '../window/window';
import { TriageMenuSk } from '../triage-menu-sk/triage-menu-sk';
import '../triage-menu-sk/triage-menu-sk';
import '@material/web/button/outlined-button.js';

import { ChromeTraceFormatter } from '../trace-details-formatter/traceformatter';
import '../../../elements-sk/modules/spinner-sk';

import '../../../elements-sk/modules/icons/help-icon-sk';
import { handleKeyboardShortcut, KeyboardShortcutHandler } from '../common/keyboard-shortcuts';
import '../keyboard-shortcuts-help-sk/keyboard-shortcuts-help-sk';
import { KeyboardShortcutsHelpSk } from '../keyboard-shortcuts-help-sk/keyboard-shortcuts-help-sk';
import { SelectionController } from './selection-controller';
import { ReportNavigationController } from './report-navigation-controller';
import { AnomalyGroupingController } from './anomaly-grouping-controller';
import { AnomalyTransformer } from './anomaly-transformer';
import './anomalies-grouping-settings-sk';
import '../bug-tooltip-sk/bug-tooltip-sk';

@customElement('anomalies-table-sk')
export class AnomaliesTableSk extends LitElement implements KeyboardShortcutHandler {
  onOpenHelp(): void {
    const help = this.querySelector('keyboard-shortcuts-help-sk') as KeyboardShortcutsHelpSk;
    help.open();
  }

  @state()
  anomalyList: Anomaly[] = [];

  @state()
  showPopup: boolean = false;

  private selectionController = new SelectionController<Anomaly>(this);

  @property({ attribute: false })
  set config(val: AnomalyGroupingConfig) {
    this.groupingController.setConfig(val);
  }

  get config(): AnomalyGroupingConfig {
    return this.groupingController.config;
  }

  private reportNavigationController = new ReportNavigationController(this);

  private groupingController = new AnomalyGroupingController(this);

  private initiallyRequestedAnomalyIDs: Set<string> = new Set<string>();

  private triageMenu: TriageMenuSk | null = null;

  private headerCheckbox: HTMLInputElement | null = null;

  private traceFormatter: ChromeTraceFormatter | null = null;

  shortcutUrl: string = '';

  @state()
  private loadingGraphForAnomaly: Map<string, boolean> = new Map<string, boolean>();

  private uniqueId =
    'anomalies-table-sk-' + new Date().getTime() + '-' + Math.floor(Math.random() * 1000);

  @property({ type: Boolean, attribute: 'show-requested-groups-first', reflect: true })
  show_requested_groups_first: boolean = false;

  createRenderRoot() {
    return this;
  }

  public openAnomalyChartListener = (e: Event) => {
    const anomaly = (e as CustomEvent<Anomaly>).detail;
    if (anomaly) {
      const newTab = window.open('', '_blank');
      if (newTab) {
        newTab.document.write('Loading graph...');
      }

      void this.reportNavigationController.openMultiGraphUrl(anomaly, newTab);
    }
  };

  connectedCallback() {
    super.connectedCallback();
    // this.render(); // LitElement handles initial render

    // Move queries to firstUpdated or check for existence
    this.traceFormatter = new ChromeTraceFormatter();
    this.addEventListener('click', (e: Event) => {
      const triageButton = this.querySelector(`#triage-button-${this.uniqueId}`);
      const popup = this.querySelector('.popup');
      if (this.showPopup && !popup!.contains(e.target as Node) && e.target !== triageButton) {
        this.showPopup = false;
        // this.render();
      }
    });
    this.addEventListener('open-anomaly-chart', this.openAnomalyChartListener);
    window.addEventListener('keydown', this.keyDown);
  }

  protected firstUpdated() {
    this.triageMenu = this.querySelector(`#triage-menu-${this.uniqueId}`);
    if (this.triageMenu) {
      this.triageMenu.disableNudge();
    }
    this.headerCheckbox = this.querySelector(
      `#header-checkbox-${this.uniqueId}`
    ) as HTMLInputElement;
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    window.removeEventListener('keydown', this.keyDown);
  }

  private keyDown = (e: KeyboardEvent) => {
    // Ignore if no anomalies selected
    if (this.selectionController.size === 0) {
      return;
    }

    if (!this.triageMenu) return;

    handleKeyboardShortcut(e, this);
  };

  onTriagePositive(): void {
    this.showPopup = true;
    // this.render();
    // The second and third arguments are empty arrays because we are only
    // triaging anomalies, not trace keys or graph configs.
    this.triageMenu!.fileBug();
  }

  onTriageNegative(): void {
    this.showPopup = true;
    // this.render();
    // The second and third arguments are empty arrays because we are only
    // triaging anomalies, not trace keys or graph configs.
    this.triageMenu!.ignoreAnomaly();
  }

  onTriageExisting(): void {
    this.showPopup = true;
    // this.render();
    // The second and third arguments are empty arrays because we are only
    // triaging anomalies, not trace keys or graph configs.
    this.triageMenu!.openExistingBugDialog();
  }

  onOpenReport(): void {
    void this.openReport();
  }

  onOpenGroupReport(): void {
    void this.openAnomalyGroupReportPage();
  }

  private groupingSettingsTemplate() {
    return html`
      <anomalies-grouping-settings-sk
        .config=${{ ...this.groupingController.config }}
        @revision-mode-change=${(e: CustomEvent<RevisionGroupingMode>) =>
          this.groupingController.setRevisionMode(e.detail)}
        @group-singles-change=${(e: CustomEvent<boolean>) =>
          this.groupingController.setGroupSingles(e.detail)}
        @group-by-change=${(e: CustomEvent<{ criteria: GroupingCriteria; enabled: boolean }>) =>
          this.groupingController.toggleGroupBy(
            e.detail.criteria,
            e.detail.enabled
          )}></anomalies-grouping-settings-sk>
    `;
  }

  render() {
    const totalCount = this.anomalyList.length;
    const selectedCount = this.selectionController.size;

    // Checked only if ALL are selected
    const isAllSelected = totalCount > 0 && selectedCount === totalCount;
    // Indeterminate if SOME (but not all) are selected
    const isIndeterminate = selectedCount > 0 && selectedCount < totalCount;

    return html`
      <div class="filter-buttons" ?hidden="${this.anomalyList.length === 0}">
        <button
          id="triage-button-${this.uniqueId}"
          @click="${this.togglePopup}"
          ?disabled="${this.selectionController.size === 0}">
          Triage Selected
        </button>
        <button
          id="graph-button-${this.uniqueId}"
          @click="${this.openReport}"
          ?disabled="${this.selectionController.size === 0}">
          Graph Selected
        </button>
        <button
          id="open-group-button-${this.uniqueId}"
          @click="${this.openAnomalyGroupReportPage}"
          ?disabled="${this.selectionController.size === 0}">
          Graph Selected by Group
        </button>
        ${this.groupingSettingsTemplate()}
      </div>
      <div class="popup-container" ?hidden="${!this.showPopup}">
        <div class="popup">
          <triage-menu-sk
            id="triage-menu-${this.uniqueId}"
            .anomalies=${this.selectionController.items}
            .traceNames=${[]}></triage-menu-sk>
        </div>
      </div>
      <sort-sk id="as_table-${this.uniqueId}" target="rows-${this.uniqueId}">
        <table
          id="anomalies-table-${this.uniqueId}"
          class="anomalies-table"
          ?hidden=${this.anomalyList.length === 0}>
          <tr class="headers">
            <th id="group-${this.uniqueId}"></th>
            <th id="checkbox-${this.uniqueId}">
              <label for="header-checkbox-${this.uniqueId}"
                ><input
                  type="checkbox"
                  id="header-checkbox-${this.uniqueId}"
                  .checked=${isAllSelected}
                  .indeterminate=${isIndeterminate}
                  @change=${() => {
                    this.toggleAllCheckboxes();
                  }}
              /></label>
            </th>
            <th id="graph_header-${this.uniqueId}">Chart</th>
            <th id="bug_id-${this.uniqueId}" data-key="bugid">Bug ID</th>
            <th id="revision_range-${this.uniqueId}" data-key="revisions" data-default="down">
              Revisions
            </th>
            <th id="bot-${this.uniqueId}" data-key="bot" data-sort-type="alpha">Bot</th>
            <th id="testsuite-${this.uniqueId}" data-key="testsuite" data-sort-type="alpha">
              Test Suite
            </th>
            <th id="test-${this.uniqueId}" data-key="test" data-sort-type="alpha">Test</th>
            <th id="percent_changed-${this.uniqueId}" data-key="delta">Delta %</th>
          </tr>
          <tbody id="rows-${this.uniqueId}">
            ${this.generateGroups()}
          </tbody>
        </table>
      </sort-sk>
      <keyboard-shortcuts-help-sk .handler=${this}></keyboard-shortcuts-help-sk>
      <h1 id="clear-msg-${this.uniqueId}" ?hidden=${this.anomalyList.length > 0 || this.loading}>
        All anomalies are triaged!
      </h1>
    `;
  }

  @property({ type: Boolean })
  loading: boolean = false;

  async openReportForAnomalyIds(anomalies: Anomaly[]) {
    await this.reportNavigationController.openReportForAnomalyIds(anomalies);
  }

  async openReport() {
    await this.reportNavigationController.openReportForAnomalyIds(this.selectionController.items);
  }

  async openAnomalyGroupReportPage() {
    for (const group of this.groupingController.groups) {
      const isGroupSelected = group.anomalies.some((a) => this.selectionController.has(a));
      if (isGroupSelected) {
        await this.reportNavigationController.openReportForAnomalyIds(group.anomalies);
      }
    }
  }

  togglePopup() {
    this.showPopup = !this.showPopup;
    // this.render();
  }

  private isGroupInitiallyRequested(group: AnomalyGroup): boolean {
    return group.anomalies.some((anomaly) => this.initiallyRequestedAnomalyIDs.has(anomaly.id));
  }

  private isGroupSelected(group: AnomalyGroup): boolean {
    return group.anomalies.some((anomaly) => this.selectionController.has(anomaly));
  }

  private generateGroups() {
    if (!this.show_requested_groups_first || this.initiallyRequestedAnomalyIDs.size === 0) {
      return this.groupingController.groups.map((group) => this.generateRows(group));
    }

    const requestedGroups: AnomalyGroup[] = [];
    const otherGroups: AnomalyGroup[] = [];

    for (const group of this.groupingController.groups) {
      if (this.isGroupInitiallyRequested(group)) {
        requestedGroups.push(group);
      } else {
        otherGroups.push(group);
      }
    }

    const renderedRequested = requestedGroups.map((group) => this.generateRows(group));
    const renderedOthers = otherGroups.map((group) => this.generateRows(group, 'other-group'));

    if (requestedGroups.length === 0 || otherGroups.length === 0) {
      return [...renderedRequested, ...renderedOthers];
    }

    const separatorRow = html`
      <tr>
        <td colspan="9" class="separator-cell">
          <div class="separator-container">
            <span class="separator-line"></span>
            <span class="separator-text"
              >Other groups, related to requested ones (with overlapping commits range)</span
            >
            <span class="separator-line"></span>
          </div>
        </td>
      </tr>
    `;

    return [...renderedRequested, [separatorRow], ...renderedOthers];
  }

  private findGroupForAnomaly(anomaly: Anomaly): AnomalyGroup | null {
    for (const group of this.groupingController.groups) {
      if (group.anomalies.find((a) => a.id === anomaly.id)) {
        return group;
      }
    }
    return null;
  }

  private _updateCheckedState(chkbox: HTMLInputElement | null, a: Anomaly) {
    if (chkbox) {
      if (chkbox.checked) {
        this.selectionController.select(a);
      } else {
        this.selectionController.deselect(a);
      }
    } else {
      this.selectionController.toggle(a);
    }

    this.dispatchEvent(
      new CustomEvent('anomalies_checked', {
        detail: {
          anomalies: [a],
          checked: this.selectionController.has(a),
        },
        bubbles: true,
      })
    );
  }

  private anomalyChecked(chkbox: HTMLInputElement | null, a: Anomaly) {
    this._updateCheckedState(chkbox, a);
    // this.render();
  }

  private generateRows(anomalyGroup: AnomalyGroup, rowClass: string = ''): TemplateResult[] {
    const rows: TemplateResult[] | never = [];
    const length = anomalyGroup.anomalies.length;
    if (length > 1) {
      rows.push(this.generateSummaryRow(anomalyGroup, rowClass));
      // optimization: if collapsed, do not render children
      if (!anomalyGroup.expanded) {
        return rows;
      }
    }

    // If it's a single item group, we always render it (it's not "expandable" in the same sense).
    // If it's a multi-item group, we only reach here if it is expanded.

    for (let i = 0; i < anomalyGroup.anomalies.length; i++) {
      const anomalySortValues = AnomalyTransformer.getProcessedAnomaly(anomalyGroup.anomalies[i]);
      const anomaly = anomalyGroup.anomalies[i];
      const processedAnomaly = AnomalyTransformer.getProcessedAnomaly(anomaly);
      const anomalyClass = processedAnomaly.isImprovement ? 'improvement' : 'regression';
      const isLoading = this.loadingGraphForAnomaly.get(anomaly.id) || false;
      rows.push(html`
        <tr
          data-bugid="${anomalySortValues.bugId}"
          data-revisions="${anomalySortValues.revision}"
          data-bot="${anomalySortValues.bot}"
          data-testsuite="${anomalySortValues.testsuite}"
          data-test="${anomalySortValues.test}"
          data-delta="${anomalySortValues.delta}"
          class="${this.getRowClass(i + 1, anomalyGroup)} ${rowClass}">
          <td></td>
          <td>
            <label
              ><input
                type="checkbox"
                @change=${(e: Event) => {
                  this.anomalyChecked(e.target as HTMLInputElement, anomaly);
                }}
                .checked=${this.selectionController.has(anomaly)}
                id="anomaly-row-${this.uniqueId}-${anomaly.id}"
            /></label>
          </td>
          <td class="center-content">
            ${isLoading
              ? html`<spinner-sk active></spinner-sk>` // Show spinner if loading
              : html`
                  <button
                    class="trendingicon-link"
                    @click=${async () => {
                      const newTab = window.open('', '_blank');
                      if (newTab) {
                        newTab.document.write('Loading graph...');
                      }

                      this.loadingGraphForAnomaly.set(anomaly.id, true);
                      this.requestUpdate();

                      await this.reportNavigationController.openMultiGraphUrl(anomaly, newTab);

                      this.loadingGraphForAnomaly.set(anomaly.id, false);
                      this.requestUpdate();
                    }}>
                    <trending-up-icon-sk></trending-up-icon-sk>
                  </button>
                `}
          </td>
          <td class="tooltip-cell">
            ${this.getReportLinkForBugId(anomaly.bug_id)}
            <close-icon-sk
              id="btnUnassociate"
              @click=${() => {
                this.triageMenu!.makeEditAnomalyRequest([anomaly], [], 'RESET');
              }}
              ?hidden=${anomaly!.bug_id === 0}>
            </close-icon-sk>
            <bug-tooltip-sk
              .bugs=${anomalyGroup.anomalies[i].bugs || []}
              totalLabel="total"></bug-tooltip-sk>
          </td>
          <td>
            <span
              >${AnomalyTransformer.computeRevisionRange(
                anomaly.start_revision,
                anomaly.end_revision
              )}</span
            >
          </td>
          <td>${processedAnomaly.bot}</td>
          <td>${processedAnomaly.testsuite}</td>
          <td>${processedAnomaly.test}</td>
          <td class=${anomalyClass}>${formatPercentage(processedAnomaly.delta)}%</td>
        </tr>
      `);
    }
    return rows;
  }

  private generateSummaryRow(anomalyGroup: AnomalyGroup, rowClass: string = ''): TemplateResult {
    if (!anomalyGroup.anomalies || anomalyGroup.anomalies.length === 0) {
      return html``; // Handle empty group
    }

    const improvements = anomalyGroup.anomalies.filter((a) => a.is_improvement).length;
    const regressions = anomalyGroup.anomalies.length - improvements;
    const selectedCount = anomalyGroup.anomalies.filter((a) =>
      this.selectionController.has(a)
    ).length;
    const totalCount = anomalyGroup.anomalies.length;

    // It is checked ONLY if ALL are selected
    const isChecked = totalCount > 0 && selectedCount === totalCount;
    // It is indeterminate if SOME (but not all) are selected
    const isIndeterminate = selectedCount > 0 && selectedCount < totalCount;

    const [deltaValue, isSummaryRegression] =
      AnomalyTransformer.determineSummaryDelta(anomalyGroup);
    const summaryClass = isSummaryRegression ? 'regression' : 'improvement';

    const firstAnomaly = anomalyGroup.anomalies[0];
    const processedAnomalies = anomalyGroup.anomalies.map((a) =>
      AnomalyTransformer.getProcessedAnomaly(a)
    );
    const firstProcessed = processedAnomalies[0];

    // Aggregate Revisions
    const minStartRevision = Math.min(...anomalyGroup.anomalies.map((a) => a.start_revision));
    const maxEndRevision = Math.max(...anomalyGroup.anomalies.map((a) => a.end_revision));

    // Check for consistency
    const allSameBot = processedAnomalies.every((p) => p.bot === firstProcessed.bot);
    const allSameTestSuite = processedAnomalies.every(
      (p) => p.testsuite === firstProcessed.testsuite
    );

    const summaryData = {
      startRevision: minStartRevision,
      endRevision: maxEndRevision,
      bot: allSameBot ? firstProcessed.bot : '*',
      testsuite: allSameTestSuite ? firstProcessed.testsuite : '*',
      test: AnomalyTransformer.findLongestSubTestPath(anomalyGroup.anomalies),
      delta: deltaValue,
    };

    const anomalyForBugReportLink = this.getReportLinkForSummaryRowBugId(anomalyGroup);
    const bugIdForLink = anomalyForBugReportLink ? anomalyForBugReportLink.bug_id : 0;
    const summaryBugs = this.getSummaryBugs(anomalyGroup.anomalies);

    return html`
      <tr
        data-bugid="${bugIdForLink}"
        data-revisions="${summaryData.endRevision}"
        data-bot="${summaryData.bot}"
        data-testsuite="${summaryData.testsuite}"
        data-test="${summaryData.test}"
        data-delta="${summaryData.delta}"
        class="${this.getRowClass(0, anomalyGroup)} ${rowClass}">
        <td>
          <button
            class="expand-button"
            @click=${() => this.expandGroup(anomalyGroup)}
            ?hidden=${anomalyGroup.anomalies.length === 1}>
            <span class="regression">${regressions}</span> |
            <span class="improvement">${improvements}</span>
          </button>
        </td>
        <td>
          <label>
            <input
              type="checkbox"
              @change="${() => {
                this.toggleChildrenCheckboxes(anomalyGroup);
              }}"
              .checked=${isChecked}
              .indeterminate=${isIndeterminate}
              id="anomaly-row-${this.uniqueId}-${this.getGroupId(anomalyGroup)}" />
          </label>
        </td>
        <td class="center-content"></td>
        <td class="tooltip-cell">
          ${this.getReportLinkForBugId(bugIdForLink)}
          <close-icon-sk
            id="btnUnassociate"
            @click=${() => {
              this.triageMenu!.makeEditAnomalyRequest(
                [anomalyForBugReportLink || firstAnomaly],
                [],
                'RESET'
              );
            }}
            ?hidden=${!anomalyForBugReportLink}>
          </close-icon-sk>
          <bug-tooltip-sk .bugs=${summaryBugs || []} totalLabel="distinct total"></bug-tooltip-sk>
        </td>
        <td>
          <span
            >${AnomalyTransformer.computeRevisionRange(
              summaryData.startRevision,
              summaryData.endRevision
            )}</span
          >
        </td>
        <td>${summaryData.bot}</td>
        <td>${summaryData.testsuite}</td>
        <td>${summaryData.test}</td>
        <td class=${summaryClass}>${formatPercentage(summaryData.delta)}%</td>
      </tr>
    `;
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
    if (anomalyGroup.anomalies.every((a) => a.bug_id === -2)) {
      return anomalyGroup.anomalies[0];
    }
    for (const anomaly of anomalyGroup.anomalies) {
      if (anomaly.bug_id !== null && anomaly.bug_id !== 0 && anomaly.bug_id !== -2) {
        return anomaly;
      }
    }
    return undefined;
  }

  getRowClass(index: number, anomalyGroup: AnomalyGroup) {
    if (anomalyGroup.expanded === true) {
      if (index === 0) {
        return 'parent-expanded-row';
      } else {
        return 'child-expanded-row';
      }
    }
    return '';
  }

  // Returns all distinct bugs in this FE group.
  getSummaryBugs(anomalies: Anomaly[]) {
    const distinctBugsMap = new Map<string, RegressionBug>();
    anomalies
      .flatMap((a) => a.bugs || [])
      .forEach((bug) => {
        if (bug) {
          const key = `${bug.bug_id}-${bug.bug_type}`;
          if (!distinctBugsMap.has(key)) {
            distinctBugsMap.set(key, bug);
          }
        }
      });
    return Array.from(distinctBugsMap.values());
  }

  expandGroup(anomalyGroup: AnomalyGroup) {
    anomalyGroup.expanded = !anomalyGroup.expanded;
    this.requestUpdate();
  }

  async populateTable(anomalyList: Anomaly[]): Promise<void> {
    this.initiallyRequestedAnomalyIDs.clear();
    this.selectionController.clear();

    // We update state, Lit handles DOM updates.
    // However, if anomalyList is empty, we show clear-msg and hide table.
    // In Render, we have logic for ?hidden.

    // We must assign to this.anomalyList to trigger update.
    this.anomalyList = anomalyList;
    if (this.anomalyList.length > 0) {
      this.groupingController.setAnomalies(this.anomalyList);
    }
    // this.render(); handled by state change
  }

  /**
   * Set checkboxes to true for list of provided anomalies.
   * @param anomalyList
   */
  checkSelectedAnomalies(anomalyList: Anomaly[]): void {
    this.initiallyRequestedAnomalyIDs = new Set<string>(anomalyList.map((a) => a.id));
    this.anomalyList.forEach((anomaly) => {
      if (this.initiallyRequestedAnomalyIDs.has(anomaly.id)) {
        this.selectionController.select(anomaly);
      }
    });

    this.triageMenu!.toggleButtons(this.selectionController.items.length > 0);
    this.requestUpdate();
  }

  private checkAnomaly(checkedAnomaly: Anomaly) {
    this.selectionController.select(checkedAnomaly);

    this.anomalyChecked(null, checkedAnomaly);
  }

  /**
   * Toggles the checked state of all child checkboxes within an anomaly group when the
   * group is collapsed. This allows the user to check/uncheck all children anomalies
   * at once by interacting with the parent checkbox.
   */
  toggleChildrenCheckboxes(anomalyGroup: AnomalyGroup) {
    const summaryRowCheckbox = this.querySelector<HTMLInputElement>(
      `input[id="anomaly-row-${this.uniqueId}-${this.getGroupId(anomalyGroup)}"]`
    ) as HTMLInputElement;
    const checked = summaryRowCheckbox.checked;

    const affectedAnomalies = anomalyGroup.anomalies;
    affectedAnomalies.forEach((anomaly) => {
      // We do not need to update individual checkboxes here because we are
      // re-rendering the whole table at the end of this function.
      if (checked) {
        this.selectionController.select(anomaly);
      } else {
        this.selectionController.deselect(anomaly);
      }
    });

    this.dispatchEvent(
      new CustomEvent('anomalies_checked', {
        detail: {
          anomalies: affectedAnomalies,
          checked: checked,
        },
        bubbles: true,
      })
    );

    // Header checkbox logic is now in the template (auto-calculated) or we update it?
    // The template calculates isAllSelected/isIndeterminate for the header based on checkedAnomaliesSet.
    // So if checkedAnomaliesSet is correct, header checkbox will render correctly.
    // BUT we need to notify Lit that checkedAnomaliesSet changed.

    this.requestUpdate();
  }

  /**
   * Toggles the 'checked' state of all checkboxes in the table based on the state of
   * the header checkbox. This provides a convenient way to select or deselect all
   * anomalies at once.
   */
  toggleAllCheckboxes() {
    // Header checkbox state is not automatically available if we use .checked=${...} property binding
    // without also reading 'changed' event target.
    // But inside the event handler in render(), we call this.
    // We should look at headerCheckbox element or just toggle based on current state.

    // Better: check the element state.
    const headerCheckbox = this.querySelector(
      `#header-checkbox-${this.uniqueId}`
    ) as HTMLInputElement;
    const checked = headerCheckbox.checked;

    // Actually, if we use @change, the element.checked is already updated by browser.

    if (checked) {
      this.anomalyList.forEach((a) => this.selectionController.select(a));
    } else {
      this.selectionController.clear();
    }

    this.dispatchEvent(
      new CustomEvent('anomalies_checked', {
        detail: {
          anomalies: this.anomalyList,
          checked: checked,
        },
        bubbles: true,
      })
    );

    this.requestUpdate();
  }

  public async openMultiGraphUrl(anomaly: Anomaly, newTab: Window | null) {
    console.log('AnomaliesTableSk.openMultiGraphUrl called');
    await this.reportNavigationController.openMultiGraphUrl(anomaly, newTab);
  }

  getCheckedAnomalies(): Anomaly[] {
    return this.selectionController.items;
  }

  initialCheckAllCheckbox() {
    // This expects DOM to be ready.
    // If called before render, headerCheckbox might be null.
    // In legacy, it was called manually?
    // It seems unused in the provided code snippet, but might be used by parent.
    // Safe to check for null.
    if (this.headerCheckbox) {
      this.headerCheckbox.checked = true;
      this.toggleAllCheckboxes();
    }
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
