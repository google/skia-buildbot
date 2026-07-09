/**
 * @module modules/chart-tooltip-sk
 * @description <h2><code>chart-tooltip-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { html, LitElement } from 'lit';
import { property, state, query, customElement } from 'lit/decorators.js';
import { createRef, ref } from 'lit/directives/ref.js';
import { Anomaly, ColumnHeader, CommitNumber } from '../json';
import { formatBug, formatNumber, formatPercentage, getPercentChange } from '../common/anomaly';
import '../commit-range-sk/commit-range-sk';
import { CommitRangeSk } from '../commit-range-sk/commit-range-sk';
import '../window/window';
import { TriageMenuSk, NudgeEntry } from '../triage-menu-sk/triage-menu-sk';
import '../triage-menu-sk/triage-menu-sk';
import '../user-issue-sk/user-issue-sk';
import '../bisect-dialog-sk/bisect-dialog-sk';
import '../pinpoint-try-job-dialog-sk/pinpoint-try-job-dialog-sk';
import { UserIssueSk } from '../user-issue-sk/user-issue-sk';
import '../../../elements-sk/modules/icons/close-icon-sk';
import '@material/web/elevation/elevation.js';
import { formatSpecialFunctions } from '../paramtools';
import '../point-links-sk/point-links-sk';
import { PointLinksSk, CommitLinks } from '../point-links-sk/point-links-sk';
import { BisectDialogSk, BisectPreloadParams } from '../bisect-dialog-sk/bisect-dialog-sk';
import {
  PinpointTryJobDialogSk,
  TryJobPreloadParams,
} from '../pinpoint-try-job-dialog-sk/pinpoint-try-job-dialog-sk';
import { defaultColors } from '../common/plot-builder';
import '../json-source-sk/json-source-sk';
import { JSONSourceSk } from '../json-source-sk/json-source-sk';
import { DEFAULT_OPTION_LABEL } from '../common/test-picker';
import { TrimHash } from '../common/commit';

@customElement('chart-tooltip-sk')
export class ChartTooltipSk extends LitElement {
  @property({ type: Number })
  index: number = -1;

  @property({ type: String })
  color: string = '';

  @property({ type: String })
  test_name: string = '';

  @state()
  private _trace_name: string = '';

  @property({ type: String })
  unit_type: string = '';

  @property({ type: Number })
  y_value: number = -1;

  @property({ attribute: false })
  date_value: Date = new Date();

  @property({ attribute: false })
  commit_position: CommitNumber | null = null;

  @property({ attribute: false })
  commit_info: ColumnHeader | null = null;

  @property({ attribute: false })
  anomaly: Anomaly | null = null;

  @state()
  private _nudgeList: NudgeEntry[] | null = null;

  @property({ type: String })
  bug_host_url: string = window.perf ? window.perf.bug_host_url : '';

  @property({ type: Number })
  bug_id: number = 0;

  @state()
  _show_pinpoint_buttons =
    window.perf?.git_repo_url?.includes('https://chromium.googlesource.com/chromium/src') || false;

  @state()
  show_bisect_button = !!window.perf?.show_bisect_btn;

  @query('#triage-menu')
  private triageMenu!: TriageMenuSk | null;

  @property({ type: Boolean })
  tooltip_fixed: boolean = false;

  @state()
  _is_range: boolean | null = null;

  @state()
  _close_button_action: () => void = () => {};

  @query('#tooltip-commit-range-link')
  commitRangeSk!: CommitRangeSk | null;

  @query('#tooltip-user-issue-sk')
  userIssueSk!: UserIssueSk | null;

  @state()
  private margin: { left?: number; right?: number; bottom?: number; top?: number } = {};

  private containerDiv = createRef<HTMLDivElement>();

  @query('#tooltip-point-links')
  pointLinks!: PointLinksSk | null;

  @query('#json-source-sk')
  jsonSourceDialog!: JSONSourceSk | null;

  @property({ type: Boolean })
  json_source: boolean = window.perf ? window.perf.show_json_file_display : false;

  @state()
  private _always_show_commit_info: boolean = window.perf
    ? window.perf.always_show_commit_info
    : false;

  @query('#bisect-dialog-sk')
  bisectDialog!: BisectDialogSk | null;

  @query('#pinpoint-try-job-dialog-sk')
  tryJobDialog!: PinpointTryJobDialogSk | null;

  protected createRenderRoot() {
    return this;
  }

  protected render() {
    return html`
      <div class="container" ${ref(this.containerDiv)}>
        <md-elevation></md-elevation>
        <button id="closeIcon" @click=${this._close_button_action} ?hidden=${!this.tooltip_fixed}>
          <close-icon-sk></close-icon-sk>
        </button>
        <h3>
          <span style="color:${this.color}">
            ${this.test_name || `${DEFAULT_OPTION_LABEL}`}
            <span ?hidden=${!this.anomaly}> [Anomaly] </span>
          </span>
        </h3>
        <ul class="table">
          <li>
            <span id="tooltip-key">Date</span>
            <span id="tooltip-text">${this.date_value.toUTCString()}</span>
          </li>
          <li>
            <span id="tooltip-key">Value</span>
            <span id="tooltip-text">${this.y_value} ${this.unit_type}</span>
          </li>
          <li>
            <span id="tooltip-key">Point Range</span>
            <commit-range-sk id="tooltip-commit-range-link"></commit-range-sk>
          </li>
        </ul>
        <point-links-sk id="tooltip-point-links"></point-links-sk>
        ${this.getCommitInfo()} ${this.anomalyTemplate()}
        <triage-menu-sk id="triage-menu" ?hidden=${!(this.anomaly && this.anomaly!.bug_id === 0)}>
        </triage-menu-sk>

        <ul class="table" ?hidden=${!this.show_bisect_button && !this._show_pinpoint_buttons}>
          <li>
            <span id="tooltip-key">Pinpoint</span>
            <div class="buttons">
              <button
                id="bisect"
                @click=${this.openBisectDialog}
                ?hidden=${!this.show_bisect_button}>
                Bisect
              </button>
              <button
                id="try-job"
                @click=${this.openTryJobDialog}
                ?hidden=${!this._show_pinpoint_buttons}>
                Request Trace
              </button>
            </div>
          </li>
        </ul>

        <ul class="table" ?hidden=${!!this.anomaly}>
          <li>
            <span id="tooltip-key">User Issues</span>
            <div class="buttons">
              <user-issue-sk id="tooltip-user-issue-sk"></user-issue-sk>
            </div>
          </li>
        </ul>

        <div id="json-source-dialog" ?hidden=${!this.json_source}>
          <json-source-sk id="json-source-sk"></json-source-sk>
        </div>
        <bisect-dialog-sk id="bisect-dialog-sk"></bisect-dialog-sk>
        <pinpoint-try-job-dialog-sk id="pinpoint-try-job-dialog-sk"></pinpoint-try-job-dialog-sk>
      </div>
    `;
  }

  moveTo(position: { x: number; y: number } | null): void {
    const div = this.containerDiv.value;
    if (!div) {
      return;
    }
    if (!position) {
      div!.style.display = 'none';
      return;
    }
    div!.style.display = 'block';

    const viewportWidth = Math.max(
      document.documentElement.clientWidth || 0,
      window.innerWidth || 0
    );
    const viewportHeight = Math.max(
      document.documentElement.clientHeight || 0,
      window.innerHeight || 0
    );

    this.margin.left = this.margin.left ?? parseInt(getComputedStyle(div!).marginLeft);
    this.margin.right = this.margin.right ?? parseInt(getComputedStyle(div!).marginRight);
    this.margin.top = this.margin.top ?? parseInt(getComputedStyle(div!).marginTop);
    this.margin.bottom = this.margin.bottom ?? parseInt(getComputedStyle(div!).marginBottom);

    const parentLeft = div.parentElement?.getBoundingClientRect().left || 0;
    const parentTop = this.parentElement?.getBoundingClientRect().top || 0;
    const rect = div.getBoundingClientRect();
    const left = parentLeft + position.x + this.margin.left! + rect.width;
    const top = parentTop + position.y + this.margin.top! + rect.height;

    const adjustedX =
      left > viewportWidth
        ? position.x - (rect.width + this.margin.left! + this.margin.right!)
        : position.x;

    const adjustedY = top > viewportHeight ? position.y - (top - viewportHeight) : position.y;

    div!.style.left = `${adjustedX}px`;
    div!.style.top = `${adjustedY}px`;
  }

  private getCommitInfo() {
    if (
      this.commit_info === null ||
      ((this._is_range || this._is_range === null) && !this._always_show_commit_info)
    ) {
      return html``;
    }
    return html`<ul class="table">
      <li>
        <span id="tooltip-key">Author</span>
        <span id="tooltip-text">${this.commit_info?.author.split('(')[0]} </span>
      </li>
      <li>
        <span id="tooltip-key">Message</span>
        <span id="tooltip-text">${this.commit_info?.message} </span>
      </li>
      <li>
        <span id="tooltip-key">Commit</span>
        <span id="tooltip-text">
          <a href="${this.commit_info?.url}" target="_blank">${TrimHash(this.commit_info?.hash)}</a>
        </span>
      </li>
    </ul>`;
  }

  private anomalyTemplate() {
    if (this.anomaly === null) {
      this.triageMenu?.toggleButtons(true);
      return html``;
    }

    if (this.anomaly.is_improvement) {
      this._nudgeList = null;
    }

    if (this.anomaly.bug_id === 0) {
      this.triageMenu!.setAnomalies([this.anomaly!], [this._trace_name], this._nudgeList);
    }

    return html`
      <ul class="table" id="anomaly-details">
        <li>
          <span id="tooltip-key">Anomaly</span>
          <span id="tooltip-text">${this.anomalyType()}</span>
        </li>
        <li>
          <span id="tooltip-key">Anomaly Range</span>
          <commit-range-sk id="anomaly-commit-range-link"></commit-range-sk>
        </li>
        <li>
          <span id="tooltip-key">Median</span>
          <span id="tooltip-text">
            ${formatNumber(this.anomaly!.median_after_anomaly)} ${this.unit_type.split(' ')[0]}
          </span>
        </li>
        <li>
          <span id="tooltip-key">Previous</span>
          <span id="tooltip-text">
            ${formatNumber(this.anomaly!.median_before_anomaly)} [${this.anomalyChange()}%]
          </span>
        </li>
        ${this.anomaly!.multiplicity
          ? html` <li>
              <span id="tooltip-key">Multiplicity</span>
              <span id="tooltip-text"> ${this.anomaly!.multiplicity} </span>
            </li>`
          : ''}
        ${this.anomaly!.is_legacy
          ? html` <li>
              <span id="tooltip-key">Legacy anomaly</span>
              <span id="tooltip-text"> </span>
            </li>`
          : ''}
        ${this.anomaly!.bug_id
          ? html` <li>
              <span id="tooltip-key">Bug ID</span>
              <span id="tooltip-text">${formatBug(this.bug_host_url, this.anomaly!.bug_id)}</span>
              <close-icon-sk
                id="unassociate-bug-button"
                @click=${this.unassociateBug}
                ?hidden=${this.anomaly!.bug_id === 0}>
              </close-icon-sk>
            </li>`
          : ''}
        <li>${this.pinpointJobLinks()}</li>
      </ul>
    `;
  }

  connectedCallback(): void {
    super.connectedCallback();

    this.addEventListener('anomaly-changed', () => {
      this.requestUpdate();
    });

    this.addEventListener('user-issue-changed', (e) => {
      this.bug_id = (e as CustomEvent).detail.bug_id;
    });
  }

  private anomalyType() {
    if (this.anomaly!.is_improvement) {
      return html`<span class="improvement">Improvement</span>`;
    }
    return html`<span class="regression">Regression</span>`;
  }

  private anomalyChange() {
    let divClass: string = 'regression';
    const change = formatPercentage(
      getPercentChange(this.anomaly!.median_before_anomaly, this.anomaly!.median_after_anomaly)
    );
    if (this.anomaly!.is_improvement) {
      divClass = 'improvement';
    }
    return html`<span class="${divClass}">${change}</span>`;
  }

  private pinpointJobLinks() {
    if (!this.anomaly || !this.anomaly.bisect_ids || this.anomaly.bisect_ids.length === 0) {
      return html``;
    }
    const links = this.anomaly.bisect_ids.map(
      (id) =>
        html`<a href="https://pinpoint-dot-chromeperf.appspot.com/job/${id}" target="_blank"
          >${id}</a
        >`
    );

    return html` <span id="tooltip-key">Pinpoint</span>
      <span id="tooltip-text">
        ${links.map((link, index) => html`${link}${index < links.length - 1 ? html`<br />` : ''}`)}
      </span>`;
  }

  load(
    index: number,
    test_name: string,
    trace_name: string,
    unit_type: string,
    y_value: number,
    date_value: Date,
    commit_position: CommitNumber,
    bug_id: number,
    anomaly: Anomaly | null,
    nudgeList: NudgeEntry[] | null,
    commit: ColumnHeader | null,
    tooltipFixed: boolean,
    commitRange: CommitRangeSk | null,
    closeButtonAction: () => void,
    color?: string,
    user_id?: string
  ): void {
    this.index = index;
    this.test_name = test_name;
    this._trace_name = trace_name;
    this.unit_type = unit_type.replace('_', ' ');
    this.y_value = y_value;
    this.date_value = date_value;
    this.commit_position = commit_position;
    this.bug_id = bug_id;
    this.anomaly = anomaly;
    this._nudgeList = nudgeList;
    this._close_button_action = closeButtonAction;
    this.tooltip_fixed = tooltipFixed;
    this.commit_info = commit;
    this.color = color || defaultColors[this.index % defaultColors.length];

    if (commitRange && this.commitRangeSk) {
      this._is_range = commitRange.isRange();
      this.commitRangeSk.hashes = commitRange.hashes;
      this.commitRangeSk.trace = commitRange.trace;
      this.commitRangeSk.header = commitRange.header;
      this.commitRangeSk.commitIndex = commitRange.commitIndex;
    }

    if (this.userIssueSk) {
      this.userIssueSk.user_id = user_id || '';
      this.userIssueSk.bug_id = bug_id;
      this.userIssueSk.trace_key = formatSpecialFunctions(this._trace_name);
      const commitPos = this.commit_position?.toString() || '';
      this.userIssueSk.commit_position = parseInt(commitPos);
    }
  }

  protected updated(changedProperties: Map<string | number | symbol, unknown>) {
    super.updated(changedProperties);
    if (changedProperties.has('anomaly') && this.anomaly) {
      const anomalyRangeSk = this.querySelector('#anomaly-commit-range-link') as CommitRangeSk;
      if (anomalyRangeSk) {
        const prev_commit = (this.anomaly.start_revision - 1) as CommitNumber;
        const commit = this.anomaly.end_revision as CommitNumber;
        anomalyRangeSk.setRange(prev_commit, commit);
      }
    }
  }

  loadPointLinks(
    commit_position: CommitNumber | null,
    prev_commit_position: CommitNumber | null,
    trace_id: string,
    keysForCommitRange: string[],
    keysForUsefulLinks: string[],
    commitLinks: (CommitLinks | null)[]
  ): Promise<(CommitLinks | null)[]> {
    return this.pointLinks!.load(
      commit_position,
      prev_commit_position,
      trace_id,
      keysForCommitRange!,
      keysForUsefulLinks!,
      commitLinks
    );
  }

  loadJsonResource(commit_position: CommitNumber | null, trace_id: string) {
    if (this.jsonSourceDialog === null || commit_position === null || trace_id === null) {
      return;
    }
    this.jsonSourceDialog!.cid = commit_position;
    this.jsonSourceDialog!.traceid = trace_id;
  }

  private handleCommitRangeChanged = () => {
    // handled by properties changes in Lit
    this.requestUpdate();
  };

  reset(): void {
    this.bug_id = 0;
    this.index = -1;
    this.commit_info = null;
    this.commitRangeSk?.reset();
    this.pointLinks?.reset();
    this.bisectDialog?.reset();
  }

  private unassociateBug() {
    this.triageMenu!.makeEditAnomalyRequest([this.anomaly!], [this._trace_name], 'RESET');
  }

  private openBisectDialog() {
    this.bisectDialog!.open();
  }

  private openTryJobDialog() {
    this.tryJobDialog!.open();
  }

  setBisectInputParams(preloadInputs: BisectPreloadParams): void {
    this.bisectDialog!.setBisectInputParams(preloadInputs);
  }

  setTryJobInputParams(preloadInputs: TryJobPreloadParams): void {
    this.tryJobDialog!.setTryJobInputParams(preloadInputs);
  }

  openNewBug(): void {
    if (this.triageMenu) {
      this.triageMenu.fileBug();
    }
  }

  openExistingBug(): void {
    if (this.triageMenu) {
      this.triageMenu.openExistingBugDialog();
    }
  }

  ignoreAnomaly(): void {
    if (this.triageMenu) {
      this.triageMenu.ignoreAnomaly();
    }
  }
}
