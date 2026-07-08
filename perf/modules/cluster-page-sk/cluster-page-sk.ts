/**
 * @module module/cluster-page-sk
 * @description <h2><code>cluster-page-sk</code></h2>
 *
 *   The top level element for clustering traces.
 *
 */
import { html, LitElement } from 'lit';
import { customElement, state, query } from 'lit/decorators.js';
import { fromObject, toParamSet } from '../../../infra-sk/modules/query';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { HintableObject } from '../../../infra-sk/modules/hintable';
import { SpinnerSk } from '../../../elements-sk/modules/spinner-sk/spinner-sk';
import { errorMessage } from '../errorMessage';
import { CountMetric } from '../telemetry/telemetry';

import '../../../elements-sk/modules/spinner-sk';
import '../../../elements-sk/modules/checkbox-sk';

import '../../../infra-sk/modules/paramset-sk';
import '../../../infra-sk/modules/sort-sk';
import '../../../infra-sk/modules/query-sk';

import '../algo-select-sk';
import '../cluster-summary2-sk';
import '../commit-detail-picker-sk';
import '../day-range-sk';
import '../query-count-sk';
import {
  FrameResponse,
  ParamSet,
  RegressionDetectionRequest,
  ClusterAlgo,
  Commit,
  FullSummary,
  ConfirmedRegression,
  progress,
  SerializesToString,
} from '../json';
import { AlgoSelectAlgoChangeEventDetail } from '../algo-select-sk/algo-select-sk';
import { QuerySkQueryChangeEventDetail } from '../../../infra-sk/modules/query-sk/query-sk';
import { ClusterSummary2SkOpenKeysEventDetail } from '../cluster-summary2-sk/cluster-summary2-sk';
import { CommitDetailPanelSkCommitSelectedDetails } from '../commit-detail-panel-sk/commit-detail-panel-sk';
import { messagesToErrorString, startRequest } from '../progress/progress';

// The state that gets reflected to the URL.
class State {
  begin: number = Math.floor(Date.now() / 1000 - 24 * 60 * 60);

  end: number = Math.floor(Date.now() / 1000);

  offset: number = -1;

  radius: number = window.perf.radius;

  query: string = '';

  k: number = 0;

  algo: ClusterAlgo = 'kmeans';

  interesting: number = window.perf.interesting;

  sparse: boolean = false;

  constructor() {
    if (window.perf.demo) {
      this.begin = Math.floor(new Date(2020, 4, 1).valueOf() / 1000);
      this.end = Math.floor(new Date(2020, 5, 1).valueOf() / 1000);
    }
  }
}

@customElement('cluster-page-sk')
export class ClusterPageSk extends LitElement {
  // The state to be reflected to the URL.
  @state()
  private state = new State();

  @state()
  private paramset = ParamSet({});

  // The computed clusters.
  @state()
  private summaries: FullSummary[] = [];

  // The commits to choose from.
  private cids: Commit[] = [];

  // The id of the current cluster request. Will be the empty string if
  // there is no pending request.
  @state()
  private requestId: string = '';

  // The status of a running request.
  @state()
  private status: string = '';

  // The spinner we display when waiting for results.
  @query('#run-spinner')
  private spinner!: SpinnerSk | null;

  // The text status of the currently running request.
  @state()
  private runningStatus = '';

  createRenderRoot() {
    return this;
  }

  render() {
    return html`
      <h2>Commit</h2>
      <commit-detail-picker-sk
        @commit-selected=${this.commitSelected}
        .selection=${this.state.offset}
        id="commit"></commit-detail-picker-sk>

      <h2>Algorithm</h2>
      <algo-select-sk algo=${this.state.algo} @algo-change=${this.algoChange}></algo-select-sk>

      <h2>Query</h2>
      <div class="query-action">
        <query-sk
          @query-change=${this.queryChanged}
          .key_order=${window.perf.key_order}
          .paramset=${this.paramset}
          current_query=${this.state.query}></query-sk>
        <div id="selections">
          <h3>Selections</h3>
          <paramset-sk id="summary" .paramsets=${[toParamSet(this.state.query)]}></paramset-sk>
          <div>
            Matches:
            <query-count-sk
              url="/_/count"
              current_query=${this.state.query}
              @paramset-changed=${this.paramsetChanged}></query-count-sk>
          </div>
          <button
            @click=${this.start}
            class="action"
            id="start"
            ?disabled=${!!this.requestId || this.state.offset === -1}>
            Run
          </button>
          <div>
            <spinner-sk id="run-spinner"></spinner-sk>
            <span>${this.status}</span>
          </div>
        </div>
      </div>

      <details>
        <summary id="advanced">Advanced</summary>
        <div id="inputs">
          <label for="k_input">
            K (A value of 0 means the server chooses).
            <input id="k_input" .value=${this.state.k.toString()} @input=${this.kChange} />
          </label>
          <label for="radius_input">
            Number of commits to include on either side.
            <input
              id="radius_input"
              .value=${this.state.radius.toString()}
              @input=${this.radiusChange} />
          </label>
          <label for="interesting_input">
            Clusters are interesting if regression score &gt;= this.
            <input
              id="interesting_input"
              .value=${this.state.interesting.toString()}
              @input=${this.interestingChange} />
          </label>
          <checkbox-sk
            ?checked=${this.state.sparse}
            label="Data is sparse, so only include commits that have data."
            @input=${this.sparseChange}></checkbox-sk>
        </div>
      </details>

      <h2>Results</h2>
      <pre class="messages">${this.runningStatus}</pre>
      <sort-sk target="clusters">
        <button data-key="clustersize">Cluster Size</button>
        <button data-key="stepregression" data-default="up">Regression</button>
        <button data-key="stepsize">Step Size</button>
        <button data-key="steplse">Least Squares</button>
      </sort-sk>
      <div id="clusters" @open-keys=${this.openKeys}>${this._summaryRows()}</div>
    `;
  }

  private _summaryRows() {
    const ret = this.summaries.map(
      (summary) => html`
        <cluster-summary2-sk .full_summary=${summary} notriage></cluster-summary2-sk>
      `
    );
    if (!ret.length) {
      ret.push(html`<p class="info">No clusters found.</p>`);
    }
    return ret;
  }

  connectedCallback(): void {
    super.connectedCallback();

    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
    fetch(`/_/initpage/?tz=${tz}`)
      .then(jsonOrThrow)
      .then((json: FrameResponse) => {
        this.paramset = ParamSet(json.dataframe!.paramset);
      })
      .catch(errorMessage);

    this.stateHasChanged = stateReflector(
      () => this.state as unknown as HintableObject,
      (newState) => {
        this.state = newState as unknown as State;
      }
    );
  }

  // Call this anytime something in private state is changed. Will be replaced
  // with the real function once stateReflector has been setup.
  // eslint-disable-next-line @typescript-eslint/no-empty-function
  private stateHasChanged = () => {};

  private algoChange(e: CustomEvent<AlgoSelectAlgoChangeEventDetail>) {
    this.state = { ...this.state, algo: e.detail.algo };
    this.stateHasChanged();
  }

  private kChange(e: InputEvent) {
    this.state = { ...this.state, k: +(e.target! as HTMLInputElement).value };
    this.stateHasChanged();
  }

  private radiusChange(e: InputEvent) {
    this.state = { ...this.state, radius: +(e.target! as HTMLInputElement).value };
    this.stateHasChanged();
  }

  private interestingChange(e: InputEvent) {
    this.state = { ...this.state, interesting: +(e.target! as HTMLInputElement).value };
    this.stateHasChanged();
  }

  private sparseChange(e: InputEvent) {
    this.state = { ...this.state, sparse: (e.target! as HTMLInputElement).checked };
    this.stateHasChanged();
  }

  private queryChanged(e: CustomEvent<QuerySkQueryChangeEventDetail>) {
    this.state = { ...this.state, query: e.detail.q };
    this.stateHasChanged();
  }

  private paramsetChanged(e: CustomEvent<ParamSet>) {
    this.paramset = e.detail;
  }

  private openKeys(e: CustomEvent<ClusterSummary2SkOpenKeysEventDetail>) {
    const queryObj = {
      keys: e.detail.shortcut,
      begin: e.detail.begin,
      end: e.detail.end,
      xbaroffset: e.detail.xbar.offset,
      num_commits: 50,
      request_type: 1,
    };
    window.open(`/e/?${fromObject(queryObj)}`, '_blank');
  }

  private commitSelected(e: CustomEvent<CommitDetailPanelSkCommitSelectedDetails>) {
    this.state = { ...this.state, offset: (e.detail.commit as unknown as Commit).offset };
    this.stateHasChanged();
  }

  private catch(msg: string) {
    this.requestId = '';
    this.status = '';
    if (msg) {
      errorMessage(msg);
    }
  }

  private createRegressionDetectionRequest(): RegressionDetectionRequest {
    return {
      step: 0,
      total_queries: 0,
      alert: {
        id_as_string: '-1',
        display_name: '',
        radius: +this.state.radius,
        query: this.state.query,
        k: +this.state.k,
        algo: this.state.algo,
        interesting: +this.state.interesting,
        issue_tracker_component: SerializesToString(''),
        sparse: this.state.sparse,
        step: '',
        alert: '',
        bug_uri_template: '',
        state: 'ACTIVE',
        owner: '',
        step_up_only: false,
        direction: 'BOTH',
        group_by: '',
        minimum_num: 0,
        category: '',
        action: 'noaction',
      },
      domain: {
        offset: +this.state.offset,
        n: 0,
        end: new Date().toISOString(),
      },
    };
  }

  private processConfirmedRegressionResults(
    confirmedRegression?: ConfirmedRegression
  ): FullSummary[] {
    if (!confirmedRegression) {
      return [];
    }
    if (
      !confirmedRegression.summary ||
      !confirmedRegression.summary.Clusters ||
      !confirmedRegression.frame
    ) {
      errorMessage(
        'Received invalid ConfirmedRegression payload: missing summary, Clusters, or frame.',
        0,
        {
          countMetricSource: CountMetric.ConfirmedRegressionInvalidPayload,
          source: 'cluster-page',
        }
      );
      return [];
    }
    const frame = confirmedRegression.frame;
    const summaries: FullSummary[] = [];
    confirmedRegression.summary.Clusters.forEach((clusterSummary) => {
      if (clusterSummary) {
        summaries.push({
          summary: clusterSummary,
          frame,
          triage: {
            status: '',
            message: '',
          },
        });
      }
    });
    return summaries;
  }

  private async start() {
    if (this.requestId) {
      errorMessage('There is a pending query already running.');
      return;
    }

    const body = this.createRegressionDetectionRequest();
    this.summaries = [];
    this.requestId = 'pending';
    this.runningStatus = '';

    try {
      const prog = await startRequest('/_/cluster/start', body, {
        pollingIntervalMs: 300,
        onProgressUpdate: (p: progress.SerializedProgress) => {
          this.runningStatus = p.messages.map((msg) => `${msg.key}: ${msg.value}`).join('\n');
        },
        onStart: () => {
          this.spinner!.active = true;
        },
        onSettled: () => {
          this.spinner!.active = false;
        },
      });

      if (prog.status === 'Error') {
        throw new Error(messagesToErrorString(prog.messages));
      }

      this.summaries = this.processConfirmedRegressionResults(prog.results as ConfirmedRegression);
    } catch (error: any) {
      this.catch(error);
    } finally {
      this.requestId = '';
    }
  }
}
