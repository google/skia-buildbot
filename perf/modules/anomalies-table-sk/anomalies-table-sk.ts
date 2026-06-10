/**
 * @module modules/anomalies-table-sk
 * @description <h2><code>anomalies-table-skr: </code></h2>
 *
 * Display table of anomalies
 */

import { html, TemplateResult, LitElement } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';

import { Anomaly, RegressionBug } from '../json';
import {
  AnomalyGroup,
  RevisionGroupingMode,
  GroupingCriteria,
  AnomalyGroupingConfig,
  SummaryData,
} from './grouping';
import { errorMessage } from '../errorMessage';
import { CountMetric } from '../telemetry/telemetry';
import { StatusCodes } from 'http-status-codes';

const ANOMALIES_TABLE_SOURCE = 'anomalies-table-sk';

import { formatPercentage } from '../common/anomaly';
import '../window/window';
import { TriageMenuSk } from '../triage-menu-sk/triage-menu-sk';
import '../triage-menu-sk/triage-menu-sk';
import '@material/web/button/outlined-button.js';

import '../../../elements-sk/modules/spinner-sk';
import '../../../elements-sk/modules/icons/arrow-drop-down-icon-sk';
import '../../../elements-sk/modules/icons/filter-list-icon-sk';
import '../../../elements-sk/modules/icons/arrow-drop-up-icon-sk';
import '../../../elements-sk/modules/icons/expand-less-icon-sk';
import '../../../elements-sk/modules/icons/expand-more-icon-sk';

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

function matchesFilter(value: string, filterString: string): boolean {
  const trimmed = filterString?.trim();
  if (!trimmed) {
    return true;
  }

  try {
    return new RegExp(trimmed, 'i').test(value);
  } catch (_e) {
    // Fallback to simple substring matching if the regular expression is invalid
    return value.toLowerCase().includes(trimmed.toLowerCase());
  }
}

@customElement('anomalies-table-sk')
export class AnomaliesTableSk extends LitElement implements KeyboardShortcutHandler {
  onOpenHelp(): void {
    const help = this.querySelector('keyboard-shortcuts-help-sk') as KeyboardShortcutsHelpSk;
    help.open();
  }

  @property({ attribute: false })
  anomalyList: Anomaly[] = [];

  @property({ type: Boolean })
  showTriaged: boolean = false;

  @property({ type: Boolean, attribute: 'is-dry-run' })
  isDryRun: boolean = false;

  @state()
  showPopup: boolean = false;

  @state()
  private filterBot: string = '';

  @state()
  private filterBenchmark: string = '';

  @state()
  private filterTest: string = '';

  @state()
  private filterRevision: string = '';

  @state()
  private activeFilterPopup: string | null = null;

  // TODO(ansid): Consider computing `this.filteredAnomalies` once in `willUpdate`
  // and storing it in a private `@state` property to speed up filtering for large anomaly lists (e.g. 5k+ anomalies).
  get filteredAnomalies(): Anomaly[] {
    return this.anomalyList.filter((anomaly) => {
      const processed = AnomalyTransformer.getProcessedAnomaly(anomaly);
      if (this.filterBot && !matchesFilter(processed.bot, this.filterBot)) {
        return false;
      }
      if (this.filterBenchmark && !matchesFilter(processed.testsuite, this.filterBenchmark)) {
        return false;
      }
      if (this.filterTest && !matchesFilter(processed.test, this.filterTest)) {
        return false;
      }
      if (this.filterRevision) {
        const revVal = this.filterRevision.trim();
        const revNum = Number(revVal);
        const matchesRange =
          !isNaN(revNum) && anomaly.start_revision <= revNum && anomaly.end_revision >= revNum;
        const matchesString =
          (anomaly.start_revision !== null &&
            matchesFilter(String(anomaly.start_revision), revVal)) ||
          (anomaly.end_revision !== null && matchesFilter(String(anomaly.end_revision), revVal));
        if (!matchesRange && !matchesString) {
          return false;
        }
      }
      return true;
    });
  }

  @state()
  private sortKey: string = 'revisions';

  @state()
  private sortDirection: 'up' | 'down' = 'down';

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

  private onWindowClick = (e: Event) => {
    if (this.activeFilterPopup) {
      const composedPath = e.composedPath();
      const isInsideFilter = composedPath.some(
        (node: any) => node?.classList?.contains('header-filter-container')
      );
      if (!isInsideFilter) {
        this.activeFilterPopup = null;
      }
    }
  };

  connectedCallback() {
    super.connectedCallback();

    // Move queries to firstUpdated or check for existence
    this.addEventListener('click', (e: Event) => {
      const triageButton = this.querySelector(`#triage-button-${this.uniqueId}`);
      const popup = this.querySelector('.popup');
      if (this.showPopup && !popup!.contains(e.target as Node) && e.target !== triageButton) {
        this.showPopup = false;
      }
    });
    this.addEventListener('anomaly-changed', () => {
      this.showPopup = false;
    });
    window.addEventListener('keydown', this.keyDown);
    window.addEventListener('click', this.onWindowClick);
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
    window.removeEventListener('click', this.onWindowClick);
  }

  protected willUpdate(changedProperties: Map<string, any>): void {
    const anomalyListChanged = changedProperties.has('anomalyList');
    const showTriagedChanged = changedProperties.has('showTriaged');
    const filtersChanged = ['filterBot', 'filterBenchmark', 'filterTest', 'filterRevision'].some(
      (prop) => changedProperties.has(prop)
    );

    if (anomalyListChanged || showTriagedChanged || filtersChanged) {
      const filtered = this.filteredAnomalies;
      if (anomalyListChanged) {
        this.initiallyRequestedAnomalyIDs.clear();
        this.selectionController.clear();
      } else if (filtersChanged) {
        const filteredSet = new Set(filtered);
        this.selectionController.items.forEach((item) => {
          if (!filteredSet.has(item)) {
            this.selectionController.deselect(item);
          }
        });
      }
      this.groupingController.setAnomalies(filtered);

      // ShowTriaged button should change default sort order if sortKey is 'revisions'.
      // Otherwise, we continue sorting by whatever user selected.
      if (showTriagedChanged && this.sortKey === 'revisions') {
        this.sortDirection = this.showTriaged ? 'down' : 'up';
      }
    }
  }

  private handleSort(key: string) {
    if (this.sortKey === key) {
      this.sortDirection = this.sortDirection === 'up' ? 'down' : 'up';
    } else {
      this.sortKey = key;
      if (key === 'revisions') {
        this.sortDirection = this.showTriaged ? 'down' : 'up';
      } else {
        this.sortDirection = 'up';
      }
    }
  }

  private getSortIcon(key: string): TemplateResult {
    if (this.sortKey !== key) {
      return html``;
    }
    return this.sortDirection === 'up'
      ? html`<arrow-drop-up-icon-sk></arrow-drop-up-icon-sk>`
      : html`<arrow-drop-down-icon-sk></arrow-drop-down-icon-sk>`;
  }

  private getAriaSort(key: string): 'ascending' | 'descending' | 'none' | 'other' {
    if (this.sortKey !== key) {
      return 'none';
    }
    return this.sortDirection === 'up' ? 'ascending' : 'descending';
  }

  private sortGroups(groups: AnomalyGroup[]): AnomalyGroup[] {
    const up = this.sortDirection === 'up';
    return [...groups].sort((a, b) => {
      const valA = this.getGroupSortValue(a);
      const valB = this.getGroupSortValue(b);

      if (valA === valB) return 0;

      const comparison =
        typeof valA === 'string' && typeof valB === 'string'
          ? valA.localeCompare(valB)
          : (valA as number) > (valB as number)
            ? 1
            : -1;

      return up ? comparison : -comparison;
    });
  }

  private getGroupSortValue(group: AnomalyGroup): string | number {
    if (group.anomalies.length === 0) return '';

    // summary hasn't been calculated for the group yet, calculate & memoize it.
    if (group.summaryData.calculated === false) {
      this.calculateSummaryForGroup(group);
    }

    switch (this.sortKey) {
      case 'bugid':
        return group.summaryData.bug;
      case 'revisions':
        return group.summaryData.endRevision;
      case 'bot':
        return group.summaryData.bot;
      case 'testsuite':
        return group.summaryData.testsuite;
      case 'test':
        return group.summaryData.test;
      case 'delta':
        return group.summaryData.delta;
      default:
        return '';
    }
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
    // The second and third arguments are empty arrays because we are only
    // triaging anomalies, not trace keys or graph configs.
    this.triageMenu!.fileBug();
  }

  onTriageNegative(): void {
    this.showPopup = true;
    // The second and third arguments are empty arrays because we are only
    // triaging anomalies, not trace keys or graph configs.
    this.triageMenu!.ignoreAnomaly();
  }

  onTriageExisting(): void {
    this.showPopup = true;
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
        @group-by-change=${(e: CustomEvent<{ criteria: GroupingCriteria; enabled: boolean }>) =>
          this.groupingController.toggleGroupBy(
            e.detail.criteria,
            e.detail.enabled
          )}></anomalies-grouping-settings-sk>
    `;
  }

  private async toggleFilterPopup(e: Event, column: string) {
    e.stopPropagation();
    this.activeFilterPopup = this.activeFilterPopup === column ? null : column;
    if (this.activeFilterPopup) {
      await this.updateComplete;
      const input = this.querySelector<HTMLInputElement>(`.header-filter-popup input`);
      if (input) {
        input.focus();
        input.select();
      }
    }
  }

  private renderSortableHeader(
    sortKey: string,
    label: string,
    filterProp?: string,
    filterPlaceholder?: string
  ): TemplateResult {
    const idPrefix = sortKey;
    const hasFilter = filterProp !== undefined;
    const filterValue = hasFilter ? (this as any)[filterProp] : '';
    const isFilterActive = filterValue.trim() !== '';
    const isPopupOpen = this.activeFilterPopup === sortKey;

    return html`
      <th
        id="${idPrefix}-${this.uniqueId}"
        class="${isPopupOpen ? 'has-active-popup' : ''}"
        aria-sort="${this.getAriaSort(sortKey)}">
        <div class="header-cell-container">
          <button class="sort-button" @click=${() => this.handleSort(sortKey)}>
            ${label} ${this.getSortIcon(sortKey)}
          </button>
          ${hasFilter
            ? html`
                <div class="header-filter-container" @click=${(e: Event) => e.stopPropagation()}>
                  <button
                    class="filter-toggle-btn ${isFilterActive ? 'active' : ''}"
                    title="Filter ${label}"
                    @click=${(e: Event) => this.toggleFilterPopup(e, sortKey)}>
                    <filter-list-icon-sk></filter-list-icon-sk>
                  </button>
                  ${isPopupOpen
                    ? html`
                        <div class="header-filter-popup">
                          <input
                            type="text"
                            placeholder="${filterPlaceholder || 'Filter...'}"
                            .value="${filterValue}"
                            @input=${(e: Event) => {
                              (this as any)[filterProp] = (e.target as HTMLInputElement).value;
                            }}
                            @keydown=${(e: KeyboardEvent) => {
                              if (e.key === 'Escape' || e.key === 'Enter') {
                                this.activeFilterPopup = null;
                              }
                            }}
                            autofocus />
                          <button
                            class="clear-filter-btn"
                            title="Clear filter and close"
                            @click=${() => {
                              (this as any)[filterProp] = '';
                              this.activeFilterPopup = null;
                            }}>
                            <close-icon-sk></close-icon-sk>
                          </button>
                        </div>
                      `
                    : ''}
                </div>
              `
            : ''}
        </div>
      </th>
    `;
  }

  render() {
    const filtered = this.filteredAnomalies;
    const totalCount = filtered.length;
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
          ${this.renderSortableHeader('bugid', 'Bug ID')}
          ${this.renderSortableHeader('revisions', 'Revisions', 'filterRevision', 'Filter rev...')}
          ${this.renderSortableHeader('bot', 'Bot', 'filterBot', 'Filter bot...')}
          ${this.renderSortableHeader(
            'testsuite',
            'Benchmark',
            'filterBenchmark',
            'Filter bench...'
          )}
          ${this.renderSortableHeader('test', 'Test', 'filterTest', 'Filter test...')}
          ${this.renderSortableHeader('delta', 'Delta %')}
        </tr>
        <tbody id="rows-${this.uniqueId}">
          ${this.generateGroups()}
        </tbody>
      </table>
      <div class="empty-filter-msg" ?hidden=${this.anomalyList.length === 0 || filtered.length > 0}>
        No anomalies match the selected filters.
      </div>
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

  private async handleAnomalyChartClick(e: MouseEvent, anomaly: Anomaly) {
    if (e.type === 'click' && e.button !== 0) return;
    if (e.type === 'auxclick' && e.button !== 1) return;
    e.preventDefault();
    const newTab = window.open('', '_blank');
    if (newTab) {
      newTab.document.write('<title>Loading...</title>Loading graph...');
      newTab.document.close();
    }

    this.loadingGraphForAnomaly.set(anomaly.id, true);
    this.requestUpdate();

    await this.reportNavigationController.openMultiGraphUrl(anomaly, newTab, this.isDryRun);

    this.loadingGraphForAnomaly.set(anomaly.id, false);
    this.requestUpdate();
  }

  private async handleGroupChartClick(e: MouseEvent, anomalyGroup: AnomalyGroup) {
    if (e.type === 'click' && e.button !== 0) return;
    if (e.type === 'auxclick' && e.button !== 1) return;
    e.preventDefault();
    const newTab = window.open('', '_blank');
    if (newTab) {
      newTab.document.write('<title>Loading...</title>Loading graph...');
      newTab.document.close();
    }
    await this.reportNavigationController.openReportForAnomalyIds(anomalyGroup.anomalies, newTab);
  }

  async openAnomalyGroupReportPage() {
    const reportPromises = this.groupingController.groups
      .filter((group) => group.anomalies.some((a) => this.selectionController.has(a)))
      .map((group) => this.reportNavigationController.openReportForAnomalyIds(group.anomalies));

    const results = await Promise.all(reportPromises);

    if (results.some((success) => !success)) {
      errorMessage('Popups blocked. Allow them (in the address bar) and retry', 0, {
        countMetricSource: CountMetric.FrontendErrorReported,
        source: ANOMALIES_TABLE_SOURCE,
        errorCode: StatusCodes.OK.toString(),
      });
    }
  }

  togglePopup() {
    this.showPopup = !this.showPopup;
  }

  private isGroupInitiallyRequested(group: AnomalyGroup): boolean {
    return group.anomalies.some((anomaly) => this.initiallyRequestedAnomalyIDs.has(anomaly.id));
  }

  private generateGroups() {
    const sortedGroups = this.sortGroups(this.groupingController.groups);

    if (!this.show_requested_groups_first || this.initiallyRequestedAnomalyIDs.size === 0) {
      return sortedGroups.map((group) => this.generateRows(group));
    }

    const requestedGroups: AnomalyGroup[] = [];
    const otherGroups: AnomalyGroup[] = [];

    for (const group of sortedGroups) {
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
  }

  private generateRows(anomalyGroup: AnomalyGroup, rowClass: string = ''): TemplateResult[] {
    const rows: TemplateResult[] | never = [];
    const length = anomalyGroup.anomalies.length;
    this.calculateSummaryForGroup(anomalyGroup);

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
                    @click=${(e: MouseEvent) => this.handleAnomalyChartClick(e, anomaly)}
                    @auxclick=${(e: MouseEvent) => this.handleAnomalyChartClick(e, anomaly)}>
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

  private calculateSummaryForGroup(anomalyGroup: AnomalyGroup) {
    if (!anomalyGroup.anomalies || anomalyGroup.anomalies.length === 0) {
      return;
    }
    if (anomalyGroup.summaryData.calculated) {
      return;
    }

    const [deltaValue, isSummaryRegression] =
      AnomalyTransformer.determineSummaryDelta(anomalyGroup);

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

    const anomalyForBugReportLink = this.getReportLinkForSummaryRowBugId(anomalyGroup);
    const bugIdForLink = anomalyForBugReportLink ? anomalyForBugReportLink.bug_id : 0;

    const summaryData: SummaryData = {
      startRevision: minStartRevision,
      endRevision: maxEndRevision,
      bot: allSameBot ? firstProcessed.bot : '*',
      testsuite: allSameTestSuite ? firstProcessed.testsuite : '*',
      test: AnomalyTransformer.findLongestSubTestPath(anomalyGroup.anomalies),
      delta: deltaValue,
      isSummaryRegression: isSummaryRegression,
      bug: bugIdForLink,
      calculated: true,
    };

    anomalyGroup.summaryData = summaryData;
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

    const firstAnomaly = anomalyGroup.anomalies[0];
    const summaryBugs = this.getSummaryBugs(anomalyGroup.anomalies);

    const summaryData = anomalyGroup.summaryData;
    const summaryClass = summaryData.isSummaryRegression ? 'regression' : 'improvement';
    const anomalyForBugReportLink = this.getReportLinkForSummaryRowBugId(anomalyGroup);

    return html`
      <tr
        role="button"
        tabindex="0"
        aria-expanded="${anomalyGroup.expanded}"
        @click=${() => this.expandGroup(anomalyGroup)}
        @keydown=${(e: KeyboardEvent) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            this.expandGroup(anomalyGroup);
          }
        }}
        data-bugid="${summaryData.bug}"
        data-revisions="${summaryData.endRevision}"
        data-bot="${summaryData.bot}"
        data-testsuite="${summaryData.testsuite}"
        data-test="${summaryData.test}"
        data-delta="${summaryData.delta}"
        class="${this.getRowClass(0, anomalyGroup)} ${rowClass} summary-row">
        <td>
          <span
            class="expand-indicator"
            title="${regressions} regressions, ${improvements} improvements"
            ?hidden=${anomalyGroup.anomalies.length === 1}>
            <expand-less-icon-sk ?hidden=${!anomalyGroup.expanded}></expand-less-icon-sk>
            <expand-more-icon-sk ?hidden=${anomalyGroup.expanded}></expand-more-icon-sk>
            <span class="regression">${regressions}</span> |
            <span class="improvement">${improvements}</span>
          </span>
        </td>
        <td>
          <label>
            <input
              type="checkbox"
              @click=${(e: Event) => e.stopPropagation()}
              @change="${(e: Event) => {
                e.stopPropagation();
                this.toggleChildrenCheckboxes(anomalyGroup);
              }}"
              .checked=${isChecked}
              .indeterminate=${isIndeterminate}
              id="anomaly-row-${this.uniqueId}-${this.getGroupId(anomalyGroup)}" />
          </label>
        </td>
        <td class="center-content">
          <button
            class="trendingicon-link"
            @click=${(e: MouseEvent) => {
              e.stopPropagation();
              this.handleGroupChartClick(e, anomalyGroup);
            }}
            @auxclick=${(e: MouseEvent) => {
              e.stopPropagation();
              this.handleGroupChartClick(e, anomalyGroup);
            }}>
            <trending-up-icon-sk></trending-up-icon-sk>
          </button>
        </td>
        <td class="tooltip-cell">
          ${this.getReportLinkForBugId(summaryData.bug)}
          <close-icon-sk
            id="btnUnassociate"
            @click=${(e: Event) => {
              e.stopPropagation();
              this.triageMenu!.makeEditAnomalyRequest(
                [anomalyForBugReportLink || firstAnomaly],
                [],
                'RESET'
              );
            }}
            ?hidden=${!anomalyForBugReportLink}>
          </close-icon-sk>
          <bug-tooltip-sk
            @click=${(e: Event) => e.stopPropagation()}
            .bugs=${summaryBugs || []}
            totalLabel="distinct total"></bug-tooltip-sk>
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
    return html`<a
      href="http://b/${bug_id}"
      target="_blank"
      @click=${(e: Event) => e.stopPropagation()}
      >${bug_id}</a
    >`;
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

  /**
   * Populates the table with the provided list of anomalies.
   * Deprecated: Use .anomalyList property instead.
   */
  async populateTable(anomalyList: Anomaly[]): Promise<void> {
    this.anomalyList = anomalyList;
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
  toggleAllCheckboxes(forceChecked?: boolean) {
    const headerCheckbox = this.querySelector(
      `#header-checkbox-${this.uniqueId}`
    ) as HTMLInputElement;

    const filtered = this.filteredAnomalies;
    const totalCount = filtered.length;
    const selectedCount = this.selectionController.size;
    const isAllSelected = totalCount > 0 && selectedCount === totalCount;

    let checked = false;
    if (forceChecked !== undefined) {
      checked = forceChecked;
    } else if (headerCheckbox && headerCheckbox.checked !== isAllSelected) {
      checked = headerCheckbox.checked;
    } else {
      checked = !isAllSelected;
    }

    if (checked) {
      filtered.forEach((a) => this.selectionController.select(a));
    } else {
      filtered.forEach((a) => this.selectionController.deselect(a));
    }

    this.dispatchEvent(
      new CustomEvent('anomalies_checked', {
        detail: {
          anomalies: filtered,
          checked: checked,
        },
        bubbles: true,
      })
    );

    this.requestUpdate();
  }

  public async openMultiGraphUrl(anomaly: Anomaly, newTab: Window | null, isDryRun = false) {
    await this.reportNavigationController.openMultiGraphUrl(
      anomaly,
      newTab,
      isDryRun || this.isDryRun
    );
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
