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
import { html } from 'lit';
import { createRef, ref } from 'lit/directives/ref.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { upgradeProperty } from '../../../elements-sk/modules/upgradeProperty';
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
import '../../../elements-sk/modules/icons/check-icon-sk';
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
import { JSONSourceSk } from '../json-source-sk/json-source-sk';

export class ChartTooltipSk extends ElementSk {
  constructor() {
    super(ChartTooltipSk.template);
  }

  // Index of the trace in the dataframe.
  private _index: number = -1;

  // The color of the trace.
  private _color: string = '';

  // Full name (id) of the point in question (e.detail.name)
  private _test_name: string = '';

  // Trace Name to pass to NewBugDialog.
  private _trace_name: string = '';

  // Unit of measurement for trace.
  private _unit_type: string = '';

  // The y value of the selected point on the chart.
  private _y_value: number = -1;

  // The timestamp converted to Date of the selected point on the chart.
  private _date_value: Date = new Date();

  // Commit position of the selected point on the chart,
  // usually curated through explore-simple-sk._dataframe.header[x].
  private _commit_position: CommitNumber | null = null;

  private _commit_info: ColumnHeader | null = null;

  // Anomaly information, set only when the data point is an anomaly.
  // Usually determined by content in anomaly map referenced against the result
  // of POST /_/cid.
  private _anomaly: Anomaly | null = null;

  private _nudgeList: NudgeEntry[] | null = null;

  // Host bug url, usually from window.perf.bug_host_url.
  private _bug_host_url: string = window.perf ? window.perf.bug_host_url : '';

  // bug_id = 0 signifies no buganizer issue available in the database for the
  // data point. bug_id > 0 means we have an existing buganizer issue.
  private _bug_id: number = 0;

  _show_pinpoint_buttons =
    window.perf?.git_repo_url?.includes('https://chromium.googlesource.com/chromium/src') || false;

  show_bisect_button = !!window.perf?.show_bisect_btn;

  private triageMenu: TriageMenuSk | null = null;

  private preloadBisectInputs: BisectPreloadParams | null = null;

  private preloadTryJobInputs: TryJobPreloadParams | null = null;

  private _is_tooltip_fixed: boolean = false;

  _is_range: boolean | null = null;

  _close_button_action: () => void = () => {};

  // Commit range element. Values usually set by explore-simple-sk when a point
  // is selected.
  commitRangeSk: CommitRangeSk | null = null;

  // Shows any buganizer issue associated with a data point.
  userIssueSk: UserIssueSk | null = null;

  // Cached margin to compute once.
  private margin: { left?: number; right?: number; bottom?: number; top?: number } = {};

  private containerDiv = createRef<HTMLDivElement>();

  // Point links display commit ranges for points (ie/ V8, WebRTC) if configured
  // for the instance. See "data_point_config" in chrome-perf-non-public.json
  // for an example of the configuration.
  pointLinks: PointLinksSk | null = null;

  // dialog for displaying JSON source if configured for the instance.
  // See "data_point_config" in chrome-perf-non-public.json
  // for an example of the configuration.
  jsonSourceDialog: JSONSourceSk | null = null;

  // Whether to display of json source dialog.
  private _show_json_source: boolean = window.perf ? window.perf.show_json_file_display : false;

  private _always_show_commit_info: boolean = window.perf
    ? window.perf.always_show_commit_info
    : false;

  // Bisect Dialog.
  bisectDialog: BisectDialogSk | null = null;

  // Request debug trace dialog. This dialog creates a try job on legacy Pinpoint
  // TODO(b/391784563): hide request debug trace when no tracing links have surfaced
  tryJobDialog: PinpointTryJobDialogSk | null = null;

  // The overall html template for outlining the contents needed in
  // chart-tooltip.
  //
  // Notes:
  // * The "More details" button is currently set to fetch commit information
  //   via the POST /_/cid api call. Usually, the response details from that api
  //   call can also be used to determine if the given point is an anomaly, but
  //   chart tooltip is unaware of the AnomalyMap.
  //   "More details" should be updated to trigger an event to explore-simple-sk
  //   that can set commit and anoamly information to the chart-tooltip at the
  //   time the two elements are integrated.
  // * commit range information is not present because explore-simple-sk's
  //   dataframe, and the (x, y) coordinates of the selected point on the chart
  //   are needed to calculate both the trace and header for commit-range.
  //
  // TODO(b/338440689) - make commit number a link to gitiles
  // TODO(b/408558084) - remove div when adding a new field to determine
  // displaying json moodule or not
  private static template = (ele: ChartTooltipSk) => html`
    <div class="container" ${ref(ele.containerDiv)}>
      <md-elevation></md-elevation>
      <button id="closeIcon" @click=${ele._close_button_action} ?hidden=${!ele.tooltip_fixed}>
        <close-icon-sk></close-icon-sk>
      </button>
      <h3>
        <span style="color:${ele.color}">
          ${ele.test_name || `Default`}
          <span ?hidden=${!ele.anomaly}> [Anomaly] </span>
        </span>
      </h3>
      <ul class="table">
        <li>
          <span id="tooltip-key">Date</span>
          <span id="tooltip-text">${ele.date_value.toUTCString()}</span>
        </li>
        <li>
          <span id="tooltip-key">Value</span>
          <span id="tooltip-text">${ele.y_value} ${ele.unit_type}</span>
        </li>
        <li>
          <span id="tooltip-key">Point Range</span>
          <commit-range-sk id="tooltip-commit-range-link"></commit-range-sk>
        </li>
      </ul>
      <point-links-sk id="tooltip-point-links"></point-links-sk>
      ${ele.getCommitInfo()} ${ele.anomalyTemplate()}
      <triage-menu-sk id="triage-menu" ?hidden=${!(ele.anomaly && ele.anomaly!.bug_id === 0)}>
      </triage-menu-sk>
      <div class="buttons">
        <button id="bisect" @click=${ele.openBisectDialog} ?hidden=${!ele.show_bisect_button}>
          Bisect
        </button>
        <button id="try-job" @click=${ele.openTryJobDialog} ?hidden=${!ele._show_pinpoint_buttons}>
          Request Trace
        </button>
        <user-issue-sk id="tooltip-user-issue-sk" ?hidden=${ele.anomaly}></user-issue-sk>
      </div>
      <div id="json-source-dialog" ?hidden=${!ele._show_json_source}>
        <json-source-sk id="json-source-sk"></json-source-sk>
      </div>
      <bisect-dialog-sk id="bisect-dialog-sk"></bisect-dialog-sk>
      <pinpoint-try-job-dialog-sk id="pinpoint-try-job-dialog-sk"></pinpoint-try-job-dialog-sk>
    </div>
  `;

  /**
   * Move the tooltip to the given position. Width uses viewport while
   * height ensures the tooltip tries to stay within the confines of
   * the chart.
   * @param position The position relative to its parent; hidden if null.
   */
  moveTo(position: { x: number; y: number } | null): void {
    const div = this.containerDiv.value;
    if (!div) {
      return;
    }
    if (!position) {
      div!.style.display = 'none';
      return;
    }
    // displaying the element here allows us to fetch the correct
    // rectangle dimensions for the tooltip
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

    // Shift to the left if the element exceeds the viewport.
    const adjustedX =
      left > viewportWidth
        ? position.x - (rect.width + this.margin.left! + this.margin.right!)
        : position.x;

    // Shift to the top if the element exceeds the chart height.
    // Rather than show the tooltip directly above or directly below the
    // data point, shift it by how much the the tooltip exceeds the viewport.
    // This prevents the tooltip from appearing out of the viewport.
    const adjustedY = top > viewportHeight ? position.y - (top - viewportHeight) : position.y;

    div!.style.left = `${adjustedX}px`;
    div!.style.top = `${adjustedY}px`;
  }

  private getCommitInfo() {
    // If commit info is a range and config is not set to always show,
    // then do not show the commit info.
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
          <a href="${this.commit_info?.url}" target="_blank"
            >${this.commit_info?.hash.substring(0, 8)}</a
          >
        </span>
      </li>
    </ul>`;
  }

  // HTML template for Anomaly information, only shown when the data
  // point is an anomaly. Usually set by the results of POST /_/cid
  // correlated against anomaly map.
  private anomalyTemplate() {
    if (this.anomaly === null) {
      this.triageMenu?.toggleButtons(true);
      return html``;
    }

    // Nullify nudgelist to ensure nudging is not available.
    if (this.anomaly.is_improvement) {
      this._nudgeList = null;
    }

    if (this.anomaly.bug_id === 0) {
      this.triageMenu!.setAnomalies([this.anomaly!], [this._trace_name], this._nudgeList);
    }

    // TOOD(jeffyoon@) - add revision range formatting
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
          ? // multiplicity is only populated in perf/anomalies/impl/sql_impl.go,
            // so only when we fetch anomalies from sql.
            html` <li>
              <span id="tooltip-key">Multiplicity</span>
              <span id="tooltip-text"> ${this.anomaly!.multiplicity} </span>
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
    upgradeProperty(this, 'test_name');
    upgradeProperty(this, 'unit_value');
    upgradeProperty(this, 'y_value');
    upgradeProperty(this, 'commit_position');
    upgradeProperty(this, 'commit');
    upgradeProperty(this, 'anomaly');
    upgradeProperty(this, 'bug_host_url');
    upgradeProperty(this, 'bug_id');
    upgradeProperty(this, 'preloadBisectInputs');
    upgradeProperty(this, 'color');
    upgradeProperty(this, 'preloadTryJobInputs');
    this._render();

    this.commitRangeSk = this.querySelector('#tooltip-commit-range-link');
    this.userIssueSk = this.querySelector('#tooltip-user-issue-sk');
    this.triageMenu = this.querySelector('#triage-menu');
    this.pointLinks = this.querySelector('#tooltip-point-links');
    this.bisectDialog = this.querySelector('#bisect-dialog-sk');
    this.tryJobDialog = this.querySelector('#pinpoint-try-job-dialog-sk');
    this.jsonSourceDialog = this.querySelector('#json-source-sk');

    this.addEventListener('anomaly-changed', () => {
      this._render();
    });

    this.addEventListener('user-issue-changed', (e) => {
      this.bug_id = (e as CustomEvent).detail.bug_id;
      this._render();
    });

    if (this.commitRangeSk) {
      this.commitRangeSk.addEventListener('commit-range-changed', this.handleCommitRangeChanged);
    }
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    // Clean up listeners when the element is removed from the DOM
    if (this.commitRangeSk) {
      this.commitRangeSk.removeEventListener('commit-range-changed', this.handleCommitRangeChanged);
    }
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

  // load function sets the value of the fields minimally required to display
  // this chart on hover.
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
    this._index = index;
    this._test_name = test_name;
    this._trace_name = trace_name;
    this._unit_type = unit_type.replace('_', ' ');
    this._y_value = y_value;
    this._date_value = date_value;
    this._commit_position = commit_position;
    this._bug_id = bug_id;
    this._anomaly = anomaly;
    this._nudgeList = nudgeList;
    this._close_button_action = closeButtonAction;
    this.tooltip_fixed = tooltipFixed;
    this.commit_info = commit;
    this.color = color || defaultColors[this._index % defaultColors.length];

    if (commitRange && this.commitRangeSk) {
      this._is_range = this.commitRangeSk.isRange();
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

    this._render();

    // Needs to be after _render().
    if (this._anomaly) {
      const anomalyRangeSk = this.querySelector('#anomaly-commit-range-link') as CommitRangeSk;
      if (anomalyRangeSk) {
        const prev_commit = (this._anomaly.start_revision - 1) as CommitNumber;
        const commit = this._anomaly.end_revision as CommitNumber;
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

  // Handles the event from commit-range-sk when its link is updated.
  private handleCommitRangeChanged = () => {
    this._render();
  };

  /** Clear Point Links */
  reset(): void {
    this.bug_id = 0;
    this.index = -1;
    this.commit_info = null;
    this.commitRangeSk?.reset();
    this.pointLinks?.reset();
    this.bisectDialog?.reset();
    this._render();
  }

  private unassociateBug() {
    this.triageMenu!.makeEditAnomalyRequest([this._anomaly!], [this._trace_name], 'RESET');
  }

  private openBisectDialog() {
    this.bisectDialog!.open();
  }

  private openTryJobDialog() {
    this.tryJobDialog!.open();
  }

  setBisectInputParams(preloadInputs: BisectPreloadParams): void {
    this.preloadBisectInputs = preloadInputs;
    this.bisectDialog!.setBisectInputParams(this.preloadBisectInputs);
    this._render();
  }

  setTryJobInputParams(preloadInputs: TryJobPreloadParams): void {
    this.preloadTryJobInputs = preloadInputs;
    this.tryJobDialog!.setTryJobInputParams(this.preloadTryJobInputs);
    this._render();
  }

  get index(): number {
    return this._index;
  }

  set index(val: number) {
    this._index = val;
    this._render();
  }

  get color(): string {
    return this._color;
  }

  set color(val: string) {
    this._color = val;
    this._render();
  }

  get test_name(): string {
    // TODO(seawardt): Separate long paths.
    return this._test_name;
  }

  set test_name(val: string) {
    this._test_name = val;
    this._render();
  }

  get unit_type(): string {
    return this._unit_type;
  }

  set unit_type(val: string) {
    this._unit_type = val;
    this._render();
  }

  get y_value(): number {
    return this._y_value;
  }

  set y_value(val: number) {
    this._y_value = val;
    this._render();
  }

  get date_value(): Date {
    return this._date_value;
  }

  set date_value(val: Date) {
    this._date_value = val;
    this._render();
  }

  get anomaly(): Anomaly | null {
    return this._anomaly;
  }

  set anomaly(val: Anomaly | null) {
    this._anomaly = val;
    // TODO(jeffyoon@) - include revision formatting and URL
    // generation
    this._render();
  }

  get commit_position(): CommitNumber | null {
    return this._commit_position;
  }

  set commit_position(val: CommitNumber | null) {
    this._commit_position = val;
    this._render();
  }

  get bug_host_url(): string {
    return this._bug_host_url;
  }

  set bug_host_url(val: string) {
    this._bug_host_url = val;
    this._render();
  }

  get bug_id(): number {
    return this._bug_id;
  }

  set bug_id(val: number) {
    this._bug_id = val;
    this._render();
  }

  get commit_info(): ColumnHeader | null {
    return this._commit_info;
  }

  set commit_info(val: ColumnHeader | null) {
    this._commit_info = val;
    this._render();
  }

  get tooltip_fixed(): boolean {
    return this._is_tooltip_fixed;
  }

  set tooltip_fixed(val: boolean) {
    this._is_tooltip_fixed = val;
    this._render();
  }

  get json_source(): boolean {
    return this._show_json_source;
  }

  set json_source(val: boolean) {
    this._show_json_source = val;
    this._render();
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

define('chart-tooltip-sk', ChartTooltipSk);
