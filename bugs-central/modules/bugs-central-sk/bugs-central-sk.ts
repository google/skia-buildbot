/**
 * @module bugs-central-sk
 * @description <h2><code>bugs-central-sk</code></h2>
 *
 * <p>
 *   Displays the enter-bugs-central-sk and display-bugs-central-sk elements.
 *   Handles calls to the backend from events originating from those elements.
 * </p>
 *
 */

import {define} from 'elements-sk/define';
import {html, TemplateResult} from 'lit-html';
import {errorMessage} from 'elements-sk/errorMessage';
import {jsonOrThrow} from 'common-sk/modules/jsonOrThrow';
import {stateReflector} from 'common-sk/modules/stateReflector';

import 'elements-sk/spinner-sk';
import '../bugs-chart-sk'

import {ChartType} from '../bugs-chart-sk/bugs-chart-sk';
import {$$} from 'common-sk/modules/dom';
import {ElementSk} from '../../../infra-sk/modules/ElementSk';
import {Statement} from 'typescript';
import {HintableObject} from 'common-sk/modules/hintable';

const CLIENT_KEY_DELIMITER = ' > ';

const SKIA_SLO_DOC = 'https://docs.google.com/document/d/1OgpX1KDDq3YkHzRJjqRHSPJ9CJ8hH0RTvMAApKVxwm8/edit';

function getClientKey(c: string, s: string, q: string) {
  if (!c) {
    return '';
  } else if (!s) {
    return `${c}`;
  } else if (!q) {
    return `${c}${CLIENT_KEY_DELIMITER}${s}`;
  } else {
    return `${c}${CLIENT_KEY_DELIMITER}${s}${CLIENT_KEY_DELIMITER}${q}`;
  }
}

function breakupClientKey(clientKey: string) {
  const ret = {
    client: '',
    source: '',
    query: '',
  }
  const tokens = clientKey.split(CLIENT_KEY_DELIMITER);
  if (tokens.length === 0) {
    // Leave all values blank.
  } else if (tokens.length === 1) {
    ret.client = tokens[0];
  } else if (tokens.length === 2) {
    ret.client = tokens[0];
    ret.source = tokens[1];
  } else if (tokens.length === 3) {
    ret.client = tokens[0];
    ret.source = tokens[1];
    ret.query = tokens[2];
  }
  return ret;
}

declare interface CountsData {
  open_count: number,
  unassigned_count: number,

  p0_count: number,
  p1_count: number,
  p2_count: number,
  p3_count: number,
  p4_count: number,
  p5_count: number,
  p6_count: number,

  p0_slo_count: number,
  p1_slo_count: number,
  p2_slo_count: number,
  p3_slo_count: number,

  query_link: string,
}

// State is reflected to the URL via stateReflector.
declare interface State {
  client: string,
  source: string,
  query: string,
}

declare interface ClientsResponse {
  clients: Record<string, Record<string, Record<string, boolean>>>,
}

export class BugsCentralSk extends ElementSk {
  private _clients_to_counts: Record<string, CountsData>;
  private _clients_map: Record<string, Record<string, Record<string, boolean>>>;
  private _state: State;
  private _updatingData: Boolean;

  constructor() {
    super(BugsCentralSk.template);

    this._clients_to_counts = {};
    this._clients_map = {};

    this._state = {
      client: '',
      source: '',
      query: '',
    }
    this._updatingData = true;
  }

  // Call this anytime something in private state is changed. Will be replaced
  // with the real function once stateReflector has been setup.
  // eslint-disable-next-line @typescript-eslint/no-empty-function
  private _stateHasChanged = () => {};

  async connectedCallback() {
    super.connectedCallback();

    // Populate map of clients to sources to queries.
    await this._doImpl('/_/get_clients_sources_queries', {}, async (json: ClientsResponse) => {
      this._clients_map = json.clients;
      console.log("GOT THIS!!")
      console.log(json.clients)
      console.log(this._clients_map)
    });

    // From this point on reflect the state to the URL.
    this.startStateReflector();

    this._updatingData = true
    await this._populateCountsAndRender();
    this._updatingData = false
    this._render();
  }

  /** @prop state - The state of the element. */
  get state(): State {
    return this._state;
  }

  set state(state: State) {
    this._state = state;
  }

  private static template = (el: BugsCentralSk) => html`
<h2>${el.getTitle()}</h2>
<spinner-sk ?active=${el._updatingData}></spinner-sk>
<br/><br/>
<div class="charts-container">
  <div class="chart-div">
    <bugs-chart-sk chart_type=${ChartType.OPEN}
                   chart_title='Bug Count'
                   client=${el._state.client}
                   source=${el._state.source}
                   query=${el._state.query}>
    </bugs-chart-sk>
  </div>
  <div class="chart-div">
    <bugs-chart-sk chart_type='${ChartType.SLO}'
                   chart_title='SLO Violations'
                   client=${el._state.client}
                   source=${el._state.source}
                   query=${el._state.query}>
    </bugs-chart-sk>
  </div>
</div>
<br/><br/>
${el.displayClientsTable()}
`;

  private displayClientsTable(): TemplateResult {
    return html`
    <table class=client-counts>
      <colgroup>
        <col span="1" style="width: 50%">
        <col span="1" style="width: 10%">
        <col span="1" style="width: 10%">
        <col span="1" style="width: 10%">
        <col span="1" style="width: 10%">
        <col span="1" style="width: 10%">
      </colgroup>
      <tr>
        <th>Client</th>
        <th>P0/P1 <span class="small">[<a href="SKIA_SLO_DOC">SLO</a>]</span></th>
        <th>P2 <span class="small">[<a href="SKIA_SLO_DOC">SLO</a>]</span></th>
        <th>P3+ <span class="small">[<a href="SKIA_SLO_DOC">SLO</a>]</span></th>
        <th>Unassigned</th>
        <th>Total</th>
      </tr>
       ${this.displayClientsRows()}
    </table>
  `;
  }

  private getTitle(): TemplateResult {
    if (!this._state.client) {
      return html`Displaying all clients`;
    }
    const clientKey = getClientKey(this._state.client, this._state.source, this._state.query);
    const clientCounts = this._clients_to_counts[clientKey];
    if (clientCounts && clientCounts.query_link) {
      return html`
        ${clientKey}
        <span class=query-link>[<a href="${clientCounts.query_link}" target=_blank>issues query</a>]</span>
      `;
    }
    return html`${clientKey}`;
  }

  private displayClientsRows(): TemplateResult[] {
    const rowsHTML = [];
    const clientKeys = Object.keys(this._clients_to_counts);
    clientKeys.sort();
    for (let i = 0; i < clientKeys.length; i++) {
      const clientKey = clientKeys[i];
      const tokens = breakupClientKey(clientKey);
      const c = tokens.client;
      const s = tokens.source;
      const q = tokens.query;
      const clientCounts = this._clients_to_counts[clientKey];
      rowsHTML.push(html`
        <tr>
        ${clientCounts.query_link
          ? html`<td><span class=client-name>${clientKey}</span></td>`
          : html`
            <td @click=${() => this._click(clientKey)}>
              <span class=client-link>${clientKey}</span>
            </td>
          `}
          <td>
            ${clientCounts.p0_count + clientCounts.p1_count}
            ${clientCounts.p0_slo_count + clientCounts.p1_slo_count > 0
          ? html`<span class="small"> [${clientCounts.p0_slo_count + clientCounts.p1_slo_count}]</span>`
          : ''}
          </td>
          <td>
            ${clientCounts.p2_count}
            ${clientCounts.p2_slo_count > 0
          ? html`<span class="small"> [${clientCounts.p2_slo_count}]</span>`
          : ''}
          </td>
          <td>
            ${clientCounts.p3_count + clientCounts.p4_count + clientCounts.p5_count + clientCounts.p6_count}
            ${clientCounts.p3_slo_count > 0
          ? html`<span class="small"> [${clientCounts.p3_slo_count}]</span>`
          : ''}
          </td>
          <td>
            ${clientCounts.unassigned_count}
          </td>
          <td>
          ${clientCounts.query_link
          ? html`<span class=query-link><a href="${clientCounts.query_link}" target=_blank>${clientCounts.open_count}</a></span>`
          : html`${clientCounts.open_count}`}
            ${clientCounts.p0_slo_count + clientCounts.p1_slo_count + clientCounts.p2_slo_count + clientCounts.p3_slo_count > 0
          ? html`<span class="small"> [${clientCounts.p0_slo_count + clientCounts.p1_slo_count + clientCounts.p2_slo_count + clientCounts.p3_slo_count}]</span>`
          : ''}
          </td>
        </tr>
      `);
    }
    return rowsHTML;
  }

  private _click(clientKey: string) {
    const tokens = breakupClientKey(clientKey);
    this._state.client = tokens.client ? tokens.client : '';
    this._state.source = tokens.source ? tokens.source : '';
    this._state.query = tokens.query ? tokens.query : '';
    this._stateHasChanged();
    this._populateCountsAndRender();
  }

  // If client is specified and there is only one source then directly display
  // it's queries. If there is only one query available then directly set it
  // on the status. This saves unnecessary clicks for users.
  //
  // Eg: When a user clicks on 'Android' the UI would show 'Android>Buganizer'.
  // Clicking on that would then display 'Android>Buganizer>query'. Instead of these
  // unnecessary clicks, this function directly displays 'Android>Buganizer>query'
  // when 'Android' is clicked.
  private addExtraInformationToState(state: State): boolean {
    let stateUpdated = false;
    if (state.client && !state.source && !state.query) {
      const sources = Object.keys(this._clients_map[state.client as string]);
      if (sources.length === 1) {
        state.source = sources[0];
        stateUpdated = true;
        const queries = Object.keys(this._clients_map[state.client as string][state.source]);
        if (queries.length === 1) {
          state.query = queries[0];
          stateUpdated = true;
        }
      }
    }
    return stateUpdated;
  }

  private startStateReflector() {
    this._stateHasChanged = stateReflector(
      /* getState */() => {
        this.addExtraInformationToState(this._state)
        return (this._state as unknown) as HintableObject;
      },
      /* setState */(newState) => {
        this._state = (newState as unknown) as State;
        const stateUpdated = this.addExtraInformationToState(this._state);
        if (stateUpdated) {
          this._stateHasChanged();
        }
        this._populateCountsAndRender();
      },
    );
  }

  // Common work done for all fetch requests.
  async _doImpl(url: string, detail: any, action: (json: any) => void) {
    await fetch(url, {
      body: JSON.stringify(detail),
      headers: {
        'content-type': 'application/json',
      },
      credentials: 'include',
      method: 'POST',
    }).then(jsonOrThrow).then((json) => {
      action(json);
      //this._render();
    }).catch((msg) => {
      console.error(msg)
      // msg.resp.text().then(errorMessage);
    });
  }

  async _getCounts(client: string, source: string, query: string) {
    const detail = {
      'client': client,
      'source': source,
      'query': query,
    };
    let countsData = {} as CountsData;
    console.log("CALLING GET COUNTS")
    await this._doImpl('/_/get_issue_counts', detail, (json: CountsData) => {
      console.log("IN HERE")
      console.log(json)
      countsData = json
    });
    console.log("RETURNING")
    console.log(countsData)
    return countsData;
  }

  async _populateCountsAndRender() {
    this._clients_to_counts = {};
    const c = this._state.client;
    let s = this._state.source;
    const q = this._state.query;
    console.log('here')
    console.log(this._clients_map)
    console.log(this._clients_to_counts)

    if (!c) {
      await Promise.all(Object.keys(this._clients_map).map(async (c) => this._clients_to_counts[getClientKey(c, '', '')] = await this._getCounts(c, '', '')));
    } else if (!s) {
      await Promise.all(Object.keys(this._clients_map[c]).map(async (s) => this._clients_to_counts[getClientKey(c, s, '')] = await this._getCounts(c, s, '')));
    } else if (!q) {
      await Promise.all(Object.keys(this._clients_map[c][s]).map(async (q) => this._clients_to_counts[getClientKey(c, s, q)] = await this._getCounts(c, s, q)));
    } else {
      this._clients_to_counts[getClientKey(c, s, q)] = await this._getCounts(c, s, q);
    }
    console.log("DONE")
    console.log(this._clients_to_counts)
    this._render();
  }
}

define('bugs-central-sk', BugsCentralSk);
