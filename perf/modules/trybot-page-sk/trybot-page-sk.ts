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
} from '../json';

import '../../../infra-sk/modules/query-sk';
import '../../../infra-sk/modules/paramset-sk';

import '../query-count-sk';
import '../commit-detail-picker-sk';
import '../day-range-sk';


export class TrybotPageSk extends ElementSk {
  private queryCount: QueryCountSk | null = null;

  private query: QuerySk | null = null;

  private summary: ParamSetSk | null = null;

  constructor() {
    super(TrybotPageSk.template);
  }

  private static template = (ele: TrybotPageSk) => html`
    <query-sk
    id=query
      @query-change=${ele.queryChangeHandler}
      @query-change-delayed=${ele.queryChangeDelayedHandler}
    ></query-sk>
    <paramset-sk id=summary></paramset-sk>
    <div class=query-counts>
      Matches: <query-count-sk id=query-count url='/_/count/' @paramset-changed=${
              ele.paramsetChanged
            }>
            </query-count-sk>
    </div>
    <div class="day-range-with-spinner">
      <day-range-sk
        id="range"
        @day-range-change=${ele.rangeChange}
        begin=${ele.state.begin}
        end=${ele.state.end}
      ></day-range-sk>
      <spinner-sk ?active=${ele.updatingCommits}></spinner-sk>
    </div>
    <commit-detail-picker-sk
    @commit-selected=${ele.commitSelected}
        .selected=${ele.selectedCommitIndex}
        .details=${ele.cids}
        id="commit"
    ></commit-detail-picker-sk>
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
        //        this.startStateReflector();
      })
      .catch(errorMessage);
  }

  private rangeChange(e: CustomEvent<DayRangeSkChangeDetail>) {
    this.state.begin = e.detail.begin;
    this.state.end = e.detail.end;
    this.stateHasChanged();
    this.updateCommitSelections();
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

  private paramsetChanged(e: CustomEvent<ParamSet>) {
    this.query!.paramset = e.detail as CommonSkParamSet;
  }

  private queryChangeDelayedHandler(
    e: CustomEvent<QuerySkQueryChangeEventDetail>,
  ) {
    this.queryCount!.current_query = e.detail.q;
  }

  private queryChangeHandler(e: CustomEvent<QuerySkQueryChangeEventDetail>) {
    const query = e.detail.q;
    this.summary!.paramsets = [toParamSet(query)];
  }
}


define('trybot-page-sk', TrybotPageSk);
