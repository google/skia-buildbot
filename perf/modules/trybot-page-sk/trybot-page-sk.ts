/**
 * @module modules/trybot-page-sk
 * @description <h2><code>trybot-page-sk</code></h2>
 *
 * This page allows the user to select either a CL or an existing commit in the
 * repo to analyze looking for regressions.
 */
import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';
import { toParamSet } from 'common-sk/modules/query';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { errorMessage } from 'elements-sk/errorMessage';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { HintableObject } from 'common-sk/modules/hintable';
import { TabSelectedSkEventDetail } from 'elements-sk/tabs-sk/tabs-sk';
import { SpinnerSk } from 'elements-sk/spinner-sk/spinner-sk';
import { byParams, AveForParam } from '../trybot/calcs';
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
  TryBotResponse,
  Params,
} from '../json';
import { CommitDetailPanelSkCommitSelectedDetails } from '../commit-detail-panel-sk/commit-detail-panel-sk';

import '../../../infra-sk/modules/query-sk';
import '../../../infra-sk/modules/paramset-sk';

import '../query-count-sk';
import '../commit-detail-picker-sk';
import '../day-range-sk';
import '../plot-simple-sk';
import '../window/window';

import 'elements-sk/spinner-sk';
import 'elements-sk/tabs-sk';
import 'elements-sk/tabs-panel-sk';
import 'elements-sk/icon/timeline-icon-sk';
import { PlotSimpleSk, PlotSimpleSkTraceEventDetails } from '../plot-simple-sk/plot-simple-sk';
import { addParamsToParamSet, fromKey, makeKey } from '../paramtools';
import { ParamSetSk } from '../../../infra-sk/modules/paramset-sk/paramset-sk';
import { startRequest } from '../progress/progress';

// Number of elements of a long lists head and tail to display.
const numHeadTail = 10;

// The maximum number of traces to plot in the By Params results.
const maxByParamsPlot = 10;

export class TrybotPageSk extends ElementSk {
  private queryCount: QueryCountSk | null = null;

  private query: QuerySk | null = null;

  private spinner: SpinnerSk | null = null;

  private results: TryBotResponse | null = null;

  private individualPlot: PlotSimpleSk | null = null

  private byParamsPlot: PlotSimpleSk | null = null

  private byParams: AveForParam[] = [];

  private byParamsTraceID: HTMLParagraphElement | null = null;

  private byParamsParamSet: ParamSetSk | null = null;

  private state: TryBotRequest = {
    kind: 'commit',
    cl: '',
    patch_number: -1,
    commit_number: -1,
    query: '',
  };

  private displayedTrace: boolean = false;

  private displayedByParamsTrace: boolean = false;

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
            @query-change=${ele.queryChange}
            @query-change-delayed=${ele.queryChangeDelayed}
            .current_query=${ele.state.query}
          ></query-sk>
          <div class=query-summary>
            <paramset-sk
              .paramsets=${[toParamSet(ele.state.query)]}
              id=summary>
            </paramset-sk>
            <div class=query-counts>
              Matches:
              <query-count-sk
                id=query-count
                url='/_/count/'
                @paramset-changed=${ele.paramsetChanged}>
              </query-count-sk>
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
            <tr>
              <th>Index</th>
              <th title="How many standard deviations this value is from the median.">StdDevs</th>
              <th>Plot</th>
              ${TrybotPageSk.paramKeysAsHeaders(ele)}
            </tr>
            ${TrybotPageSk.individualResults(ele)}
          </table>
          <p class=tiny>Hold CTRL to add multiple traces to the graph.</p>
          <p class=tiny>〃- The value is the same as the trace above it.</p>
          <p class=tiny>∅ - The key doesn't appear on this trace.</p>
          <plot-simple-sk
            id=individual-plot
            ?hidden=${!ele.displayedTrace}
            width="800"
            height="250"
          ></plot-simple-sk>
        </div>
        <div>
          <table>
            <tr>
              <th>Index</th>
              <th>Plot</th>
              <th>Param</th>
              <th title="How many standard deviations this value is from the median.">StdDevs</th>
              <th>N</th>
              <th>High</th>
              <th>Low</th>
            </tr>
            ${TrybotPageSk.byParamsResults(ele)}
          </table>
          <p class=tiny>Hold CTRL to add multiple groups of traces to the graph.</p>
          <plot-simple-sk
            id=by-params-plot
            ?hidden=${!ele.displayedByParamsTrace}
            width="800"
            height="250"
            @trace_focused=${ele.byParamsTraceFocused}
          ></plot-simple-sk>
          <div
            ?hidden=${!ele.displayedByParamsTrace}
          >
            <div
              id=by-params-traceid-container
            ></p><b>TraceID:</b><span id=by-params-traceid></span></div>
            <paramset-sk
              id=by-params-paramset>
            </paramset-sk>
          </div>
        </div>
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
      // Only display the head and tail of the Individual results.
      if (i > numHeadTail && i < ele.results!.results!.length - numHeadTail) {
        return;
      }

      const keyValueDelta: TemplateResult[] = [];
      keys.forEach((key) => {
        const value = r.params[key];
        if (value) {
          if (value !== lastParams[key] || i === 0) {
            // Highlight values that have changed, but not the first row.
            keyValueDelta.push(html`<td>${value}</td>`);
          } else {
            keyValueDelta.push(html`<td>〃</td>`);
          }
        } else {
          keyValueDelta.push(html`<td title="Does not exists on this trace.">∅</td>`);
        }
      });
      ret.push(html`<tr><td>${i + 1}</td> <td>${r.stddevRatio}</td> <td class=link @click=${(e: MouseEvent) => ele.plotIndividualTrace(e, i)}><timeline-icon-sk></timeline-icon-sk></td> ${keyValueDelta}</tr>`);
      lastParams = r.params;
    });
    return ret;
  }

  private static byParamsResults = (ele: TrybotPageSk): TemplateResult[] | null => {
    if (!ele.byParams) {
      return null;
    }
    const ret: TemplateResult[] = [];

    ele.byParams.forEach((b, i) => {
      // Only display the head and tail of the byParams results.
      if (i > numHeadTail && i < ele.byParams.length - numHeadTail) {
        return;
      }
      ret.push(html`<tr>
        <td>${i + 1}</td>
        <td class=link @click=${(e: MouseEvent) => ele.plotByParamsTraces(e, i)}><timeline-icon-sk></timeline-icon-sk></td>
        <td>${b.keyValue}</td>
        <td>${b.aveStdDevRatio}</td>
        <td>${b.n}</td>
        <td>${b.high}</td>
        <td>${b.low}</td>
      </tr>`);
    });
    return ret;
  }

  async connectedCallback(): Promise<void> {
    super.connectedCallback();
    this._render();
    this.query = this.querySelector('#query');
    this.query!.key_order = window.sk.perf.key_order || [];
    this.queryCount = this.querySelector('#query-count');
    this.spinner = this.querySelector('#run-spinner');
    this.individualPlot = this.querySelector('#individual-plot');
    this.byParamsPlot = this.querySelector('#by-params-plot');
    this.byParamsTraceID = this.querySelector('#by-params-traceid');
    this.byParamsParamSet = this.querySelector('#by-params-paramset');

    // Populate the query element.
    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;

    try {
      const resp = await fetch(`/_/initpage/?tz=${tz}`, {
        method: 'GET',
      });
      const json = await jsonOrThrow(resp);
      this.query!.paramset = json.dataframe.paramset;

      // From this point on reflect the state to the URL.
      this.startStateReflector();
    } catch (error) {
      errorMessage(error);
    }
  }

  private async run() {
    this.displayedTrace = false;
    this.displayedByParamsTrace = false;
    this.byParamsTraceID!.innerText = '';
    this._render();
    try {
      const prog = await startRequest('/_/trybot/load/', this.state, 200, this.spinner!, null);
      if (prog.status === 'Finished') {
        this.results = prog.results! as TryBotResponse;
        this.byParams = byParams(this.results!);
        this._render();
      } else {
        // TODO(jcgregorio) Add a utility func for this.
        throw new Error(prog.messages?.filter((msg) => msg?.key === 'Error').map((msg) => `${msg?.key}: ${msg?.value}`).join(''));
      }
    } catch (error) {
      errorMessage(error, 0);
    }
  }

  private getLabels() {
    return this.results!.header!.map((colHeader) => new Date(colHeader!.timestamp * 1000));
  }

  private addZero(lines: {[key: string]: number[]|null}, n: number) {
    lines.special_zero = new Array(n).fill(0);
  }

  private plotIndividualTrace(e: MouseEvent, i: number) {
    const result = this.results!.results![i];
    const params = result.params;

    const lines: {[key: string]: number[]|null} = {};
    lines[makeKey(params)] = result.values;

    this.addZero(lines, result.values!.length);
    if (!e.ctrlKey) {
      this.individualPlot!.removeAll();
    }
    this.individualPlot!.addLines(lines, this.getLabels());
    this.displayedTrace = true;
    this._render();
  }

  private plotByParamsTraces(e: MouseEvent, i: number) {
    // Pick out the array of traces that match the selected key=value.
    const result = this.byParams[i];
    const keyValue = result.keyValue;
    const [key, value] = keyValue.split('=');
    const aveStdDevRatio = result.aveStdDevRatio;

    let matches = this.results!.results!.filter((r) => r.params[key] === value);
    if (aveStdDevRatio >= 0) {
      matches = matches.filter((r) => r.stddevRatio >= 0);
      matches.sort((a, b) => b.stddevRatio - a.stddevRatio);
    } else {
      matches = matches.filter((r) => r.stddevRatio < 0);
      matches.sort((a, b) => a.stddevRatio - b.stddevRatio);
    }

    // Truncate the list.
    matches = matches.slice(0, maxByParamsPlot);

    const lines: {[key: string]: number[]|null} = {};
    matches.forEach((r) => {
      lines[makeKey(r.params)] = r.values;
    });
    if (matches) {
      this.addZero(lines, matches[0].values!.length);
    }

    if (!e.ctrlKey) {
      this.byParamsPlot!.removeAll();
    }
    this.byParamsPlot!.addLines(lines, this.getLabels());

    const ps: ParamSet = {};
    this.byParamsPlot!.getLineNames().forEach((traceName) => {
      addParamsToParamSet(ps, fromKey(traceName));
    });
    this.byParamsParamSet!.paramsets = [ps];

    this.displayedByParamsTrace = true;
    this._render();
  }

  private byParamsTraceFocused(e: CustomEvent<PlotSimpleSkTraceEventDetails>) {
    this.byParamsTraceID!.innerText = e.detail.name;
    this.byParamsParamSet!.highlight = fromKey(e.detail.name);
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
    // The query-count-sk element returned a new paramset for the given query.
    this.query!.paramset = e.detail;
  }

  private queryChangeDelayed(
    e: CustomEvent<QuerySkQueryChangeEventDetail>,
  ) {
    // Pass to queryCount so it can update the number of traces that match the
    // query.
    this.queryCount!.current_query = e.detail.q;
  }

  private queryChange(e: CustomEvent<QuerySkQueryChangeEventDetail>) {
    this.state.query = e.detail.q;
    this.stateHasChanged();
    this._render();
  }
}

define('trybot-page-sk', TrybotPageSk);
