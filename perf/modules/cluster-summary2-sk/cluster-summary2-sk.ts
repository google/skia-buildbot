/**
 * @module modules/cluster-summary2-sk
 * @description <h2><code>cluster-summary2-sk</code></h2>
 *
 * @evt open-keys - An event that is fired when the user clicks the 'View on
 *     dashboard' button that contains the shortcut id, and the timestamp range of
 *     traces in the details that should be opened in the explorer, and the xbar
 *     location specified as a serialized cid.CommitID, for example:
 *
 *     {
 *       shortcut: 'X1129832198312',
 *       begin: 1476982874,
 *       end: 1476987166,
 *       xbar: {'offset':24750,'timestamp':1476985844},
 *     }
 *
 * @evt triaged - An event generated when the 'Update' button is pressed, which
 *     contains the new triage status. The detail contains the cid and triage
 *     status, for example:
 *
 *     {
 *       cid: {
 *         source: 'master',
 *         offset: 25004,
 *       },
 *       triage: {
 *         status: 'negative',
 *         messge: 'This is a regression in ...',
 *       },
 *     }
 *
 * @attr {Boolean} notriage - If true then don't display the triage controls.
 *
 * @example
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import 'elements-sk/styles/buttons';
import 'elements-sk/collapse-sk';
import '../commit-detail-panel-sk';
import '../plot-simple-sk';
import '../triage2-sk';
import '../word-cloud-sk';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { CollapseSk } from 'elements-sk/collapse-sk/collapse-sk';
import { errorMessage } from '../errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { Login } from '../../../infra-sk/modules/login';
import {
  FullSummary,
  FrameResponse,
  ClusterSummary,
  TriageStatus,
  Commit,
  CommitNumber,
  Status,
  ColumnHeader,
  Alert,
  StepDetection,
  CIDHandlerResponse,
} from '../json';
import { PlotSimpleSkTraceEventDetails } from '../plot-simple-sk/plot-simple-sk';
import { PlotSimpleSk } from '../plot-simple-sk/plot-simple-sk';
import { CommitDetailPanelSk } from '../commit-detail-panel-sk/commit-detail-panel-sk';
import '../window/window';

/** Defines a func that takes a number and formats it as a string. */
type Formatter = (n: number)=> string;

/**
 * Each type of step detection gives different meaning to the regression,
 * stepSize, and least_squares values in the cluster summary, so we create a
 * mapping to describe each of their labels and how the values should be
 * formatted.
 */
interface LabelsAndFormatters {
  regression: string;
  stepSize: string;
  lse: string;
  regressionFormatter: Formatter;
  stepSizeFormatter: Formatter;
  lseFormatter: Formatter;
}

// Oddly this seems to be the only way to retrieve the Intl locale for the
// browser.
const locale = Intl.NumberFormat().resolvedOptions().locale;

// These are different number formatters used in labelsForStepDetection.
const percentFormatter = Intl.NumberFormat(locale, { style: 'percent', maximumSignificantDigits: 4 }).format;
const decimalFormatter = Intl.NumberFormat(locale, { style: 'decimal', maximumSignificantDigits: 4 }).format;
const emptyFormatter = () => '';

/** Map each StepDetection to the labels and formatters used for that type. */
const labelsForStepDetection: Record<StepDetection, LabelsAndFormatters> = {
  '': {
    regression: 'Regression Factor:',
    regressionFormatter: decimalFormatter,
    stepSize: 'Step Size:',
    stepSizeFormatter: decimalFormatter,
    lse: 'Least Squares Error:',
    lseFormatter: decimalFormatter,
  },
  absolute: {
    regression: 'Absolute Change:',
    regressionFormatter: decimalFormatter,
    stepSize: '',
    stepSizeFormatter: emptyFormatter,
    lse: '',
    lseFormatter: emptyFormatter,
  },
  const: {
    regression: 'Constant Threshhold:',
    regressionFormatter: decimalFormatter,
    stepSize: '',
    stepSizeFormatter: emptyFormatter,
    lse: '',
    lseFormatter: emptyFormatter,
  },
  percent: {
    regression: 'Percentage Change:',
    regressionFormatter: percentFormatter,
    stepSize: '',
    stepSizeFormatter: emptyFormatter,
    lse: '',
    lseFormatter: emptyFormatter,
  },
  cohen: {
    regression: 'Standard Deviations:',
    regressionFormatter: decimalFormatter,
    stepSize: '',
    stepSizeFormatter: emptyFormatter,
    lse: '',
    lseFormatter: emptyFormatter,
  },
  mannwhitneyu: {
    regression: 'p:',
    regressionFormatter: percentFormatter,
    stepSize: '',
    stepSizeFormatter: emptyFormatter,
    lse: 'U:',
    lseFormatter: decimalFormatter,
  },
};

export interface ClusterSummary2SkTriagedEventDetail {
  columnHeader: ColumnHeader;
  triage: TriageStatus;
}

export interface ClusterSummary2SkOpenKeysEventDetail {
  shortcut: string;
  begin: number;
  end: number;
  xbar: ColumnHeader;
}

export class ClusterSummary2Sk extends ElementSk {
  private summary: ClusterSummary;

  private triageStatus: TriageStatus;

  private wordCloud: CollapseSk | null = null;

  private status: HTMLDivElement | null = null;

  private graph: PlotSimpleSk | null = null;

  private rangelink: HTMLAnchorElement | null = null;

  private commits: CommitDetailPanelSk | null = null;

  private frame: FrameResponse | null = null;

  private fullSummary: FullSummary | null = null;

  private _alert: Alert | null = null;

  private labels: LabelsAndFormatters = labelsForStepDetection['']

  constructor() {
    super(ClusterSummary2Sk.template);
    this.summary = {
      centroid: null,
      shortcut: '',
      step_point: null,
      num: 0,
      step_fit: {
        regression: 0,
        least_squares: 0,
        step_size: 0,
        turning_point: 0,
        status: 'Uninteresting',
      },
      param_summaries2: [],
      ts: new Date().toISOString(),
    };
    this.triageStatus = {
      status: '',
      message: '',
    };
  }

  /**
   * Look up the commit ids for the given offsets and sources.
   *
   * @param An array of CommitNumbers.
   * @returns A Promise that resolves the cids and returns an Array of serialized perfgit.Commit.
   */
  static lookupCids(cids: CommitNumber[]): Promise<CIDHandlerResponse> {
    return fetch('/_/cid/', {
      method: 'POST',
      body: JSON.stringify(cids),
      headers: {
        'Content-Type': 'application/json',
      },
    }).then(jsonOrThrow);
  }

  private static template = (ele: ClusterSummary2Sk) => html`
    <div class="regression ${ele.statusClass()}">
      ${ele.labels.regression}
      <span>${ele.labels.regressionFormatter(ele.summary!.step_fit!.regression)}</span>
    </div>
    <div class="stats">
      <div class="labelled">
        Cluster Size:
        <span>${ele.summary.num}</span>
      </div>
      ${ClusterSummary2Sk.leastSquares(ele)}
      <div class="labelled">
        ${ele.labels.stepSize}
        <span>${ele.labels.stepSizeFormatter(ele.summary!.step_fit!.step_size)}</span>
      </div>
    </div>
    <plot-simple-sk
      class="plot"
      width="800"
      height="250"
      specialevents
      @trace_selected=${ele.traceSelected}
    ></plot-simple-sk>
    <div id="status" class=${ele.hiddenClass()}>
      <p class="disabledMessage">You must be logged in to change the status.</p>
      <triage2-sk
        value=${ele.triageStatus.status}
        @change=${(e: CustomEvent<Status>) => {
    ele.triageStatus.status = e.detail;
  }}
      ></triage2-sk>
      <input
        type="text"
        .value=${ele.triageStatus.message}
        @change=${(e: InputEvent) => {
    ele.triageStatus.message = (e.currentTarget! as HTMLInputElement).value;
  }}
        label="Message"
      />
      <button class="action" @click=${ele.update}>Update</button>
    </div>
    <commit-detail-panel-sk id="commits" selectable></commit-detail-panel-sk>
    <div class="actions">
      <button id="shortcut" @click=${ele.openShortcut}>
        View on dashboard
      </button>
      <button @click=${ele.toggleWordCloud}>Word Cloud</button>
      <a id="permalink" class=${ele.hiddenClass()} href=${ele.permaLink()}>
        Permlink
      </a>
      <a id="rangelink" href="" target="_blank"></a>
    </div>
    <collapse-sk class="wordCloudCollapse" closed>
      <word-cloud-sk .items=${ele.summary.param_summaries2}></word-cloud-sk>
    </collapse-sk>
  `;

  private static leastSquares = (ele: ClusterSummary2Sk) => html`
        <div class="labelled">
          ${ele.labels.lse}
          <span>${ele.labels.lseFormatter(ele.summary!.step_fit!.least_squares)}</span>
        </div>
      `;

  connectedCallback(): void {
    super.connectedCallback();
    this._upgradeProperty('full_summary');
    this._upgradeProperty('triage');
    this._upgradeProperty('alert');
    this._render();
    this.wordCloud = this.querySelector('.wordCloudCollapse');
    this.status = this.querySelector('#status');
    this.graph = this.querySelector('plot-simple-sk');
    this.rangelink = this.querySelector('#rangelink');
    this.commits = this.querySelector('#commits');
    Login.then((status) => {
      this.status!.classList.toggle('disabled', status.Email === '');
    }).catch(errorMessage);

    // eslint-disable-next-line no-self-assign
    this.full_summary = this.full_summary;
    // eslint-disable-next-line no-self-assign
    this.triage = this.triage;
  }

  private update() {
    const columnHeader = this.summary.step_point!;
    const detail: ClusterSummary2SkTriagedEventDetail = {
      columnHeader,
      triage: this.triage,
    };
    this.dispatchEvent(
      new CustomEvent<ClusterSummary2SkTriagedEventDetail>('triaged', {
        detail,
        bubbles: true,
      }),
    );
  }

  private openShortcut() {
    const detail: ClusterSummary2SkOpenKeysEventDetail = {
      shortcut: this.summary.shortcut,
      begin: this.frame!.dataframe!.header![0]!.timestamp,
      end:
        this.frame!.dataframe!.header![
          this.frame!.dataframe!.header!.length - 1
        ]!.timestamp + 1,
      xbar: this.summary.step_point!,
    };
    this.dispatchEvent(
      new CustomEvent<ClusterSummary2SkOpenKeysEventDetail>('open-keys', {
        detail,
        bubbles: true,
      }),
    );
  }

  private traceSelected(e: CustomEvent<PlotSimpleSkTraceEventDetails>) {
    const commitNumber = this.frame!.dataframe!.header![e.detail.x]?.offset;
    ClusterSummary2Sk.lookupCids([commitNumber!])
      .then((json) => {
        this.commits!.details = json.commitSlice || [];
      })
      .catch(errorMessage);
  }

  private toggleWordCloud() {
    this.wordCloud!.closed = !this.wordCloud!.closed;
  }

  private hiddenClass() {
    return this.hasAttribute('notriage') ? 'hidden' : '';
  }

  private permaLink() {
    // Bounce to the triage page, but with the time range narrowed to
    // contain just the step_point commit.
    if (!this.summary || !this.summary.step_point) {
      return '';
    }
    const begin = this.summary.step_point.timestamp;
    const end = begin + 1;
    return `/t/?begin=${begin}&end=${end}&subset=all`;
  }

  private statusClass() {
    if (!this.summary) {
      return '';
    }
    const status = this.summary!.step_fit!.status || '';
    return status.toLowerCase();
  }

  /** @prop full_summary {string} A serialized:
   *
   *  {
   *    summary: cluster2.ClusterSummary,
   *    frame: dataframe.FrameResponse,
   *  }
   *
   */
  get full_summary(): FullSummary | null {
    return this.fullSummary;
  }

  set full_summary(val: FullSummary | null) {
    if (!val) {
      return;
    }
    if (!val.frame) {
      return;
    }
    this.fullSummary = val;
    this.summary = val.summary;
    this.frame = val.frame;
    if (!this.graph) {
      return;
    }

    // Set the data- attributes used for sorting cluster summaries.
    this.dataset.clustersize = this.summary.num.toString();
    this.dataset.steplse = this.summary!.step_fit!.least_squares.toPrecision(2);
    this.dataset.stepsize = this.summary!.step_fit!.step_size.toPrecision(2);
    this.dataset.stepregression = this.summary!.step_fit!.regression.toPrecision(
      2,
    );
    // We take in a ClusterSummary, but need to transform all that data
    // into a format that plot-sk can handle.
    this.graph.removeAll();
    const labels: Date[] = [];
    this.full_summary!.frame!.dataframe!.header!.forEach((header) => {
      labels.push(new Date(header!.timestamp * 1000));
    });
    this.graph.addLines({ centroid: this.summary.centroid! }, labels);
    // Set the x-bar but only if status != uninteresting.
    if (this.summary!.step_fit!.status !== 'Uninteresting') {
      // Loop through the dataframe header to find the location we should
      // place the x-bar at.
      const step = this.summary.step_point;
      let xbar = -1;
      this.frame!.dataframe!.header!.forEach((h, i) => {
        if (h!.offset === step!.offset) {
          xbar = i;
        }
      });
      if (xbar !== -1) {
        this.graph.xbar = xbar;
      }

      // If step_point is set then display the commit
      // details for the xbar location.
      if (step && step.offset > 0) {
        ClusterSummary2Sk.lookupCids([step.offset])
          .then((json) => {
            this.commits!.details = json.commitSlice || [];
          })
          .catch(errorMessage);
      }

      // Populate rangelink.
      if (window.sk.perf.commit_range_url !== '') {
        // First find the commit at step_fit, and the next previous commit that has data.
        let prevCommit = xbar - 1;
        while (prevCommit > 0 && this.summary!.centroid![prevCommit] === 1e32) {
          prevCommit -= 1;
        }
        const cids: CommitNumber[] = [
          this.frame!.dataframe!.header![prevCommit]!.offset,
          this.frame!.dataframe!.header![xbar]!.offset,
        ];
        // Run those through cid lookup to get the hashes.
        ClusterSummary2Sk.lookupCids(cids)
          .then((json: CIDHandlerResponse) => {
            // Create the URL.
            let url = window.sk.perf.commit_range_url;
            url = url.replace('{begin}', json.commitSlice![0].hash);
            url = url.replace('{end}', json.commitSlice![1].hash);
            // Now populate link, including text and href.
            this.rangelink!.href = url;
            this.rangelink!.innerText = 'Commits At Step';
          })
          .catch(errorMessage);
      } else {
        this.rangelink!.href = '';
        this.rangelink!.innerText = '';
      }
    } else {
      this.rangelink!.href = '';
      this.rangelink!.innerText = '';
    }

    this._render();
  }

  /** @prop triage {string} The triage status of the cluster.
   *     Something of the form:
   *
   *    {
   *      status: 'untriaged',
   *      message: 'This is a regression.',
   *    }
   *
   */
  get triage(): TriageStatus {
    return this.triageStatus;
  }

  set triage(val: TriageStatus) {
    if (!val) {
      return;
    }
    this.triageStatus = val;
    this._render();
  }

  /** The configured Alert that found this regression. */
  get alert(): Alert | null {
    return this._alert;
  }

  set alert(val: Alert | null) {
    if (!val) {
      return;
    }
    this._alert = val;
    this.labels = labelsForStepDetection[val!.step];
    this._render();
  }
}

define('cluster-summary2-sk', ClusterSummary2Sk);
