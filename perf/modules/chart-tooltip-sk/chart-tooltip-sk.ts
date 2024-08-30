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
import { html, TemplateResult } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { upgradeProperty } from '../../../elements-sk/modules/upgradeProperty';
import { Anomaly, CommitNumber } from '../json';
import { AnomalySk } from '../anomaly-sk/anomaly-sk';
import { lookupCids } from '../cid/cid';
import { CommitRangeSk } from '../commit-range-sk/commit-range-sk';
import '../window/window';
import { IngestFileLinksSk } from '../ingest-file-links-sk/ingest-file-links-sk';
import '../../../elements-sk/modules/icons/close-icon-sk';

export class Commit {
  commit_hash: string = '';

  timestamp: string = '';

  author: string = '';

  commit_url: string = '';

  constructor(
    commit_hash: string,
    timestamp: number,
    author: string,
    commit_url: string
  ) {
    this.commit_hash = commit_hash;
    // timestamp is in seconds, so we need to multiply by 1000 to get ms
    this.timestamp = new Date(timestamp * 1000).toDateString();
    this.author = author;
    this.commit_url = commit_url;
  }

  // render generates commit information into a list. note that this does not
  // include the header.
  render(): TemplateResult {
    return html`
      <ul class="table">
        <li>
          <span>Commit:</span>
          <span>
            <a href="${this.commit_url}" target="_blank">
              ${this.commit_hash.substring(0, 7)}
            </a>
          </span>
        </li>
        <li>
          <span>Date:</span>
          <span>${this.timestamp}</span>
        </li>
        <li>
          <span>Author:</span>
          <span>${this.author}</span>
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

  // The y value of the selected point on the chart.
  private _y_value: number = -1;

  // Commit position of the selected point on the chart,
  // usually curated through explore-simple-sk._dataframe.header[x].
  private _commit_position: CommitNumber | null = null;

  // Commit details
  private _commit: Commit | null = null;

  // Anomaly information, set only when the data point is an anomaly.
  // Usually determined by content in anomaly map referenced against the result
  // of POST /_/cid.
  private _anomaly: Anomaly | null = null;

  // Host bug url, usually from window.perf.bug_host_url.
  private _bug_host_url: string = window.perf.bug_host_url;

  _tooltip_fixed: boolean = false;

  _close_button_action: Function = () => {};

  // Commit range element. Values usually set by explore-simple-sk when a point
  // is selected.
  commitRangeSk: CommitRangeSk | null = null;

  // Ingest file links element. Provides links based on cid and
  ingestFileLinks: IngestFileLinksSk | null = null;

  // Fields below are used for chart tooltip styling

  // If the tooltip is to be displayed or hidden
  _display: boolean = false;

  // The position from top of it's first position:relative parent in px
  top: number = 0;

  // The position from left of it's first position:relative parent in px
  left: number = 0;

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
    <div
      class="container"
      style="display: ${ele._display ? 'block' : 'none'};
             left: ${ele.left}px; top: ${ele.top}px;">
      <button
        id="closeIcon"
        @click=${ele._close_button_action}
        ?hidden=${!ele._tooltip_fixed}>
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
      <div class="revlink">
        <a href="/v/?revisionId=${ele.commit_position}" target="_blank">
          Regressions at ${ele.commit_position}
        </a>
      </div>
      ${ele.anomalyTemplate()} ${ele.commitTemplate()}
      <commit-range-sk id="tooltip-commit-range-sk"></commit-range-sk>
      <ingest-file-links-sk
        id="tooltip-ingest-file-links"></ingest-file-links-sk>
      ${ele.seeMoreText()}
      <button
        class="action"
        id="close"
        @click=${ele._close_button_action}
        ?hidden=${!ele._tooltip_fixed}>
        Close
      </button>
    </div>
  `;

  // HTML template for Commit information, only displayed when commit
  // data is provided. Commit information is usually provided by the
  // results of POST /_/cid response.
  private commitTemplate() {
    if (this.commit == null) {
      return html``;
    }

    return html` <h4>Commit Details</h4>
      ${this.commit.render()}`;
  }

  private seeMoreText() {
    if (this._commit !== null) {
      return html``;
    }

    return html`
      <span class="see-more-text">*Click on the point to see more details</span>
    `;
  }

  // HTML template for Anomaly information, only shown when the data
  // point is an anomaly. Usually set by the results of POST /_/cid
  // correlated against anomaly map.
  private anomalyTemplate() {
    if (this.anomaly == null) {
      return html``;
    }

    // TOOD(jeffyoon@) - add revision range formatting
    return html`
      <h4>Anomaly Details</h4>
      <ul class="table">
        <li>
          <span>Score:</span>
          <span>
            ${AnomalySk.formatNumber(this.anomaly!.median_after_anomaly)}
          </span>
        </li>
        <li>
          <span>Prior Score:</span>
          <span>
            ${AnomalySk.formatNumber(this.anomaly!.median_before_anomaly)}
          </span>
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
    this._render();

    this.commitRangeSk = this.querySelector('#tooltip-commit-range-sk');
    this.ingestFileLinks = this.querySelector('#tooltip-ingest-file-links');
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
    this.commit = new Commit(
      details.hash,
      details.ts,
      details.author,
      details.url
    );
  };

  // load function sets the value of the fields minimally required to display
  // this chart on hover.
  load(
    test_name: string,
    y_value: number,
    commit_position: CommitNumber,
    anomaly: Anomaly | null,
    commit: Commit | null,
    displayFileLinks: boolean,
    tooltipFixed: boolean,
    closeButtonAction: Function
  ): void {
    this._test_name = test_name;
    this._y_value = y_value;
    this._commit_position = commit_position;
    this._anomaly = anomaly;
    this._commit = commit;
    this._tooltip_fixed = tooltipFixed;
    this._close_button_action = closeButtonAction;

    if (displayFileLinks && commit_position != null && test_name !== '') {
      this.ingestFileLinks?.load(commit_position, test_name);
    }

    this._render();
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

  get commit(): Commit | null {
    return this._commit;
  }

  set commit(val: Commit | null) {
    this._commit = val;
    this._render();
  }

  get bug_host_url(): string {
    return this._bug_host_url;
  }

  set bug_host_url(val: string) {
    this._bug_host_url = val;
    this._render();
  }

  set display(val: boolean) {
    this._display = val;
    this._render();
  }
}

define('chart-tooltip-sk', ChartTooltipSk);
