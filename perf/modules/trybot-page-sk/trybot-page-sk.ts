/**
 * @module modules/trybot-page-sk
 * @description <h2><code>trybot-page-sk</code></h2>
 *
 */
import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';
import { toParamSet } from 'common-sk/modules/query';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { errorMessage } from 'elements-sk/errorMessage';
import { ParamSet as CommonSkParamSet } from 'common-sk/modules/query';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { HintableObject } from 'common-sk/modules/hintable';
import { TabSelectedSkEventDetail } from 'elements-sk/tabs-sk/tabs-sk';
import { SpinnerSk } from 'elements-sk/spinner-sk/spinner-sk';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
  QuerySk,
  QuerySkQueryChangeEventDetail,
} from '../../../infra-sk/modules/query-sk/query-sk';

import { QueryCountSk } from '../query-count-sk/query-count-sk';
import {
  ParamSet,
  Commit,
  TryBotRequest,
  TryBotResponse, Params,
} from '../json';
import { CommitDetailPanelSkCommitSelectedDetails } from '../commit-detail-panel-sk/commit-detail-panel-sk';

import '../../../infra-sk/modules/query-sk';
import '../../../infra-sk/modules/paramset-sk';

import '../query-count-sk';
import '../commit-detail-picker-sk';
import '../day-range-sk';

import 'elements-sk/spinner-sk';
import 'elements-sk/tabs-sk';
import 'elements-sk/tabs-panel-sk';

// Only show the first and last N elements of a list, because showing the full
// list would be too much.
const numHeadTail = 10;

export class TrybotPageSk extends ElementSk {
  private queryCount: QueryCountSk | null = null;

  private query: QuerySk | null = null;

  private spinner: SpinnerSk | null = null;

  private results: TryBotResponse | null = null;

  private state: TryBotRequest = {
    kind: 'commit',
    cl: '',
    patch_number: -1,
    commit_number: -1,
    query: '',
  };

  constructor() {
    super(TrybotPageSk.template);
  }

  private static template = (ele: TrybotPageSk) => html`
    <tabs-sk
      @tab-selected-sk=${ele.tabSelected}
      selected=${ele.state.kind === 'commit' ? 0 : 1}
    >
      <button>Commit</button>
      <button>TryBot</button>
    </tabs-sk>
    <tabs-panel-sk>
      <div>
        <h2>Choose which commit to analyze:</h2>
        <commit-detail-picker-sk
          @commit-selected=${ele.commitSelected}
          .selection=${ele.state.commit_number}
          id="commit"
        ></commit-detail-picker-sk>

        <h2
          ?hidden=${ele.state.commit_number === -1}
        >Choose the traces to analyze:</h2>
        <div
          ?hidden=${ele.state.commit_number === -1}
          class=query
          >
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
        <div class=run>
          <button
            ?hidden=${ele.state.commit_number === -1 || ele.state.query === ''}
            @click=${ele.run}
            id=run
            class=action
          >Run</button>
          <spinner-sk id=run-spinner></spinner-sk>
        </div>
      </div>
      <div>
        TryBot Stuff Goes Here
      </div>
    </tabs-panel-sk>
    <div
      class=results
      ?hidden=${ele.results === null}
    >
      <h2>Results</h2>
      <tabs-sk>
        <button>Individual</button>
        <button>By Params</button>
      </tabs-sk>
      <tabs-panel-sk>
        <div>
          <table>
            <tr><th>Index</th><th>StdDev Ratio</th> ${TrybotPageSk.paramKeysAsHeaders(ele)} </tr>
            ${TrybotPageSk.individualResults(ele)}
          </table>
        </div>
        <div></div>
      </tabs-panel-sk>
    </div>
  `;

private static paramKeysAsHeaders = (ele: TrybotPageSk): TemplateResult[] | null => {
  if (!ele.results) {
    return null;
  }
  const keys = Object.keys(ele.results.paramset);
  keys.sort();

  return keys.map((key) => html`<th>${key}</th>`);
}

private static individualResults = (ele: TrybotPageSk): TemplateResult[] | null => {
  if (!ele.results) {
    return null;
  }
  const keys = Object.keys(ele.results.paramset);
  // TODO(jcgregorio) Deduplicate this from here and paramKeysAsHeaders.
  keys.sort();

  let lastParams: Params = {};
  const ret: TemplateResult[] = [];
  ele.results.results!.forEach((r, i) => {
    // Only display the head and tail of the Individual results since the list
    // can be huge.
    if (i > numHeadTail && i < ele.results!.results!.length - numHeadTail) {
      return;
    }

    const keyValueDelta: TemplateResult[] = [];
    keys.forEach((key) => {
      const value = r.params[key];
      if (value) {
        if (value !== lastParams[key]) {
          // Highlight values that have changed.
          keyValueDelta.push(html`<td class=changed>${value}</td>`);
        } else {
          keyValueDelta.push(html`<td>${value}</td>`);
        }
      } else {
        keyValueDelta.push(html`<td></td>`);
      }
    });
    ret.push(html`<tr><td>${i + 1}</td> <td>${r.stddevRatio}</td> ${keyValueDelta}</tr>`);
    lastParams = r.params;
  });
  return ret;
}

connectedCallback(): void {
  super.connectedCallback();
  this._render();
  this.query = this.querySelector('#query');
    this.query!.key_order = window.sk.perf.key_order || [];
    this.queryCount = this.querySelector('#query-count');
    this.spinner = this.querySelector('#run-spinner');

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

private async run() {
  try {
      this.spinner!.active = true;
      const resp = await fetch('/_/trybot/load/', {
        method: 'POST',
        body: JSON.stringify(this.state),
        headers: {
          'Content-Type': 'application/json',
        },
      });
      this.results = await jsonOrThrow(resp);
      // Calculate By Params view here.
      this._render();
  } catch (error) {
    errorMessage(error);
  } finally {
      this.spinner!.active = false;
  }
}

private tabSelected(e: CustomEvent<TabSelectedSkEventDetail>) {
  this.state.kind = e.detail.index === 0 ? 'commit' : 'trybot';
  this.stateHasChanged();
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
    this._render();
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
    this.stateHasChanged();
    this._render();
  }
}

define('trybot-page-sk', TrybotPageSk);
