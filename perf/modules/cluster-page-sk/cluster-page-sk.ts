/**
 * @module module/cluster-page-sk
 * @description <h2><code>cluster-page-sk</code></h2>
 *
 *   The top level element for clustering traces.
 *
 */
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { fromObject, toParamSet } from 'common-sk/modules/query';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { HintableObject } from 'common-sk/modules/hintable';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import 'elements-sk/spinner-sk';
import 'elements-sk/checkbox-sk';
import 'elements-sk/styles/buttons';

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
  RangeRequest,
  Commit,
  ClusterStartResponse,
  ClusterStatus,
  FullSummary,
  RegressionDetectionResponse,
} from '../json';
import { AlgoSelectAlgoChangeEventDetail } from '../algo-select-sk/algo-select-sk';
import { QuerySkQueryChangeEventDetail } from '../../../infra-sk/modules/query-sk/query-sk';
import { ClusterSummary2SkOpenKeysEventDetail } from '../cluster-summary2-sk/cluster-summary2-sk';
import { DayRangeSkChangeDetail } from '../day-range-sk/day-range-sk';
import { CommitDetailPanelSkCommitSelectedDetails } from '../commit-detail-panel-sk/commit-detail-panel-sk';

// The state that gets reflected to the URL.
class State {
  begin: number = Math.floor(Date.now() / 1000 - 24 * 60 * 60);

  end: number = Math.floor(Date.now() / 1000);

  offset: number = -1;

  radius: number = window.sk.perf.radius;

  query: string = '';

  k: number = 0;

  algo: ClusterAlgo = 'kmeans';

  interesting: number = window.sk.perf.interesting;

  sparse: boolean = false;

  constructor() {
    if (window.sk.perf.demo) {
      this.begin = Math.floor(new Date(2020, 4, 1).valueOf() / 1000);
      this.end = Math.floor(new Date(2020, 5, 1).valueOf() / 1000);
    }
  }
}

// The date range over which commits are presented.
interface Range {
  begin: number | null;
  end: number | null;
}

export class ClusterPageSk extends ElementSk {
  // The state to be reflected to the URL.
  private state = new State();

  private paramset: ParamSet = {};

  // The computed clusters.
  private summaries: FullSummary[] = [];

  // The commits to choose from.
  private cids: Commit[] = [];

  // Which commit is selected.
  private selectedCommitIndex: number = -1;

  // The id of the current cluster request. Will be the empty string if
  // there is no pending request.
  private requestId: string = '';

  // The status of a running request.
  private status: string = '';

  // True if we are fetching a new list of _cids from the server.
  private updatingCommits: boolean = false;

  // Only update _cids if the date range is different from the last fetch.
  private lastRange: Range = {
    begin: null,
    end: null,
  };

  constructor() {
    super(ClusterPageSk.template);
  }

  private static template = (ele: ClusterPageSk) => html`
    <h2>Commit</h2>
    <h3>Appears in Date Range</h3>
    <div class="day-range-with-spinner">
      <day-range-sk
        id="range"
        @day-range-change=${ele.rangeChange}
        begin=${ele.state.begin}
        end=${ele.state.end}
      ></day-range-sk>
      <spinner-sk ?active=${ele.updatingCommits}></spinner-sk>
    </div>
    <h3>Commit</h3>
    <div>
      <commit-detail-picker-sk
        @commit-selected=${ele.commitSelected}
        .selected=${ele.selectedCommitIndex}
        .details=${ele.cids}
        id="commit"
      ></commit-detail-picker-sk>
    </div>

    <h2>Algorithm</h2>
    <algo-select-sk
      algo=${ele.state.algo}
      @algo-change=${ele.algoChange}
    ></algo-select-sk>

    <h2>Query</h2>
    <div class="query-action">
      <query-sk
        @query-change=${ele.queryChanged}
        .key_order=${window.sk.perf.key_order}
        .paramset=${ele.paramset}
        current_query=${ele.state.query}
      ></query-sk>
      <div id="selections">
        <h3>Selections</h3>
        <paramset-sk
          id="summary"
          .paramsets=${[toParamSet(ele.state.query)]}
        ></paramset-sk>
        <div>
          Matches:
          <query-count-sk
            url="/_/count/"
            current_query=${ele.state.query}
            @paramset-changed=${ele.paramsetChanged}
          ></query-count-sk>
        </div>
        <button
          @click=${ele.start}
          class="action"
          id="start"
          ?disabled=${!!ele.requestId}
        >
          Run
        </button>
        <div>
          <spinner-sk ?active=${!!ele.requestId}></spinner-sk>
          <span>${ele.status}</span>
        </div>
      </div>
    </div>

    <details>
      <summary id="advanced">
        Advanced
      </summary>
      <div id="inputs">
        <label>
          K (A value of 0 means the server chooses).
          <input .value=${ele.state.k} @input=${ele.kChange} />
        </label>
        <label>
          Number of commits to include on either side.
          <input .value=${ele.state.radius} @input=${ele.radiusChange} />
        </label>
        <label>
          Clusters are interesting if regression score &gt;= this.
          <input
            .value=${ele.state.interesting}
            @input=${ele.interestingChange}
          />
        </label>
        <checkbox-sk
          ?checked=${ele.state.sparse}
          label="Data is sparse, so only include commits that have data."
          @input=${ele.sparseChange}
        ></checkbox-sk>
      </div>
    </details>

    <h2>Results</h2>
    <sort-sk target="clusters">
      <button data-key="clustersize">Cluster Size</button>
      <button data-key="stepregression" data-default="up">Regression</button>
      <button data-key="stepsize">Step Size</button>
      <button data-key="steplse">Least Squares</button>
    </sort-sk>
    <div id="clusters" @open-keys=${ele.openKeys}>
      ${ClusterPageSk._summaryRows(ele)}
    </div>
  `;

  private static _summaryRows = (ele: ClusterPageSk) => {
    const ret = ele.summaries.map(
      (summary) => html`
          <cluster-summary2-sk
            .full_summary=${summary}
            notriage
          ></cluster-summary2-sk>
        `,
    );
    if (!ret.length) {
      ret.push(html`<p class="info"> No clusters found. </p>`);
    }
    return ret;
  };

  connectedCallback(): void {
    super.connectedCallback();
    this._render();

    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
    fetch(`/_/initpage/?tz=${tz}`)
      .then(jsonOrThrow)
      .then((json: FrameResponse) => {
        this.paramset = json.dataframe!.paramset;
        this._render();
      })
      .catch(errorMessage);

    this.stateHasChanged = stateReflector(
      () => (this.state as unknown) as HintableObject,
      (state) => {
        this.state = (state as unknown) as State;
        this._render();
        this.updateCommitSelections();
      },
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

  private rangeChange(e: CustomEvent<DayRangeSkChangeDetail>) {
    this.state.begin = e.detail.begin;
    this.state.end = e.detail.end;
    this.stateHasChanged();
    this.updateCommitSelections();
  }

  private commitSelected(
    e: CustomEvent<CommitDetailPanelSkCommitSelectedDetails>,
  ) {
    this.state.offset = ((e.detail.commit as unknown) as Commit).offset;
    this.stateHasChanged();
  }

  private updateCommitSelections() {
    if (
      this.lastRange.begin === this.state.begin
      && this.lastRange.end === this.state.end
    ) {
      return;
    }
    this.lastRange = {
      begin: this.state.begin,
      end: this.state.end,
    };
    const body: RangeRequest = {
      begin: this.state.begin,
      end: this.state.end,
      offset: this.state.offset,
    };
    this.updatingCommits = true;
    fetch('/_/cidRange/', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((cids: Commit[]) => {
        this.updatingCommits = false;
        cids.reverse();
        this.cids = cids;

        this.selectedCommitIndex = -1;
        // Look for commit id in this._cids.
        for (let i = 0; i < cids.length; i++) {
          if (((cids[i] as unknown) as Commit).offset === this.state.offset) {
            this.selectedCommitIndex = i;
            break;
          }
        }

        if (!this.state.begin) {
          this.state.begin = cids[cids.length - 1].ts;
          this.state.end = cids[0].ts;
        }
        this._render();
      })
      .catch((msg) => {
        if (msg) {
          errorMessage(msg, 10000);
        }
        this.updatingCommits = false;
        this._render();
      });
  }

  private catch(msg: string) {
    this.requestId = '';
    this.status = '';
    if (msg) {
      errorMessage(msg, 10000);
    }
    this._render();
  }

  private checkClusterRequestStatus(
    cb: (summaries: RegressionDetectionResponse)=> void,
  ) {
    fetch(`/_/cluster/status/${this.requestId}`)
      .then(jsonOrThrow)
      .then((json: ClusterStatus) => {
        if (json.state === 'Running') {
          this.status = json.message;
          this._render();
          window.setTimeout(() => this.checkClusterRequestStatus(cb), 300);
        } else {
          if (json.value) {
            cb(json.value);
          }
          this.catch(json.message);
        }
      })
      .catch((msg) => this.catch(msg));
  }

  private start() {
    if (this.requestId) {
      errorMessage('There is a pending query already running.');
      return;
    }
    const body: RegressionDetectionRequest = {
      query: this.state.query,
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
    this._render();
    fetch('/_/cluster/start', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((json: ClusterStartResponse) => {
        this.requestId = json.id;
        this.checkClusterRequestStatus(
          (regressionDetectionResponse: RegressionDetectionResponse) => {
            this.summaries = [];
            regressionDetectionResponse.summary!.Clusters!.forEach(
              (clusterSummary) => {
                this.summaries.push({
                  summary: clusterSummary!,
                  frame: regressionDetectionResponse.frame!,
                  triage: {
                    status: '',
                    message: '',
                  },
                });
              },
            );
            this._render();
          },
        );
      })
      .catch((msg) => this.catch(msg));
  }
}

define('cluster-page-sk', ClusterPageSk);
