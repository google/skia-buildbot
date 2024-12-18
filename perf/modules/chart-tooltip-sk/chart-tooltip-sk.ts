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
import { IngestFileLinksSk } from '../ingest-file-links-sk/ingest-file-links-sk';
import { TriageMenuSk, NudgeEntry } from '../triage-menu-sk/triage-menu-sk';
import '../triage-menu-sk/triage-menu-sk';
import '../user-issue-sk/user-issue-sk';
import { UserIssueSk } from '../user-issue-sk/user-issue-sk';
import '../../../elements-sk/modules/icons/close-icon-sk';
import '../../../elements-sk/modules/icons/check-icon-sk';
import '@material/web/elevation/elevation.js';
import { removeSpecialFunctions } from '../paramtools';

@customElement('commit-info-sk')
export class CommitInfoSk extends LitElement {
  static styles = css`
    ul.table {
      list-style: none;
      padding: 0;
      margin: 0;
      width: 100%;
      border-collapse: collapse;
      margin-bottom: 16px;

      li {
        display: table-row;

        span {
          &:first-child {
            font-weight: bold;
          }

          display: table-cell;
          padding: 1px 6px;
        }
      }

      a {
        color: var(--primary);
      }
    }

    ul.table#anomaly-details {
      margin-bottom: 5px;
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
          <span>Commit:</span>
          <span>
            <a href="${this.commitInfo.url}" target="_blank">
              ${this.commitInfo.hash.substring(0, 7)}
            </a>
          </span>
        </li>
        <li>
          <span>Date:</span>
          <span>${new Date(this.commitInfo.ts * 1000).toDateString()}</span>
        </li>
        <li>
          <span>Author:</span>
          <span>${this.commitInfo.author}</span>
        </li>
      </ul>
    `;
  }
}

export class ChartTooltipSk extends ElementSk {
  constructor() {
    super(ChartTooltipSk.template);
  }

  // Full name (id) of the point in question (e.detail.name)
  private _test_name: string = '';

  // Trace Name to pass to NewBugDialog.
  private _trace_name: string = '';

  // The y value of the selected point on the chart.
  private _y_value: number = -1;

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

  _tooltip_fixed: boolean = false;

  _close_button_action: () => void = () => {};

  // Commit range element. Values usually set by explore-simple-sk when a point
  // is selected.
  commitRangeSk: CommitRangeSk | null = null;

  // Ingest file links element. Provides links based on cid and
  ingestFileLinks: IngestFileLinksSk | null = null;

  // Shows any buganizer issue associated with a data point.
  userIssueSk: UserIssueSk | null = null;

  // Cached margin to compute once.
  private margin: { left?: number; right?: number; bottom?: number; top?: number } = {};

  private containerDiv = createRef<HTMLDivElement>();

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
      <h3>${ele.test_name}</h3>
      <ul class="table">
        <li>
          <span>Value:</span>
          <span>${ele.y_value}</span>
        </li>
        <li>
          <span>Commit Number:</span>
          <span>${ele.commit_position}</span>
        </li>
      </ul>
      <user-issue-sk id="tooltip-user-issue-sk"></user-issue-sk>
      <div class="revlink">
        <a href="/v/?revisionId=${ele.commit_position}" target="_blank">
          Regressions at ${ele.commit_position}
        </a>
      </div>
      <commit-info-sk .commitInfo=${ele.commitInfo}></commit-info-sk>
      <commit-range-sk id="tooltip-commit-range-sk"></commit-range-sk>
      <ingest-file-links-sk id="tooltip-ingest-file-links"></ingest-file-links-sk>
      ${ele.seeMoreText()} ${ele.anomalyTemplate()}
      <triage-menu-sk
        id="triage-menu"
        ?hidden=${!(ele._tooltip_fixed && ele.anomaly && ele.anomaly!.bug_id === 0)}>
      </triage-menu-sk>
      <button
        class="action"
        id="close"
        @click=${ele._close_button_action}
        ?hidden=${!ele._tooltip_fixed}>
        Close
      </button>
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

    return html`<span class="see-more-text">*Click on the point to see more details</span>`;
  }

  // HTML template for Anomaly information, only shown when the data
  // point is an anomaly. Usually set by the results of POST /_/cid
  // correlated against anomaly map.
  private anomalyTemplate() {
    if (this.anomaly === null) {
      return html``;
    }

    if (this.anomaly.bug_id === 0) {
      this.triageMenu!.setAnomalies([this.anomaly!], [this._trace_name], this._nudgeList);
    }

    // TOOD(jeffyoon@) - add revision range formatting
    return html`
      <h4>Anomaly Details</h4>
      <ul class="table" id="anomaly-details">
        <li>
          <span>Score:</span>
          <span> ${AnomalySk.formatNumber(this.anomaly!.median_after_anomaly)} </span>
        </li>
        <li>
          <span>Prior Score:</span>
          <span> ${AnomalySk.formatNumber(this.anomaly!.median_before_anomaly)} </span>
        </li>
        <li>
          <span>Percentage Change:</span>
          <span>
            ${AnomalySk.formatPercentage(
              AnomalySk.getPercentChange(
                this.anomaly!.median_before_anomaly,
                this.anomaly!.median_after_anomaly
              )
            )}%
          </span>
        </li>
        <li>
          <span>Improvement:</span>
          <span>${this.anomaly!.is_improvement}</span>
        </li>
        <li>
          <span>Bug Id:</span>
          <span>
            ${AnomalySk.formatBug(this.bug_host_url, this.anomaly!.bug_id)}
            <close-icon-sk
              id="unassociate-bug-button"
              @click=${this.unassociateBug}
              ?hidden=${this.anomaly!.bug_id === 0}>
            </close-icon-sk>
          </span>
        </li>
      </ul>
    `;
  }

  connectedCallback(): void {
    super.connectedCallback();
    upgradeProperty(this, 'test_name');
    upgradeProperty(this, 'y_value');
    upgradeProperty(this, 'commit_position');
    upgradeProperty(this, 'commit');
    upgradeProperty(this, 'anomaly');
    upgradeProperty(this, 'bug_host_url');
    upgradeProperty(this, 'bug_id');
    this._render();

    this.commitRangeSk = this.querySelector('#tooltip-commit-range-sk');
    this.ingestFileLinks = this.querySelector('#tooltip-ingest-file-links');
    this.userIssueSk = this.querySelector('#tooltip-user-issue-sk');
    this.triageMenu = this.querySelector('#triage-menu');

    this.addEventListener('anomaly-changed', () => {
      this._render();
    });

    this.addEventListener('user-issue-changed', (e) => {
      this.bug_id = (e as CustomEvent).detail.bug_id;
      this._render();
    });
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
    test_name: string,
    trace_name: string,
    y_value: number,
    commit_position: CommitNumber,
    bug_id: number,
    anomaly: Anomaly | null,
    nudgeList: NudgeEntry[] | null,
    commit: Commit | null,
    displayFileLinks: boolean,
    tooltipFixed: boolean,
    closeButtonAction: () => void
  ): void {
    this._test_name = test_name;
    this._trace_name = trace_name;
    this._y_value = y_value;
    this._commit_position = commit_position;
    this._bug_id = bug_id;
    this._anomaly = anomaly;
    this._nudgeList = nudgeList;
    this._tooltip_fixed = tooltipFixed;
    this._close_button_action = closeButtonAction;
    this.commitInfo = commit;

    if (displayFileLinks && commit_position !== null && test_name !== '') {
      this.ingestFileLinks?.load(commit_position, test_name);
    }

    if (this.userIssueSk !== null) {
      this.userIssueSk.bug_id = bug_id;
      this.userIssueSk.trace_key = removeSpecialFunctions(this._trace_name);
      const commitPos = this.commit_position?.toString() || '';
      this.userIssueSk.commit_position = parseInt(commitPos);
    }

    this._render();
  }

  private unassociateBug() {
    this.triageMenu!.makeEditAnomalyRequest([this._anomaly!], [this._trace_name], 'RESET');
  }

  get test_name(): string {
    return this._test_name;
  }

  set test_name(val: string) {
    this._test_name = val;
    this._render();
  }

  get y_value(): number {
    return this._y_value;
  }

  set y_value(val: number) {
    this._y_value = val;
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
    if (val && this.test_name !== '') {
      this.ingestFileLinks?.load(val, this.test_name);
    }
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
