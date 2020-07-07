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
  ClusterSummary,
  CommitID,
  RangeRequest,
  CommitDetail,
  ClusterStartResponse,
  ClusterStatus,
  FullSummary,
} from '../json';
import { HintableObject } from 'common-sk/modules/hintable';
import { AlgoSelectAlgoChangeEventDetail } from '../algo-select-sk/algo-select-sk';
import { QuerySkQueryChangeEventDetail } from '../../../infra-sk/modules/query-sk/query-sk';
import { QueryCountSk } from '../query-count-sk/query-count-sk';
import { ClusterSummary2SkOpenKeysEventDetail } from '../cluster-summary2-sk/cluster-summary2-sk';
import { DayRangeSkChangeDetail } from '../day-range-sk/day-range-sk';
import { CommitDetailPickerSk } from '../commit-detail-picker-sk/commit-detail-picker-sk';
import {
  CommitDetailPanelSkCommitSelectedDetails,
  CommitDetailPanelSk,
} from '../commit-detail-panel-sk/commit-detail-panel-sk';

// The state that gets reflected to the URL.
class State {
  begin: number;
  end: number;
  offset: number;
  radius: number;
  query: string;
  k: number;
  algo: ClusterAlgo;
  interesting: number;
  sparse: boolean;

  constructor() {
    this.begin = Math.floor(Date.now() / 1000 - 24 * 60 * 60);
    this.end = Math.floor(Date.now() / 1000);
    this.offset = -1;
    this.radius = window.sk.perf.radius;
    this.query = '';
    this.k = 0;
    this.algo = 'kmeans';
    this.interesting = window.sk.perf.interesting;
    this.sparse = false;
  }
}

interface Range {
  begin: number | null;
  end: number | null;
}

export class ClusterPageSk extends ElementSk {
  private static template = (ele: ClusterPageSk) => html`
    <h2>Commit</h2>
    <h3>Appears in Date Range</h3>
    <div class="day-range-with-spinner">
      <day-range-sk
        id="range"
        @day-range-change=${ele._rangeChange}
        begin=${ele._state.begin}
        end=${ele._state.end}
      ></day-range-sk>
      <spinner-sk ?active=${ele._updating_commits}></spinner-sk>
    </div>
    <h3>Commit</h3>
    <div>
      <commit-detail-picker-sk
        @commit-selected=${ele._commitSelected}
        .selected=${ele._selected_commit_index}
        .details=${ele._cids}
        id="commit"
      ></commit-detail-picker-sk>
    </div>

    <h2>Algorithm</h2>
    <algo-select-sk
      algo=${ele._state.algo}
      @algo-change=${ele._algoChange}
    ></algo-select-sk>

    <h2>Query</h2>
    <div class="query-action">
      <query-sk
        @query-change=${ele._queryChanged}
        .key_order=${window.sk.perf.key_order}
        .paramset=${ele._paramset}
        current_query=${ele._state.query}
      ></query-sk>
      <div id="selections">
        <h3>Selections</h3>
        <paramset-sk
          id="summary"
          .paramsets=${[toParamSet(ele._state.query)]}
        ></paramset-sk>
        <div>
          Matches:
          <query-count-sk
            url="/_/count/"
            current_query=${ele._state.query}
            @paramset-changed=${ele._paramsetChanged}
          ></query-count-sk>
        </div>
        <button
          @click=${ele._start}
          class="action"
          id="start"
          ?disabled=${!!ele._requestId}
        >
          Run
        </button>
        <div>
          <spinner-sk ?active=${!!ele._requestId}></spinner-sk>
          <span>${ele._status}</span>
        </div>
      </div>
    </div>

    <details>
      <summary id="advanced">
        <h2>Advanced</h2>
      </summary>
      <div id="inputs">
        <label>
          K (A value of 0 means the server chooses).
          <input
            type="number"
            min="0"
            max="100"
            .value=${ele._state.k}
            @input=${ele._kChange}
          />
        </label>
        <label>
          Number of commits to include on either side.
          <input
            type="number"
            min="1"
            max="25"
            .value=${ele._state.radius}
            @input=${ele._radiusChange}
          />
        </label>
        <label>
          Clusters are interesting if regression score &gt;= this.
          <input
            type="number"
            min="0"
            max="500"
            .value=${ele._state.interesting}
            @input=${ele._interestingChange}
          />
        </label>
        <checkbox-sk
          ?checked=${ele._state.sparse}
          label="Data is sparse, so only include commits that have data."
          @input=${ele._sparseChange}
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
    <div id="clusters" @open-keys=${ele._openKeys}>
      ${ClusterPageSk._summaryRows(ele)}
    </div>
  `;

  private static _summaryRows = (ele: ClusterPageSk) => {
    const ret = ele._summaries.map(
      (summary) =>
        html`
          <cluster-summary2-sk
            .full_summary=${summary}
            notriage
          ></cluster-summary2-sk>
        `
    );
    if (!ret.length) {
      ret.push(html`
        <p class="info">
          No clusters found.
        </p>
      `);
    }
    return ret;
  };

  private _state = new State();
  private _paramset: ParamSet = {};

  // The computed clusters.
  private _summaries: FullSummary[] = [];

  // The commits to choose from.
  private _cids: CommitDetail[] = [];

  // Which commit is selected.
  private _selected_commit_index: number = -1;

  // The id of the current cluster request. Will be the empty string if
  // there is no pending request.
  private _requestId: string = '';

  // The status of a running request.
  private _status: string = '';

  // True if we are fetching a new list of _cids from the server.
  private _updating_commits: boolean = false;

  // Only update _cids if the date range is different from the last fetch.
  private _lastRange: Range = {
    begin: null,
    end: null,
  };

  // Call this anytime something in private state is changed. Will be replaced
  // with the real function once stateReflector has been setup.
  // tslint:disable-next-line: no-empty
  private _stateHasChanged = () => {};

  private _clusters: HTMLDivElement | null = null;

  constructor() {
    super(ClusterPageSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this._clusters = this.querySelector<HTMLDivElement>('#clusters');

    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
    fetch(`/_/initpage/?tz=${tz}`)
      .then(jsonOrThrow)
      .then((json: FrameResponse) => {
        this._paramset = json.dataframe!.paramset;
        this._render();
      })
      .catch(errorMessage);

    this._stateHasChanged = stateReflector(
      () => (this._state as unknown) as HintableObject,
      (state) => {
        this._state = (state as unknown) as State;
        this._render();
        this._updateCommitSelections();
      }
    );
  }

  _algoChange(e: CustomEvent<AlgoSelectAlgoChangeEventDetail>) {
    this._state.algo = e.detail.algo;
    this._stateHasChanged();
  }

  _kChange(e: InputEvent) {
    this._state.k = +(e.target! as HTMLInputElement).value;
    this._stateHasChanged();
  }

  _radiusChange(e: InputEvent) {
    this._state.radius = +(e.target! as HTMLInputElement).value;
    this._stateHasChanged();
  }

  _interestingChange(e: InputEvent) {
    this._state.interesting = +(e.target! as HTMLInputElement).value;
    this._stateHasChanged();
  }

  _sparseChange(e: InputEvent) {
    this._state.sparse = (e.target! as HTMLInputElement).checked;
    this._stateHasChanged();
  }

  _queryChanged(e: CustomEvent<QuerySkQueryChangeEventDetail>) {
    this._state.query = e.detail.q;
    this._stateHasChanged();
    this._render();
  }

  _paramsetChanged(e: CustomEvent<ParamSet>) {
    this._paramset = e.detail;
    this._render();
  }

  _openKeys(e: CustomEvent<ClusterSummary2SkOpenKeysEventDetail>) {
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

  _rangeChange(e: CustomEvent<DayRangeSkChangeDetail>) {
    this._state.begin = e.detail.begin;
    this._state.end = e.detail.end;
    this._stateHasChanged();
    this._updateCommitSelections();
  }

  _commitSelected(e: CustomEvent<CommitDetailPanelSkCommitSelectedDetails>) {
    this._state.offset = ((e.detail.commit as unknown) as CommitID).offset;
    this._stateHasChanged();
  }

  _updateCommitSelections() {
    if (
      this._lastRange.begin === this._state.begin &&
      this._lastRange.end === this._state.end
    ) {
      return;
    }
    this._lastRange = {
      begin: this._state.begin,
      end: this._state.end,
    };
    const body: RangeRequest = {
      begin: this._state.begin,
      end: this._state.end,
      offset: this._state.offset,
    };
    this._updating_commits = true;
    fetch('/_/cidRange/', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((cids: CommitDetail[]) => {
        this._updating_commits = false;
        cids.reverse();
        this._cids = cids;

        this._selected_commit_index = -1;
        // Look for commit id in this._cids.
        for (let i = 0; i < cids.length; i++) {
          if (
            // TODO(jcgregorio) Fix how go2ts handles nested structs.
            ((cids[i] as unknown) as CommitID).offset === this._state.offset
          ) {
            this._selected_commit_index = i;
            break;
          }
        }

        if (!this._state.begin) {
          this._state.begin = cids[cids.length - 1].ts;
          this._state.end = cids[0].ts;
        }
        this._render();
      })
      .catch((msg) => {
        if (msg) {
          errorMessage(msg, 10000);
        }
        this._updating_commits = false;
        this._render();
      });
  }

  _catch(msg: string) {
    this._requestId = '';
    this._status = '';
    if (msg) {
      errorMessage(msg, 10000);
    }
    this._render();
  }

  _checkClusterRequestStatus(cb: (summaries: ClusterStatus) => void) {
    fetch(`/_/cluster/status/${this._requestId}`)
      .then(jsonOrThrow)
      .then((json) => {
        if (json.state === 'Running') {
          this._status = json.message;
          this._render();
          window.setTimeout(() => this._checkClusterRequestStatus(cb), 300);
        } else {
          if (json.value) {
            cb(json.value);
          }
          this._catch(json.message);
        }
      })
      .catch((msg) => this._catch(msg));
  }

  _start() {
    if (this._requestId) {
      errorMessage('There is a pending query already running.');
      return;
    }
    const body: RegressionDetectionRequest = {
      query: this._state.query,
      step: 0,
      total_queries: 0,
      alert: {
        id: -1,
        display_name: '',
        radius: +this._state.radius,
        query: this._state.query,
        k: +this._state.k,
        algo: this._state.algo,
        interesting: +this._state.interesting,
        sparse: this._state.sparse,
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
        offset: +this._state.offset,
        n: 0,
        end: '',
      },
    };
    this._summaries = [];
    // Set a value for _requestId so the spinner starts, and we don't start
    // another request too soon.
    this._requestId = 'pending';
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
        this._requestId = json.id;
        this._checkClusterRequestStatus((summaries) => {
          this._summaries = [];
          summaries.value!.summary!.Clusters!.forEach((cl) => {
            this._summaries.push({
              summary: cl,
              frame: summaries.value!.frame!,
              triage: {
                status: '',
                message: '',
              },
            });
          });
          this._render();
        });
      })
      .catch((msg) => this._catch(msg));
  }
}

define('cluster-page-sk', ClusterPageSk);
