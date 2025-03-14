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
import { html, css, LitElement } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { createRef, ref } from 'lit/directives/ref.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { upgradeProperty } from '../../../elements-sk/modules/upgradeProperty';
import { Anomaly, Commit, CommitNumber } from '../json';
import { AnomalySk } from '../anomaly-sk/anomaly-sk';
import { lookupCids } from '../cid/cid';
import { CommitRangeSk } from '../commit-range-sk/commit-range-sk';
import '../window/window';
import { TriageMenuSk, NudgeEntry } from '../triage-menu-sk/triage-menu-sk';
import '../triage-menu-sk/triage-menu-sk';
import '../user-issue-sk/user-issue-sk';
import '../bisect-dialog-sk/bisect-dialog-sk';
import { UserIssueSk } from '../user-issue-sk/user-issue-sk';
import '../../../elements-sk/modules/icons/close-icon-sk';
import '../../../elements-sk/modules/icons/check-icon-sk';
import '@material/web/elevation/elevation.js';
import { removeSpecialFunctions } from '../paramtools';
import { PointLinksSk } from '../point-links-sk/point-links-sk';
import { BisectDialogSk, BisectPreloadParams } from '../bisect-dialog-sk/bisect-dialog-sk';
import { defaultColors } from '../common/plot-builder';

@customElement('commit-info-sk')
export class CommitInfoSk extends LitElement {
  static styles = css`
    ul.table {
      display: table;
      list-style: none;
      padding: 0;
      margin: 0;
      width: 100%;
      border-collapse: collapse;
      margin-bottom: 16px;
      li {
        display: table-row;
        text-align: right;
        b {
          display: table-cell;
          text-align: left;
          padding-right: 1em;
        }
      }
      a {
        color: var(--primary);
      }
    }
  `;

  @property({ attribute: false })
  commitInfo: Commit | null = null;

  // render generates commit information into a list. note that this does not
  // include the header.
  render() {
    if (!this.commitInfo) {
      return;
    }

    return html`
      <ul class="table">
        <li>
          <b>Author:</b>
          ${this.commitInfo.author.split('(')[0]}
        </li>
        <li>
          <b>Message:</b>
          ${this.commitInfo.message}
        </li>
      </ul>
    `;
  }
}

export class ChartTooltipSk extends ElementSk {
  constructor() {
    super(ChartTooltipSk.template);
  }

  // Index of the trace in the dataframe.
  private _index: number = -1;

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

  commitInfo: Commit | null = null;

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

  private triageMenu: TriageMenuSk | null = null;

  private preloadBisectInputs: BisectPreloadParams | null = null;

  _tooltip_fixed: boolean = false;

  _close_button_action: () => void = () => {};

  // Commit range element. Values usually set by explore-simple-sk when a point
  // is selected.
  commitRangeSk: CommitRangeSk | null = null;

  // Whether to skip display of commit detail.
  private _skip_commit_detail_display: boolean = window.perf
    ? window.perf.skip_commit_detail_display
    : false;

  // Shows any buganizer issue associated with a data point.
  userIssueSk: UserIssueSk | null = null;

  // Cached margin to compute once.
  private margin: { left?: number; right?: number; bottom?: number; top?: number } = {};

  private containerDiv = createRef<HTMLDivElement>();

  // Point links display commit ranges for points (ie/ V8, WebRTC) if configured
  // for the instance. See "data_point_config" in chrome-perf-non-public.json
  // for an example of the configuration.
  private pointLinks: PointLinksSk | null = null;

  // Bisect Dialog.
  bisectDialog: BisectDialogSk | null = null;

  // The overall html template for outlining the contents needed in
  // chart-tooltip.
  //
  // Notes:
  // * The "More details" button is currently set to fetch commit information
  //   via the POST /_/cid api call. Usually, the response details from that api
  //   call can also be used to determine if the given point is an anomaly, but
  //   chart tooltip is unaware of the anoamly map maintained in plot-simple-sk.
  //   "More details" should be updated to trigger an event to explore-simple-sk
  //   that can set commit and anoamly information to the chart-tooltip at the
  //   time the two elements are integrated.
  // * commit range information is not present because explore-simple-sk's
  //   dataframe, and the (x, y) coordinates of the selected point on the chart
  //   are needed to calculate both the trace and header for commit-range.
  //
  // TODO(b/338440689) - make commit number a link to gitiles
  private static template = (ele: ChartTooltipSk) => html`
    <div class="container" ${ref(ele.containerDiv)}>
      <md-elevation style="--md-elevation-level: 3"></md-elevation>
      <button id="closeIcon" @click=${ele._close_button_action} ?hidden=${!ele._tooltip_fixed}>
        <close-icon-sk></close-icon-sk>
      </button>
      <h3>
        <span style="color:${defaultColors[ele.index % defaultColors.length]}">
          ${ele.test_name || `untitled_key`}
          <span ?hidden=${!ele.anomaly}> [Anomaly] </span>
        </span>
      </h3>
      <ul class="table">
        <li>
          <b>Date:</b>
          ${ele.date_value.toDateString()}
        </li>
        <li>
          <b>Value:</b>
          ${ele.y_value} ${ele.unit_type}
        </li>
        <li>
          <b>Change:</b>
          <commit-range-sk id="tooltip-commit-range-link"></commit-range-sk>
        </li>
      </ul>
      <commit-info-sk .commitInfo=${ele.commitInfo} ?hidden=${!ele._tooltip_fixed}></commit-info-sk>
      ${ele._tooltip_fixed ? ele.anomalyTemplate() : ele.seeMoreText()}
      <triage-menu-sk
        id="triage-menu"
        ?hidden=${!(ele._tooltip_fixed && ele.anomaly && ele.anomaly!.bug_id === 0)}>
      </triage-menu-sk>
      <div class="buttons">
        <button id="bisect" @click=${ele.openBisectDialog} ?hidden=${!ele._tooltip_fixed}>
          Bisect
        </button>
        <user-issue-sk
          id="tooltip-user-issue-sk"
          ?hidden=${!ele._tooltip_fixed || ele.anomaly}></user-issue-sk>
      </div>
      <bisect-dialog-sk id="bisect-dialog-sk"></bisect-dialog-sk>
      <point-links-sk
        id="tooltip-point-links"
        .commitPosition=${ele.commit_position}
        ?hidden=${!ele._tooltip_fixed}></point-links-sk>
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

  private seeMoreText() {
    if (this.commitInfo !== null) {
      return;
    }

    return html`<span class="see-more-text">Click for more details</span>`;
  }

  // HTML template for Anomaly information, only shown when the data
  // point is an anomaly. Usually set by the results of POST /_/cid
  // correlated against anomaly map.
  private anomalyTemplate() {
    if (this.anomaly === null) {
      this.triageMenu!.toggleButtons(true);
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
          <b>Anomaly</b>
          ${this.anomalyType()}
        </li>
        <li>
          <b>Median Value:</b>
          ${AnomalySk.formatNumber(this.anomaly!.median_after_anomaly)}
          ${this.unit_type.split(' ')[0]}
        </li>
        <li>
          <b>Prior Median:</b>
          <span
            >${AnomalySk.formatNumber(this.anomaly!.median_before_anomaly)}
            [${this.anomalyChange()}%]</span
          >
        </li>
        ${this.anomaly!.bug_id
          ? html` <li>
              <b>Bug Id:</b>
              ${AnomalySk.formatBug(this.bug_host_url, this.anomaly!.bug_id)}
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
    this._render();

    this.commitRangeSk = this.querySelector('#tooltip-commit-range-link');
    this.userIssueSk = this.querySelector('#tooltip-user-issue-sk');
    this.triageMenu = this.querySelector('#triage-menu');
    this.pointLinks = this.querySelector('#tooltip-point-links');
    this.bisectDialog = this.querySelector('#bisect-dialog-sk');

    this.addEventListener('anomaly-changed', () => {
      this._render();
    });

    this.addEventListener('user-issue-changed', (e) => {
      this.bug_id = (e as CustomEvent).detail.bug_id;
      this._render();
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
    const change = AnomalySk.formatPercentage(
      AnomalySk.getPercentChange(
        this.anomaly!.median_before_anomaly,
        this.anomaly!.median_after_anomaly
      )
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

    return html`<div>
      <b>Pinpoint Jobs:</b>
      ${links.map((link, index) => html`${link}${index < links.length - 1 ? ', ' : ''}`)}
    </div>`;
  }

  // fetch_details triggers an event that executes the POST /_/cid call to
  // retrieve commit details and anomaly information.
  //
  // Note: This should be updated to trigger an event back to explore-simple-sk
  // to determine whether the currently selected point is an anomaly
  // (from anomaly map).
  fetch_details = async (): Promise<void> => {
    const cids: CommitNumber[] = [this.commit_position!];

    const json = await lookupCids(cids);
    const details = json.commitSlice![0];

    // Setter will re-render component.
    this.commitInfo = details;
  };

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
    commit: Commit | null,
    tooltipFixed: boolean,
    commitRange: CommitRangeSk | null,
    closeButtonAction: () => void
  ): void {
    this._index = index;
    this._test_name = test_name;
    this._trace_name = trace_name;
    this._unit_type = unit_type;
    this._y_value = y_value;
    this._date_value = date_value;
    this._commit_position = commit_position;
    this._bug_id = bug_id;
    this._anomaly = anomaly;
    this._nudgeList = nudgeList;
    this._tooltip_fixed = tooltipFixed;
    this._close_button_action = closeButtonAction;
    this.commitInfo = commit;

    if (commitRange && this.commitRangeSk) {
      this.commitRangeSk.showLinks = tooltipFixed;
      this.commitRangeSk.trace = commitRange.trace;
      this.commitRangeSk.commitIndex = commitRange.commitIndex;
      this.commitRangeSk.header = commitRange.header;
    }

    if (this.userIssueSk !== null) {
      this.userIssueSk.bug_id = bug_id;
      this.userIssueSk.trace_key = removeSpecialFunctions(this._trace_name);
      const commitPos = this.commit_position?.toString() || '';
      this.userIssueSk.commit_position = parseInt(commitPos);
    }
    this._render();
  }

  loadPointLinks(
    commit_position: CommitNumber | null,
    prev_commit_position: CommitNumber | null,
    trace_id: string,
    keysForCommitRange: string[],
    keysForUsefulLinks: string[]
  ) {
    if (commit_position === null || prev_commit_position === null) {
      return;
    }
    this.pointLinks!.load(
      commit_position,
      prev_commit_position,
      trace_id,
      keysForCommitRange!,
      keysForUsefulLinks!
    );
  }

  /** Clear Point Links */
  reset(): void {
    this.commitRangeSk?.reset();
    this.pointLinks?.reset();
    this._render();
  }

  private unassociateBug() {
    this.triageMenu!.makeEditAnomalyRequest([this._anomaly!], [this._trace_name], 'RESET');
  }

  private openBisectDialog() {
    this.bisectDialog!.open();
  }

  setBisectInputParams(preloadInputs: BisectPreloadParams): void {
    this.preloadBisectInputs = preloadInputs;
    this.bisectDialog!.setBisectInputParams(this.preloadBisectInputs);
    this._render();
  }

  get index(): number {
    return this._index;
  }

  set index(val: number) {
    this._index = val;
    this._render();
  }

  get test_name(): string {
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
}

define('chart-tooltip-sk', ChartTooltipSk);
