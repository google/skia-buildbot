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
import { html, TemplateResult } from 'lit-html';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { upgradeProperty } from '../../../elements-sk/modules/upgradeProperty';
import { Anomaly, CommitNumber } from '../json';
import { AnomalySk } from '../anomaly-sk/anomaly-sk';
import { lookupCids } from '../cid/cid';
import { CommitRangeSk } from '../commit-range-sk/commit-range-sk';
import '../window/window';
import { IngestFileLinksSk } from '../ingest-file-links-sk/ingest-file-links-sk';

class Commit {
  commit_hash: string = '';

  timestamp: string = '';

  author: string = '';

  message: string = '';

  commit_url: string = '';

  constructor(
    commit_hash: string,
    timestamp: number,
    author: string,
    message: string,
    commit_url: string
  ) {
    this.commit_hash = commit_hash;
    this.timestamp = new Date(timestamp).toDateString();
    this.author = author;
    this.message = message;
    this.commit_url = commit_url;
  }

  // render generates commit information into a list. note that this does not
  // include the header.
  render(): TemplateResult {
    return html`
      <ul>
        <li>commit <a href=${this.commit_url}>${this.commit_hash}</a></li>
        <li>Date: ${this.timestamp}</li>
        <li>Author: ${this.author}</li>
        <li>Message: ${this.message}</li>
        <li>Date: ${this.timestamp}</li>
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
  private _bug_host_url: string = 'https://bugs.chromium.org';

  // Commit range element. Values usually set by explore-simple-sk when a point
  // is selected.
  commitRangeSk: CommitRangeSk | null = null;

  // Ingest file links element. Provides links based on cid and
  ingestFileLinks: IngestFileLinksSk | null = null;

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
    <div>
      <h3>${ele.test_name}</h3>
      <ul>
        <li>Value: <b>${ele.y_value}</b></li>
        <li>Commit Number: ${ele.commit_position}</li>
      </ul>
      ${ele.anomalyTemplate()} ${ele.commitTemplate()}
      <commit-range-sk id="tooltip-commit-range-sk"></commit-range-sk>
      <ingest-file-links-sk
        id="tooltip-ingest-file-links"></ingest-file-links-sk>
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
      <ul>
        <li>
          Score: ${AnomalySk.formatNumber(this.anomaly!.median_after_anomaly)}
        </li>
        <li>
          Prior Score:
          ${AnomalySk.formatNumber(this.anomaly!.median_before_anomaly)}
        </li>
        <li>
          Percent Change:
          ${AnomalySk.formatPercentage(
            AnomalySk.getPercentChange(
              this.anomaly!.median_before_anomaly,
              this.anomaly!.median_after_anomaly
            )
          )}%
        </li>
        <li>Improvement: ${this.anomaly!.is_improvement}</li>
        <li>
          Bug Id:
          ${AnomalySk.formatBug(this.bug_host_url, this.anomaly!.bug_id)}
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
      details.message,
      details.url
    );
  };

  // load function sets the value of the fields minimally required to display
  // this chart on hover.
  load(
    test_name: string,
    y_value: number,
    commit_position: CommitNumber,
    anomaly: Anomaly | null
  ): void {
    this._test_name = test_name;
    this._y_value = y_value;
    this._commit_position = commit_position;
    if (anomaly !== null) {
      this._anomaly = anomaly;
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
}

define('chart-tooltip-sk', ChartTooltipSk);
