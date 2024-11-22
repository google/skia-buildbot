/**
 * @module module/cluster-page-sk
 * @description <h2><code>cluster-page-sk</code></h2>
 *
 *   The top level element for clustering traces.
 *
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { fromObject, toParamSet } from '../../../infra-sk/modules/query';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { HintableObject } from '../../../infra-sk/modules/hintable';
import { SpinnerSk } from '../../../elements-sk/modules/spinner-sk/spinner-sk';
import { errorMessage } from '../errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

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
  RegressionDetectionResponse,
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

export class ClusterPageSk extends ElementSk {
  // The state to be reflected to the URL.
  private state = new State();

  private paramset = ParamSet({});

  // The computed clusters.
  private summaries: FullSummary[] = [];

  // The commits to choose from.
  private cids: Commit[] = [];

  // The id of the current cluster request. Will be the empty string if
  // there is no pending request.
  private requestId: string = '';

  // The status of a running request.
  private status: string = '';

  // The spinner we display when waiting for results.
  private spinner: SpinnerSk | null = null;

  // The text status of the currently running request.
  private runningStatus = '';

  constructor() {
    super(ClusterPageSk.template);
  }

  private static template = (ele: ClusterPageSk) => html`
    <h2>Commit</h2>
    <commit-detail-picker-sk
      @commit-selected=${ele.commitSelected}
      .selection=${ele.state.offset}
      id="commit"></commit-detail-picker-sk>

    <h2>Algorithm</h2>
    <algo-select-sk algo=${ele.state.algo} @algo-change=${ele.algoChange}></algo-select-sk>

    <h2>Query</h2>
    <div class="query-action">
      <query-sk
        @query-change=${ele.queryChanged}
        .key_order=${window.perf.key_order}
        .paramset=${ele.paramset}
        current_query=${ele.state.query}></query-sk>
      <div id="selections">
        <h3>Selections</h3>
        <paramset-sk id="summary" .paramsets=${[toParamSet(ele.state.query)]}></paramset-sk>
        <div>
          Matches:
          <query-count-sk
            url="/_/count/"
            current_query=${ele.state.query}
            @paramset-changed=${ele.paramsetChanged}></query-count-sk>
        </div>
        <button
          @click=${ele.start}
          class="action"
          id="start"
          ?disabled=${!!ele.requestId || ele.state.offset === -1}>
          Run
        </button>
        <div>
          <spinner-sk id="run-spinner"></spinner-sk>
          <span>${ele.status}</span>
        </div>
      </div>
    </div>

    <details>
      <summary id="advanced">Advanced</summary>
      <div id="inputs">
        <label>
          K (A value of 0 means the server chooses).
          <input .value=${ele.state.k.toString()} @input=${ele.kChange} />
        </label>
        <label>
          Number of commits to include on either side.
          <input .value=${ele.state.radius.toString()} @input=${ele.radiusChange} />
        </label>
        <label>
          Clusters are interesting if regression score &gt;= this.
          <input .value=${ele.state.interesting.toString()} @input=${ele.interestingChange} />
        </label>
        <checkbox-sk
          ?checked=${ele.state.sparse}
          label="Data is sparse, so only include commits that have data."
          @input=${ele.sparseChange}></checkbox-sk>
      </div>
    </details>

    <h2>Results</h2>
    <pre class="messages">${ele.runningStatus}</pre>
    <sort-sk target="clusters">
      <button data-key="clustersize">Cluster Size</button>
      <button data-key="stepregression" data-default="up">Regression</button>
      <button data-key="stepsize">Step Size</button>
      <button data-key="steplse">Least Squares</button>
    </sort-sk>
    <div id="clusters" @open-keys=${ele.openKeys}>${ClusterPageSk._summaryRows(ele)}</div>
  `;

  private static _summaryRows = (ele: ClusterPageSk) => {
    const ret = ele.summaries.map(
      (summary) => html`
        <cluster-summary2-sk .full_summary=${summary} notriage></cluster-summary2-sk>
      `
    );
    if (!ret.length) {
      ret.push(html`<p class="info">No clusters found.</p>`);
    }
    return ret;
  };

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.spinner = this.querySelector('#run-spinner');

    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
    fetch(`/_/initpage/?tz=${tz}`)
      .then(jsonOrThrow)
      .then((json: FrameResponse) => {
        this.paramset = ParamSet(json.dataframe!.paramset);
        this._render();
      })
      .catch(errorMessage);

    this.stateHasChanged = stateReflector(
      () => this.state as unknown as HintableObject,
      (state) => {
        this.state = state as unknown as State;
        this._render();
      }
    );
  }

  // Call this anytime something in private state is changed. Will be replaced
  // with the real function once stateReflector has been setup.
  // eslint-disable-next-line @typescript-eslint/no-empty-function
  private stateHasChanged = () => {};

  private algoChange(e: CustomEvent<AlgoSelectAlgoChangeEventDetail>) {
    this.state.algo = e.detail.algo;
    this.stateHasChanged();
  }

  private kChange(e: InputEvent) {
    this.state.k = +(e.target! as HTMLInputElement).value;
    this.stateHasChanged();
  }

  private radiusChange(e: InputEvent) {
    this.state.radius = +(e.target! as HTMLInputElement).value;
    this.stateHasChanged();
  }

  private interestingChange(e: InputEvent) {
    this.state.interesting = +(e.target! as HTMLInputElement).value;
    this.stateHasChanged();
  }

  private sparseChange(e: InputEvent) {
    this.state.sparse = (e.target! as HTMLInputElement).checked;
    this.stateHasChanged();
  }

  private queryChanged(e: CustomEvent<QuerySkQueryChangeEventDetail>) {
    this.state.query = e.detail.q;
    this.stateHasChanged();
    this._render();
  }

  private paramsetChanged(e: CustomEvent<ParamSet>) {
    this.paramset = e.detail;
    this._render();
  }

  private openKeys(e: CustomEvent<ClusterSummary2SkOpenKeysEventDetail>) {
    const query = {
      keys: e.detail.shortcut,
      begin: e.detail.begin,
      end: e.detail.end,
      xbaroffset: e.detail.xbar.offset,
      num_commits: 50,
      request_type: 1,
    };
    window.open(`/e/?${fromObject(query)}`, '_blank');
  }

  private commitSelected(e: CustomEvent<CommitDetailPanelSkCommitSelectedDetails>) {
    this.state.offset = (e.detail.commit as unknown as Commit).offset;
    this.stateHasChanged();
  }

  private catch(msg: string) {
    this.requestId = '';
    this.status = '';
    if (msg) {
      errorMessage(msg);
    }
    this._render();
  }

  private async start() {
    if (this.requestId) {
      errorMessage('There is a pending query already running.');
      return;
    }
    const body: RegressionDetectionRequest = {
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
    this.summaries = [];
    // Set a value for _requestId so the spinner starts, and we don't start
    // another request too soon.
    this.requestId = 'pending';
    this.runningStatus = '';
    this._render();

    try {
      const prog = await startRequest(
        '/_/cluster/start',
        body,
        300,
        this.spinner!,
        (prog: progress.SerializedProgress) => {
          this.runningStatus = prog.messages.map((msg) => `${msg.key}: ${msg.value}`).join('\n');
          this._render();
        }
      );
      if (prog.status === 'Error') {
        throw new Error(messagesToErrorString(prog.messages));
      }

      this.summaries = [];
      const regressionDetectionResponse = prog.results as RegressionDetectionResponse;
      regressionDetectionResponse.summary!.Clusters!.forEach((clusterSummary) => {
        this.summaries.push({
          summary: clusterSummary!,
          frame: regressionDetectionResponse.frame!,
          triage: {
            status: '',
            message: '',
          },
        });
      });
    } catch (error: any) {
      this.catch(error);
    } finally {
      this.requestId = '';
      this._render();
    }
  }
}

define('cluster-page-sk', ClusterPageSk);
