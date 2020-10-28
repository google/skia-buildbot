/**
 * @module modules/trybot-page-sk
 * @description <h2><code>trybot-page-sk</code></h2>
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { toParamSet } from 'common-sk/modules/query';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { errorMessage } from 'elements-sk/errorMessage';
import { ParamSet as CommonSkParamSet } from 'common-sk/modules/query';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { HintableObject } from 'common-sk/modules/hintable';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
  QuerySk,
  QuerySkQueryChangeEventDetail,
} from '../../../infra-sk/modules/query-sk/query-sk';
import {
  ParamSetSk,
} from '../../../infra-sk/modules/paramset-sk/paramset-sk';
import { QueryCountSk } from '../query-count-sk/query-count-sk';
import {
  ParamSet,
  CommitNumber,
  Commit,
  TryBotRequestKind,
  CL, TryBotRequest,
} from '../json';
import { CommitDetailPanelSkCommitSelectedDetails } from '../commit-detail-panel-sk/commit-detail-panel-sk';

import '../../../infra-sk/modules/query-sk';
import '../../../infra-sk/modules/paramset-sk';

import '../query-count-sk';
import '../commit-detail-picker-sk';
import '../day-range-sk';

import 'elements-sk/tabs-sk';
import 'elements-sk/tabs-panel-sk';

export class TrybotPageSk extends ElementSk {
  private queryCount: QueryCountSk | null = null;

  private query: QuerySk | null = null;

  private summary: ParamSetSk | null = null;

  private state: TryBotRequest = {
    kind: 'trybot',
    cl: '',
    patch_number: -1,
    commit_number: -1,
    query: '',
  };

  constructor() {
    super(TrybotPageSk.template);
  }

  private static template = (ele: TrybotPageSk) => html`
    <tabs-sk>
      <button>Commit</button>
      <button>TryBot</button>
    </tabs-sk>
    <tabs-panel-sk>
      <div>
        <div class=query>
          <query-sk
            id=query
            @query-change=${ele.queryChangeHandler}
            @query-change-delayed=${ele.queryChangeDelayedHandler}
            .current_query=${ele.state.query}
          ></query-sk>
          <div class=query-summary>
            <paramset-sk
              .paramsets=${[toParamSet(ele.state.query)]}
              id=summary>
            </paramset-sk>
            <div class=query-counts>
              Matches: <query-count-sk id=query-count url='/_/count/' @paramset-changed=${
                    ele.paramsetChanged
                  }></query-count-sk>
            </div>
          </div>
        </div>
        <commit-detail-picker-sk
          @commit-selected=${ele.commitSelected}
          .selection=${ele.state.commit_number}
          id="commit"
        ></commit-detail-picker-sk>
      </div>
      <div>
        TryBot Stuff Goes Here
      </div>
    </tabs-panel-sk>

  `;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.query = this.querySelector('#query');
    this.query!.key_order = window.sk.perf.key_order || [];
    this.queryCount = this.querySelector('#query-count');
    this.summary = this.querySelector('#summary');

    // Populate the query element.
    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;

    fetch(`/_/initpage/?tz=${tz}`, {
      method: 'GET',
    })
      .then(jsonOrThrow)
      .then((json) => {
        this.query!.paramset = json.dataframe.paramset;

        // From this point on reflect the state to the URL.
        this.startStateReflector();
      })
      .catch(errorMessage);
  }

  // Call this anytime something in private state is changed. Will be replaced
  // with the real function once stateReflector has been setup.
  // eslint-disable-next-line @typescript-eslint/no-empty-function
  private stateHasChanged = () => {};

  private startStateReflector() {
    this.stateHasChanged = stateReflector(
      () => (this.state as unknown) as HintableObject,
      (state) => {
        this.state = (state as unknown) as TryBotRequest;
        this._render();
      },
    );
  }

  private commitSelected(
    e: CustomEvent<CommitDetailPanelSkCommitSelectedDetails>,
  ) {
    this.state.commit_number = ((e.detail.commit as unknown) as Commit).offset;
    this.stateHasChanged();
  }

  private paramsetChanged(e: CustomEvent<ParamSet>) {
    this.query!.paramset = e.detail as CommonSkParamSet;
  }

  private queryChangeDelayedHandler(
    e: CustomEvent<QuerySkQueryChangeEventDetail>,
  ) {
    this.queryCount!.current_query = e.detail.q;
  }

  private queryChangeHandler(e: CustomEvent<QuerySkQueryChangeEventDetail>) {
    this.state.query = e.detail.q;
    this._render();
    this.stateHasChanged();
  }
}


define('trybot-page-sk', TrybotPageSk);
