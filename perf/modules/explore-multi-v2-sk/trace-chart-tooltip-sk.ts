import { LitElement, css, html } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { TraceSeries, TraceRow } from './trace-types';
import { Regression, CommitNumber } from '../json';
import { fromKey } from '../paramtools';
import { formatBug, formatNumber, formatPercentage, getPercentChange } from '../common/anomaly';
import '../commit-range-sk/commit-range-sk';
import '../json-source-sk/json-source-sk';
import '../triage-menu-sk/triage-menu-sk';
import { NudgeEntry } from '../triage-menu-sk/triage-menu-sk';
import '../point-links-sk/point-links-sk';
import '../user-issue-sk/user-issue-sk';
import '../bisect-dialog-sk/bisect-dialog-sk';
import '../pinpoint-try-job-dialog-sk/pinpoint-try-job-dialog-sk';
import { AnomalyData } from '../common/anomaly-data';
import '../../../elements-sk/modules/icons/close-icon-sk';
import { makeEditAnomalyRequest } from '../anomalies-table-sk/triage-api';
import { lookupCids } from '../cid/cid';

@customElement('trace-chart-tooltip-sk')
export class TraceChartTooltipSk extends LitElement {
  @property({ type: Object }) hoveredPoint: {
    series: TraceSeries;
    row: TraceRow;
    x: number;
    y: number;
  } | null = null;

  @property({ type: Boolean }) dateMode = false;

  @property({ type: String }) yAxisLabel = 'score';

  @property({ type: String }) bug_host_url = window.perf ? window.perf.bug_host_url : '';

  @property({ type: Object }) regressions: {
    [trace_id: string]: { [commit: number]: Regression };
  } = {};

  @property({ type: Object }) diffNamesMap: Map<string, string> = new Map();

  @property({ type: Boolean }) tooltipDiffs = false;

  @property({ type: Array }) processedSeries: TraceSeries[] = [];

  @property({ type: Boolean }) showBisectButton = false;

  @property({ type: Boolean }) showPinpointButtons = false;

  @property({ type: Number }) canvasWidth = 0;

  @property({ type: Number }) canvasHeight = 0;

  @state()
  private _lastLoadedPoint: { commit: number; traceName: string } | null = null;

  @state()
  private _debouncedTraceKey: string = '';

  @state()
  private _debouncedCommitPosition: number = 0;

  @property({ type: String }) user_id = '';

  @state()
  private _tooltipHashes: string[] | null = null;

  @state()
  private _linksLoading = false;

  @state()
  private _nudgeList: NudgeEntry[] | null = null;

  private _linksTimeout: number | null = null;

  private _debounceTimer: number | null = null;

  private _handlers: { [key: string]: (e: Event) => void } = {};

  connectedCallback() {
    super.connectedCallback();
    const stopPropagation = (e: Event) => e.stopPropagation();
    const events = [
      'pointerdown',
      'pointermove',
      'pointerup',
      'click',
      'dblclick',
      'wheel',
      'mousedown',
      'mouseup',
    ];
    events.forEach((event) => {
      this._handlers[event] = stopPropagation;
      this.addEventListener(event, stopPropagation);
    });
  }

  disconnectedCallback() {
    Object.keys(this._handlers).forEach((event) => {
      this.removeEventListener(event, this._handlers[event]);
    });
    super.disconnectedCallback();
  }

  static styles = css`
    .hover-tooltip {
      position: absolute;
      background: var(--surface);
      backdrop-filter: blur(8px);
      border: 1px solid color-mix(in srgb, var(--on-surface) 10%, transparent);
      color: var(--on-surface);
      padding: 12px 16px;
      border-radius: 8px;
      font-size: 12px;
      pointer-events: auto;
      z-index: 10;
      box-shadow: 0 10px 25px -5px var(--transparent-overlay);
      max-width: 320px;
      font-family: 'JetBrains Mono', monospace;
    }

    .hover-tooltip a {
      color: var(--primary);
      text-decoration: none;
    }

    .hover-tooltip a:hover {
      text-decoration: underline;
      color: var(--primary-variant);
    }

    .tooltip-row {
      margin-bottom: 4px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      display: flex;
      gap: 8px;
    }

    .tooltip-row strong {
      color: var(--on-surface);
      font-weight: 600;
      min-width: 60px;
      opacity: 0.6;
    }

    .tooltip-section {
      margin-bottom: 8px;
      padding-bottom: 8px;
      border-bottom: 1px solid color-mix(in srgb, var(--on-surface) 10%, transparent);
    }

    .tooltip-section:last-child {
      border-bottom: none;
    }

    .color-indicator {
      position: absolute;
      left: 0;
      top: 0;
      bottom: 0;
      width: 4px;
      border-top-left-radius: 8px;
      border-bottom-left-radius: 8px;
    }

    .truncate {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      min-width: 0;
    }

    .wrap {
      white-space: normal;
      overflow-wrap: break-word;
    }

    .regression {
      color: var(--negative);
    }

    .improvement {
      color: var(--positive);
    }

    .inline-buttons {
      display: inline-block;
      margin-left: 4px;
    }

    .diff-section {
      margin-bottom: 8px;
      font-size: 13px;
      border-top: 1px solid color-mix(in srgb, var(--on-surface) 10%, transparent);
      padding-top: 6px;
    }

    .diff-title {
      color: var(--on-surface-variant);
      margin-bottom: 4px;
      font-weight: 500;
    }

    .diff-row {
      display: flex;
      align-items: center;
      margin-bottom: 2px;
      justify-content: space-between;
    }

    .diff-trace-info {
      display: flex;
      align-items: center;
      overflow: hidden;
      margin-right: 8px;
    }

    .diff-color-swatch {
      width: 8px;
      height: 8px;
      border-radius: 2px;
      margin-right: 6px;
      flex-shrink: 0;
    }

    .diff-trace-name {
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .diff-values {
      display: flex;
      align-items: center;
    }

    .diff-percent {
      font-weight: 500;
      flex-shrink: 0;
    }

    .diff-distance {
      color: var(--on-surface-variant);
      font-size: 11px;
      margin-left: 6px;
      white-space: nowrap;
    }

    .skeleton {
      display: inline-block;
      background: var(--outline);
      border-radius: 4px;
      animation: pulse 1.5s infinite;
    }

    @keyframes pulse {
      0% {
        opacity: 0.6;
      }

      50% {
        opacity: 0.3;
      }

      100% {
        opacity: 0.6;
      }
    }

    triage-menu-sk button {
      height: var(--perf-button-height);
      min-width: var(--perf-button-min-width);
      padding: var(--perf-button-padding, 0 8px);
      font-size: var(--perf-button-font-size);
      text-transform: none;
      border-radius: var(--perf-button-border-radius);
      border: solid 1px var(--outline);
      background: var(--surface);
      color: var(--primary);
      font-family: var(--font);
      cursor: pointer;
      margin: var(--perf-button-margin);
      display: inline-flex;
      align-items: center;
      justify-content: center;
      box-sizing: border-box;
      box-shadow: none;
      text-decoration: none;
      line-height: normal;
    }

    triage-menu-sk button:hover {
      background-color: var(--surface-1dp);
    }

    toast-sk {
      display: flex;
      visibility: hidden;
      position: fixed;
      left: auto;
      bottom: 0;
      color: var(--on-background);
      background: var(--background);
      fill: var(--on-background);
      padding: 10px 15px;
      opacity: 0;
      z-index: 20;
    }

    toast-sk[shown] {
      visibility: visible;
      opacity: 1;
      bottom: 30px;
      color: var(--on-background);
      background: var(--background);
      fill: var(--background);
      width: auto;
      max-width: 1000px;
    }

    triage-menu-sk #ignore_toast {
      background-color: var(--background);
      color: var(--on-background);
    }

    close-icon-sk#unassociate-bug-button {
      fill: var(--negative);
      cursor: pointer;
      height: 18px;
      width: 18px;
      margin-left: 4px;
      vertical-align: middle;
    }
  `;

  private _formatDate(ts: number): string {
    if (!ts || ts === Infinity || ts === -Infinity) return '';
    const date = ts > 1e11 ? new Date(ts) : new Date(ts * 1000);
    return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(
      date.getDate()
    ).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(
      date.getMinutes()
    ).padStart(2, '0')}`;
  }

  private _formatYValue(val: number): string {
    if (val === undefined || val === null || isNaN(val)) {
      return 'N/A';
    }
    const label = this.yAxisLabel.toLowerCase();

    if (label.includes('bytes') || label.includes('sizeinbytes')) {
      const absVal = Math.abs(val);
      if (absVal >= 1024 * 1024 * 1024) return (val / (1024 * 1024 * 1024)).toFixed(2) + ' GB';
      if (absVal >= 1024 * 1024) return (val / (1024 * 1024)).toFixed(2) + ' MB';
      if (absVal >= 1024) return (val / 1024).toFixed(2) + ' KB';
      return val.toFixed(0) + ' B';
    }

    if (label.includes('bytespersecond')) {
      const absVal = Math.abs(val);
      if (absVal >= 1024 * 1024 * 1024) return (val / (1024 * 1024 * 1024)).toFixed(2) + ' GB/s';
      if (absVal >= 1024 * 1024) return (val / (1024 * 1024)).toFixed(2) + ' MB/s';
      if (absVal >= 1024) return (val / 1024).toFixed(2) + ' KB/s';
      return val.toFixed(0) + ' B/s';
    }

    if (label.includes('n%') || label.includes('%')) {
      return val.toFixed(2) + '%';
    }

    if (label.includes('ms')) {
      const absVal = Math.abs(val);
      if (absVal >= 1000) return (val / 1000).toFixed(2) + ' s';
      return val.toFixed(2) + ' ms';
    }

    if (label.includes('ns')) {
      const absVal = Math.abs(val);
      if (absVal >= 1e6) return (val / 1e6).toFixed(2) + ' ms';
      if (absVal >= 1e3) return (val / 1e3).toFixed(2) + ' µs';
      return val.toFixed(0) + ' ns';
    }

    if (Math.abs(val) >= 1000000) return val.toExponential(2);
    if (Math.abs(val) < 0.01 && val !== 0) return val.toExponential(2);
    return val.toFixed(2);
  }

  private _formatTooltipValue(val: number): string {
    if (val === undefined || val === null || isNaN(val)) {
      return 'N/A';
    }
    const formatted = this._formatYValue(val);
    const rawStr = val.toFixed(4);
    const rawStrStripped = rawStr.replace(/\.?0+$/, '');

    if (!formatted.includes(rawStrStripped)) {
      return `${formatted} (${rawStr})`;
    }
    return formatted;
  }

  private _anomalyChange(regression: Regression) {
    const change = formatPercentage(
      getPercentChange(regression.median_before, regression.median_after)
    );
    const divClass = regression.is_improvement ? 'improvement' : 'regression';
    return html`<span class="${divClass}">${change}</span>`;
  }

  updated(changedProperties: Map<string | number | symbol, unknown>) {
    if (changedProperties.has('hoveredPoint')) {
      if (this._debounceTimer) {
        window.clearTimeout(this._debounceTimer);
        this._debounceTimer = null;
      }

      if (this.hoveredPoint) {
        this._debounceTimer = window.setTimeout(() => {
          this._loadTooltipData();
        }, 150);
      }
    }
  }

  private _loadTooltipData() {
    if (!this.hoveredPoint) return;
    const commit = this.hoveredPoint.row.commit_number;
    const traceName = this.hoveredPoint.series.id;

    if (
      !this._lastLoadedPoint ||
      this._lastLoadedPoint.commit !== commit ||
      this._lastLoadedPoint.traceName !== traceName
    ) {
      this._lastLoadedPoint = { commit, traceName };
      this._debouncedTraceKey = this.hoveredPoint.series.originalId || traceName;
      this._debouncedCommitPosition = commit;

      const s = this.hoveredPoint.series;
      const i = s.rows.findIndex((r) => r.commit_number === commit);
      const prevCommit =
        i > 0 && s.rows[i - 1].commit_number > 0 ? s.rows[i - 1].commit_number : commit;
      const prevRow = i > 0 ? s.rows[i - 1] : null;

      if (
        !this.hoveredPoint.row.author ||
        !this.hoveredPoint.row.message ||
        !this.hoveredPoint.row.hash ||
        (prevRow && !prevRow.hash)
      ) {
        lookupCids([CommitNumber(prevCommit), CommitNumber(commit)])
          .then((resp) => {
            if (resp && resp.commitSlice) {
              resp.commitSlice.forEach((c) => {
                if (c.offset === commit) {
                  this.hoveredPoint!.row.author = c.author;
                  this.hoveredPoint!.row.message = c.message;
                  this.hoveredPoint!.row.hash = c.hash;
                  this.hoveredPoint!.row.url = c.url;
                }
                if (prevRow && c.offset === prevCommit) {
                  prevRow.author = c.author;
                  prevRow.message = c.message;
                  prevRow.hash = c.hash;
                  prevRow.url = c.url;
                }
              });
              if (this.hoveredPoint && this.hoveredPoint.row.commit_number === commit) {
                this._lastLoadedPoint = null;
                this.requestUpdate();
              }
            }
          })
          .catch(console.error);
      }

      const pointLinks = this.shadowRoot?.querySelector('#tooltip-point-links') as any;
      if (pointLinks) {
        const metadata = this.hoveredPoint.row.metadata;
        window.clearTimeout(this._linksTimeout!);
        if (metadata && Object.keys(metadata).length > 0) {
          pointLinks.displayUrls = metadata;
          pointLinks.displayTexts = {};
          pointLinks.commitPosition = commit;
        } else {
          this._linksLoading = true;
          this._linksTimeout = window.setTimeout(() => {
            if (!this.hoveredPoint) return;
            const cleanTraceName = this.hoveredPoint.series.originalId || traceName;
            const prevCommitPos = i > 0 ? s.rows[i - 1].commit_number : null;

            pointLinks
              .load(
                commit,
                prevCommitPos,
                cleanTraceName,
                (window as any).perf?.keys_for_commit_range || [
                  'V8',
                  'WebRTC',
                  'V8 Git Hash',
                  'WebRTC Git Hash',
                ],
                (window as any).perf?.keys_for_useful_links || [
                  'Build Page',
                  'Tracing uri',
                  'Browser Version',
                  'Workflow',
                  'Swarming Tasks',
                ],
                []
              )
              .then(() => {
                this._linksLoading = false;
              })
              .catch((err: any) => {
                console.error(err);
                this._linksLoading = false;
              });
          }, 100);
        }
      }

      const commitRangeSk = this.shadowRoot?.querySelector('#tooltip-commit-range-link') as any;
      if (commitRangeSk) {
        const hashes: string[] = [];
        if (prevRow?.hash) {
          hashes.push(prevRow.hash);
        }
        if (this.hoveredPoint?.row.hash) {
          hashes.push(this.hoveredPoint.row.hash);
        }
        console.log('DEBUG hashes length:', hashes.length, hashes);

        console.log('DEBUG prevCommit:', prevCommit, 'commit:', commit);
        commitRangeSk.commitIndex = -1;
        commitRangeSk['_commitIds'] = [prevCommit, commit];
        commitRangeSk['updateText']();

        if (hashes.length === 2) {
          this._tooltipHashes = hashes;
          commitRangeSk.hashes = hashes;
          commitRangeSk.autoload = false;
          commitRangeSk.recalcLink();
        } else {
          console.log('DEBUG falling back to autoload');
          this._tooltipHashes = null;
          commitRangeSk.autoload = true;
          commitRangeSk.recalcLink();
        }
      }

      const regression = this.regressions[traceName]?.[commit];
      if (regression) {
        const anomalyRangeSk = this.shadowRoot?.querySelector('#anomaly-commit-range-link') as any;
        if (anomalyRangeSk) {
          const prev_commit = (regression as any).start_revision - 1;
          const end_commit = (regression as any).end_revision;
          anomalyRangeSk.setRange(prev_commit, end_commit);
        }

        const anomalyData: AnomalyData = {
          anomaly: regression as any,
          x: i,
          y: this.hoveredPoint.row.val,
          highlight: false,
        };
        this._nudgeList = this.calculateNudgeListSparse(
          s.rows,
          i,
          anomalyData,
          2,
          0,
          !(window as any).perf?.fetch_anomalies_from_sql
        );
      } else {
        this._nudgeList = null;
      }
    }
  }

  private calculateNudgeListSparse(
    rows: TraceRow[],
    currentIndex: number,
    anomalyData: AnomalyData,
    nudgeRange: number = 2,
    xOffset: number = 0,
    isLegacy: boolean = true
  ): NudgeEntry[] {
    const nudgeList: NudgeEntry[] = [];
    const length = rows.length;

    if (currentIndex < 0 || currentIndex >= length) {
      return nudgeList;
    }

    for (let i = -nudgeRange; i <= nudgeRange; i++) {
      const targetIndex = currentIndex + i;
      if (targetIndex >= 0 && targetIndex < length) {
        const prevValidIndex = targetIndex > 0 ? targetIndex - 1 : null;

        let start_revision = rows[targetIndex].commit_number;
        if (isLegacy && prevValidIndex !== null) {
          start_revision = rows[prevValidIndex].commit_number + 1;
        } else if (!isLegacy) {
          start_revision = anomalyData.anomaly.start_revision;
        }

        nudgeList.push({
          display_index: i,
          anomaly_data: anomalyData,
          selected: i === 0,
          start_revision: start_revision,
          end_revision: rows[targetIndex].commit_number,
          display_commit_number: rows[targetIndex].commit_number,
          x: targetIndex - xOffset,
          y: rows[targetIndex].val,
        });
      }
    }
    return nudgeList;
  }

  private getLastSubtest(d: any) {
    return (
      d.subtest_7 ||
      d.subtest_6 ||
      d.subtest_5 ||
      d.subtest_4 ||
      d.subtest_3 ||
      d.subtest_2 ||
      d.subtest_1 ||
      ''
    );
  }

  private constructTestPath(traceName: string): { testPath: string; story: string } {
    const params = fromKey(traceName);
    const story = this.getLastSubtest(params);
    const parts: string[] = [];
    if (params.master) parts.push(params.master);
    if (params.bot) parts.push(params.bot);
    if (params.benchmark) parts.push(params.benchmark);
    if (params.test) parts.push(params.test);

    if (params.subtest_1 && params.subtest_1 !== story) parts.push(params.subtest_1);
    if (params.subtest_2 && params.subtest_2 !== story) parts.push(params.subtest_2);
    if (params.subtest_3 && params.subtest_3 !== story) parts.push(params.subtest_3);
    if (params.subtest_4 && params.subtest_4 !== story) parts.push(params.subtest_4);

    parts.push(story);
    return { testPath: parts.join('/'), story };
  }

  private openBisectDialog() {
    if (!this.hoveredPoint) return;
    const s = this.hoveredPoint.series;
    const r = this.hoveredPoint.row;
    const traceName = s.id;

    const { testPath, story } = this.constructTestPath(traceName);
    const i = s.rows.indexOf(r);
    const prevCommit = i > 0 ? s.rows[i - 1].commit_number : r.commit_number;

    const regression = this.regressions[s.id]?.[r.commit_number];
    const bugId = (regression as any)?.bug_id ? String((regression as any).bug_id) : '';
    const anomalyId = regression ? String((regression as any).id) : '';

    const preloadBisectInputs = {
      testPath: testPath,
      startCommit: String(prevCommit),
      endCommit: String(r.commit_number),
      bugId: bugId,
      anomalyId: anomalyId,
      story: story,
    };

    const bisectDialog = this.shadowRoot?.querySelector('#bisect-dialog-sk') as any;
    if (bisectDialog) {
      bisectDialog.setBisectInputParams(preloadBisectInputs);
      bisectDialog.open();
    }
  }

  private openTryJobDialog() {
    if (!this.hoveredPoint) return;
    const s = this.hoveredPoint.series;
    const r = this.hoveredPoint.row;
    const traceName = s.id;

    const { testPath, story } = this.constructTestPath(traceName);
    const i = s.rows.indexOf(r);
    const prevCommit = i > 0 ? s.rows[i - 1].commit_number : r.commit_number;

    const params = fromKey(traceName);
    const preloadTryJobInputs = {
      testPath: testPath,
      baseCommit: String(prevCommit),
      endCommit: String(r.commit_number),
      story: story,
      configuration: params.bot || '',
      benchmark: params.benchmark || '',
    };

    const tryJobDialog = this.shadowRoot?.querySelector('#pinpoint-try-job-dialog-sk') as any;
    if (tryJobDialog) {
      tryJobDialog.setTryJobInputParams(preloadTryJobInputs);
      tryJobDialog.open();
    }
  }

  private unassociateBug(traceName: string, regression: Regression): void {
    makeEditAnomalyRequest([regression as any], [traceName], 'RESET').then(() => {
      this.dispatchEvent(
        new CustomEvent('anomaly-changed', {
          bubbles: true,
          composed: true,
          detail: {
            traceNames: [traceName],
            editAction: 'RESET',
            anomalies: [regression],
          },
        })
      );
      this.requestUpdate();
    });
  }

  private _xAccessor(r: TraceRow) {
    return this.dateMode ? r.createdat : r.commit_number;
  }

  render() {
    if (!this.hoveredPoint) return html``;

    const s = this.hoveredPoint.series;
    const r = this.hoveredPoint.row;

    return html`
      <div
        class="hover-tooltip"
        style="${this.hoveredPoint.y > this.canvasHeight / 2
          ? `bottom: ${this.canvasHeight - this.hoveredPoint.y + 10}px;`
          : `top: ${this.hoveredPoint.y + 10}px;`} ${this.canvasWidth > 0 &&
        this.hoveredPoint.x > this.canvasWidth / 2
          ? `right: ${this.canvasWidth - this.hoveredPoint.x + 10}px;`
          : `left: ${this.hoveredPoint.x + 10}px;`}">
        <div class="color-indicator" style="background-color: ${s.color};"></div>
        <!-- Section 1: Point Info -->
        <div class="tooltip-section">
          <div class="tooltip-row">
            <strong>Series:</strong>${this.diffNamesMap.get(s.id) || s.id}
          </div>
          ${r.createdat
            ? html`
                <div class="tooltip-row">
                  <strong>Date:</strong>${this._formatDate(r.createdat)}
                </div>
              `
            : ''}
          <div class="tooltip-row"><strong>Commit Number:</strong>${r.commit_number}</div>
          <div class="tooltip-row"><strong>Value:</strong>${this._formatTooltipValue(r.val)}</div>
        </div>

        <!-- Section 2: Commit Info -->
        <div class="tooltip-section">
          <div class="tooltip-row">
            <strong>Author:</strong>
            ${r.author
              ? html`<span class="truncate">${r.author.split('(')[0]}</span>`
              : html`<span class="skeleton" style="width: 80px; height: 12px;"></span>`}
          </div>
          <div class="tooltip-row">
            <strong>Message:</strong>
            ${r.message
              ? html`<span class="wrap">${r.message}</span>`
              : html`<span class="skeleton" style="width: 180px; height: 12px;"></span>`}
          </div>
          ${r.hash
            ? html`
                <div class="tooltip-row">
                  <strong>Commit:</strong>
                  <a href="${r.url}" target="_blank"> ${r.hash.substring(0, 8)} </a>
                </div>
              `
            : ''}
        </div>
        ${(() => {
          const regression = this.regressions[s.id]?.[r.commit_number];
          if (regression) {
            return html`
              <div class="tooltip-row">
                <strong>Anomaly:</strong> ${regression.is_improvement
                  ? 'Improvement'
                  : 'Regression'}
              </div>
              <div class="tooltip-row">
                <strong>Anomaly Range:</strong>
                <commit-range-sk id="anomaly-commit-range-link"></commit-range-sk>
              </div>
              <div class="tooltip-row">
                <strong>Median:</strong> ${formatNumber(regression.median_after)}
              </div>
              <div class="tooltip-row">
                <strong>Previous:</strong> ${formatNumber(regression.median_before)}
                [${this._anomalyChange(regression)}%]
              </div>
              ${(regression as any).multiplicity
                ? html`
                    <div class="tooltip-row">
                      <strong>Multiplicity:</strong> ${(regression as any).multiplicity}
                    </div>
                  `
                : ''}
              ${(regression as any).is_legacy
                ? html`
                    <div class="tooltip-row">
                      <strong>Legacy anomaly</strong>
                    </div>
                  `
                : ''}
              ${(regression as any).bisect_ids && (regression as any).bisect_ids.length > 0
                ? html`
                    <div class="tooltip-row">
                      <strong>Pinpoint:</strong>
                      <span>
                        ${(regression as any).bisect_ids.map(
                          (id: string, index: number) =>
                            html`<a
                                href="https://pinpoint-dot-chromeperf.appspot.com/job/${id}"
                                target="_blank"
                                >${id}</a
                              >${index < (regression as any).bisect_ids.length - 1
                                ? html`<br />`
                                : ''}`
                        )}
                      </span>
                    </div>
                  `
                : ''}
              ${(regression as any).bug_id
                ? html`
                    <div class="tooltip-row">
                      <strong>Bug ID:</strong>
                      <span> ${formatBug(this.bug_host_url, (regression as any).bug_id)} </span>
                      <close-icon-sk
                        id="unassociate-bug-button"
                        @click=${() => this.unassociateBug(s.originalId || s.id, regression)}>
                      </close-icon-sk>
                    </div>
                  `
                : ''}
              <div class="tooltip-row">
                <triage-menu-sk
                  id="tooltip-triage-menu"
                  .anomalies=${[regression]}
                  .traceNames=${[s.originalId || s.id]}
                  .nudgeList=${this._nudgeList}
                  ?hidden=${(regression as any).bug_id !== undefined &&
                  (regression as any).bug_id !== null &&
                  (regression as any).bug_id !== 0}>
                </triage-menu-sk>
              </div>
            `;
          }
          return '';
        })()}
        ${this.showPinpointButtons || this.showBisectButton
          ? html`
              <div class="tooltip-row">
                <strong>Pinpoint:</strong>
                <div class="buttons inline-buttons">
                  <button
                    id="bisect"
                    @click=${this.openBisectDialog}
                    ?hidden=${!this.showBisectButton}>
                    Bisect
                  </button>
                  <button
                    id="try-job"
                    @click=${this.openTryJobDialog}
                    ?hidden=${!this.showPinpointButtons}>
                    Request Trace
                  </button>
                </div>
              </div>
            `
          : ''}
        ${(() => {
          const regression = this.regressions[s.id]?.[r.commit_number];
          if (!regression) {
            return html`
              <div class="tooltip-row">
                <strong>User Issues:</strong>
                <div class="buttons inline-buttons">
                  <user-issue-sk
                    id="tooltip-user-issue-sk"
                    .bug_id=${0}
                    .trace_key=${this._debouncedTraceKey}
                    .commit_position=${this._debouncedCommitPosition}
                    .user_id=${this.user_id}>
                  </user-issue-sk>
                </div>
              </div>
            `;
          }
          return '';
        })()}
        <div class="tooltip-row">
          <strong>Point Range:</strong>
          <commit-range-sk id="tooltip-commit-range-link" .hashes=${this._tooltipHashes}>
          </commit-range-sk>
        </div>
        <div class="tooltip-row">
          <point-links-sk id="tooltip-point-links"></point-links-sk>
          ${this._linksLoading
            ? html`
                <div
                  class="subrepo-skeleton"
                  style="border-top: 1px solid rgba(255,255,255,0.1); padding-top: 8px; margin-top: 8px; width: 100%;">
                  <div class="tooltip-row">
                    <strong>V8:</strong>
                    <span class="skeleton" style="width: 120px; height: 12px;"></span>
                  </div>
                  <div class="tooltip-row">
                    <strong>WebRTC:</strong>
                    <span class="skeleton" style="width: 120px; height: 12px;"></span>
                  </div>
                </div>
              `
            : ''}
        </div>
        ${(window as any).perf?.show_json_file_display
          ? html`
              <div class="tooltip-row">
                <json-source-sk
                  id="json-source-sk"
                  .cid=${r.commit_number}
                  .traceid=${s.originalId || s.id}>
                </json-source-sk>
              </div>
            `
          : ''}
        ${this.tooltipDiffs && this.processedSeries.length > 1
          ? (() => {
              const targetX = this._xAccessor(r);
              return html`
                <div class="diff-section">
                  <div class="diff-title">Diff vs other traces:</div>
                  ${this.processedSeries.map((os) => {
                    if (os.id === s.id) return '';

                    let otherVal: number | null = null;
                    let commitDistance = 0;
                    let exactRow = null;
                    let minDist = Infinity;

                    for (const or of os.rows) {
                      const dist = Math.abs(this._xAccessor(or) - targetX);
                      if (dist < minDist) {
                        minDist = dist;
                        exactRow = or;
                      } else if (this._xAccessor(or) > targetX) {
                        break;
                      }
                    }

                    if (exactRow) {
                      otherVal = exactRow.val;
                    } else {
                      let closest: any = null;
                      let minDiff = Infinity;
                      for (const or of os.rows) {
                        const diff = Math.abs(this._xAccessor(or) - targetX);
                        if (diff < minDiff) {
                          minDiff = diff;
                          closest = or;
                        }
                      }
                      if (closest) {
                        otherVal = closest.val;
                        commitDistance = minDiff;
                      }
                    }

                    if (otherVal !== null && otherVal !== 0) {
                      const myVal = r.val;
                      const diffY = myVal - otherVal;
                      const pctY = (diffY / Math.abs(otherVal)) * 100;
                      const color = diffY >= 0 ? '#4caf50' : '#e53935';

                      return html`
                        <div class="diff-row">
                          <div class="diff-trace-info">
                            <span
                              class="diff-color-swatch"
                              style="background-color: ${os.color};"></span>
                            <span
                              class="diff-trace-name"
                              title="${this.diffNamesMap.get(os.id) || os.id}">
                              ${this.diffNamesMap.get(os.id) || os.id}
                            </span>
                          </div>
                          <div class="diff-values">
                            <span class="diff-percent" style="color: ${color};">
                              ${diffY > 0 ? '+' : ''}${pctY.toFixed(1)}%
                            </span>
                            ${commitDistance > 0
                              ? html`
                                  <span class="diff-distance" title="${commitDistance} units away">
                                    (±${commitDistance})
                                  </span>
                                `
                              : ''}
                          </div>
                        </div>
                      `;
                    }
                    return '';
                  })}
                </div>
              `;
            })()
          : ''}
      </div>
      <bisect-dialog-sk id="bisect-dialog-sk"></bisect-dialog-sk>
      <pinpoint-try-job-dialog-sk id="pinpoint-try-job-dialog-sk"></pinpoint-try-job-dialog-sk>
    `;
  }
}
