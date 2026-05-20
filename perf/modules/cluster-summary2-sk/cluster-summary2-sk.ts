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
import { html, LitElement } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import '../../../elements-sk/modules/collapse-sk';
import '../commit-detail-panel-sk';
import '../plot-google-chart-sk';
import '../triage2-sk';
import '../word-cloud-sk';
import '../commit-range-sk';
import { errorMessage } from '../errorMessage';
import { Status as LoginStatus } from '../../../infra-sk/modules/json';
import {
  FullSummary,
  FrameResponse,
  ClusterSummary,
  TriageStatus,
  Status,
  ColumnHeader,
  Alert,
  StepDetection,
} from '../json';
import { PlotShowTooltipEventDetails } from '../plot-google-chart-sk/plot-google-chart-sk';
import '../window/window';
import { lookupCids } from '../cid/cid';
import { LoggedIn } from '../../../infra-sk/modules/alogin-sk/alogin-sk';

/** Defines a func that takes a number and formats it as a string. */
type Formatter = (n: number) => string;

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
const percentFormatter = Intl.NumberFormat(locale, {
  style: 'percent',
  maximumSignificantDigits: 4,
}).format;
const decimalFormatter = Intl.NumberFormat(locale, {
  style: 'decimal',
  maximumSignificantDigits: 4,
}).format;
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
  stepiness: {
    regression: 'Stepiness:',
    regressionFormatter: decimalFormatter,
    stepSize: 'Step Size:',
    stepSizeFormatter: decimalFormatter,
    lse: '',
    lseFormatter: emptyFormatter,
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

@customElement('cluster-summary2-sk')
export class ClusterSummary2Sk extends LitElement {
  @state()
  private summary: ClusterSummary;

  @state()
  private triageStatus: TriageStatus;

  @state()
  private graphData: google.visualization.DataTable | null = null;

  @state()
  private xbarValue: number = -1;

  @state()
  private commitsDetails: any[] = [];

  @state()
  private wordCloudClosedValue: boolean = true;

  @state()
  private isEditor: boolean = false;

  private frame: FrameResponse | null = null;

  private fullSummary: FullSummary | null = null;

  private _alert: Alert | null = null;

  private labels: LabelsAndFormatters = labelsForStepDetection[''];

  constructor() {
    super();
    this.summary = {
      centroid: null,
      shortcut: '',
      step_point: null,
      notification_id: '',
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
  static lookupCids = lookupCids;

  createRenderRoot() {
    return this;
  }

  render() {
    return html`
      <div class="regression ${this.statusClass()}">
        ${this.labels.regression}
        <span>${this.labels.regressionFormatter(this.summary.step_fit?.regression || 0)}</span>
      </div>
      <div class="stats">
        <div class="labelled">
          Cluster Size:
          <span>${this.summary.num}</span>
        </div>
        ${this.leastSquares()}
        <div class="labelled">
          ${this.labels.stepSize}
          <span>${this.labels.stepSizeFormatter(this.summary.step_fit?.step_size || 0)}</span>
        </div>
        ${this.summary.notification_id
          ? html` <div>
              Bug:
              <a href="http://b/${this.summary.notification_id}"
                >b/${this.summary.notification_id}</a
              >
            </div>`
          : html``}
      </div>
      <div class="plot-wrapper">
        <plot-google-chart-sk
          specialevents
          .data=${this.graphData}
          .xbar=${this.xbarValue}
          @plot-data-select=${this.traceSelected}></plot-google-chart-sk>
      </div>
      <div id="status" class="${this.hiddenClass()} ${this.isEditor ? '' : 'disabled'}">
        <p class="disabledMessage">You must be logged in to change the status.</p>
        <triage2-sk
          .value=${this.triageStatus.status}
          @change=${(e: CustomEvent<Status>) => {
            this.triageStatus = { ...this.triageStatus, status: e.detail };
          }}></triage2-sk>
        <input
          type="text"
          .value=${this.triageStatus.message}
          @change=${(e: InputEvent) => {
            this.triageStatus = {
              ...this.triageStatus,
              message: (e.currentTarget! as HTMLInputElement).value,
            };
          }}
          label="Message" />
        <button class="action" @click=${this.updateStatus}>Update</button>
      </div>
      <commit-detail-panel-sk
        selectable
        .trace_id=${this.summary.shortcut}
        .details=${this.commitsDetails}></commit-detail-panel-sk>
      <div class="actions">
        <button id="shortcut" @click=${this.openShortcut}>View on dashboard</button>
        <button @click=${this.toggleWordCloud}>Word Cloud</button>
        <a id="permalink" class=${this.hiddenClass()} href=${this.permaLink()}> Permlink </a>
        <commit-range-sk
          .trace=${this.summary.centroid || []}
          .commitIndex=${this.xbarValue}
          .header=${this.frame?.dataframe?.header || null}></commit-range-sk>
      </div>
      <collapse-sk class="wordCloudCollapse" .closed=${this.wordCloudClosedValue}>
        <word-cloud-sk .items=${this.summary.param_summaries2}></word-cloud-sk>
      </collapse-sk>
    `;
  }

  private leastSquares() {
    return html`
      <div class="labelled">
        ${this.labels.lse}
        <span>${this.labels.lseFormatter(this.summary.step_fit?.least_squares || 0)}</span>
      </div>
    `;
  }

  connectedCallback(): void {
    super.connectedCallback();
    LoggedIn()
      .then((status: LoginStatus) => {
        this.isEditor = (status.roles || []).includes('editor');
      })
      .catch(errorMessage);
  }

  protected updated(changedProperties: Map<string | number | symbol, unknown>) {
    super.updated(changedProperties);
    if (changedProperties.has('fullSummary') && this.fullSummary) {
      this.updateGraphData();
    }
  }

  private updateGraphData() {
    this.dataset.clustersize = this.summary.num.toString();
    const step_fit = this.summary.step_fit;
    if (step_fit) {
      this.dataset.steplse = step_fit.least_squares.toPrecision(2);
      this.dataset.stepsize = step_fit.step_size.toPrecision(2);
      this.dataset.stepregression = step_fit.regression.toPrecision(2);
    }

    const headers = this.frame?.dataframe?.header;
    const validHeaders = headers ? headers.filter((h): h is ColumnHeader => h !== null) : [];

    if (
      this.summary.centroid &&
      this.summary.centroid.length > 0 &&
      validHeaders.length === this.summary.centroid.length
    ) {
      const rows = this.summary.centroid.map((value, i) => {
        const header = validHeaders[i];
        const label = new Date(header.timestamp * 1000);
        return [header.offset, label, value];
      });

      this.graphData = google.visualization.arrayToDataTable([
        ['Commit Position', 'Date', 'centroid'],
        ...rows,
      ]);
    }

    if (step_fit && step_fit.status !== 'Uninteresting') {
      const step = this.summary.step_point;
      if (step && headers) {
        let xbar = -1;
        headers.forEach((h, i) => {
          if (h && h.offset === step.offset) {
            xbar = i;
          }
        });
        if (xbar !== -1) {
          this.xbarValue = xbar;
        }

        if (step.offset > 0) {
          ClusterSummary2Sk.lookupCids([step.offset])
            .then((json) => {
              this.commitsDetails = json.commitSlice || [];
            })
            .catch(errorMessage);
        }
      }
    }
  }

  updateStatus() {
    const columnHeader = this.summary.step_point!;
    // Let's use standard CustomEvent constructor.
    this.dispatchEvent(
      new CustomEvent<ClusterSummary2SkTriagedEventDetail>('triaged', {
        detail: {
          columnHeader,
          triage: this.triage,
        },
        bubbles: true,
      })
    );
  }

  openShortcut() {
    const detail: ClusterSummary2SkOpenKeysEventDetail = {
      shortcut: this.summary.shortcut,
      begin: this.frame!.dataframe!.header![0]!.timestamp,
      end: Math.floor(Date.now() / 1000),
      xbar: this.summary.step_point!,
    };
    this.dispatchEvent(
      new CustomEvent<ClusterSummary2SkOpenKeysEventDetail>('open-keys', {
        detail,
        bubbles: true,
      })
    );
  }

  private traceSelected(e: CustomEvent<PlotShowTooltipEventDetails>) {
    const commitNumber = this.frame!.dataframe!.header![e.detail.tableRow]?.offset;
    ClusterSummary2Sk.lookupCids([commitNumber!])
      .then((json) => {
        this.commitsDetails = json.commitSlice || [];
      })
      .catch(errorMessage);
  }

  private toggleWordCloud() {
    this.wordCloudClosedValue = !this.wordCloudClosedValue;
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
    const status = this.summary.step_fit?.status || '';
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
  @property({ type: Object, noAccessor: true })
  get full_summary(): FullSummary | null {
    return this.fullSummary;
  }

  set full_summary(val: FullSummary | null) {
    if (!val || !val.frame) {
      return;
    }
    const oldVal = this.fullSummary;
    this.fullSummary = val;
    this.summary = val.summary;
    this.frame = val.frame;
    this.requestUpdate('full_summary', oldVal);
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
  @property({ type: Object, noAccessor: true })
  get triage(): TriageStatus {
    return this.triageStatus;
  }

  set triage(val: TriageStatus) {
    if (!val) {
      return;
    }
    const oldVal = this.triageStatus;
    this.triageStatus = val;
    this.requestUpdate('triage', oldVal);
  }

  @property({ type: Object, noAccessor: true })
  get alert(): Alert | null {
    return this._alert;
  }

  set alert(val: Alert | null) {
    if (!val) {
      return;
    }
    const oldVal = this._alert;
    this._alert = val;
    this.labels = labelsForStepDetection[val!.step] || labelsForStepDetection[''];
    this.requestUpdate('alert', oldVal);
  }
}
