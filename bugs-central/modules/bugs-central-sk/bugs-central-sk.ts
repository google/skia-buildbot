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

import '@google-web-components/google-chart';

import {define} from 'elements-sk/define';
import {html, TemplateResult} from 'lit-html';
import {errorMessage} from 'elements-sk/errorMessage';
import {jsonOrThrow} from 'common-sk/modules/jsonOrThrow';
import {stateReflector} from 'common-sk/modules/stateReflector';

import {$$} from 'common-sk/modules/dom';
import {ElementSk} from '../../../infra-sk/modules/ElementSk';
import {Statement} from 'typescript';
import {HintableObject} from 'common-sk/modules/hintable';

const CLIENT_KEY_DELIMITER = ' > ';

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

const OPEN_CHART_TYPE = 'open'

function getChartOptions(type: string) {
  return JSON.stringify({
    chartArea: {
      top: 15,
      left: 50,
      width: '50%',
      height: '82%',
    },
    hAxis: {
      slantedText: false,
    },
    legend: {
      position: 'bottom',
    },
    type: 'area',
    // isStacked: true,
    series: {
      '3': {
        color: '#64c1f6',
      },
      '2': {
        color: '#64B5F6',
      },
      '1': {
        color: '#1E88E5',
      },
      '0': {
        color: '#0D47A1',
      }
    }
  });
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

  constructor() {
    super(BugsCentralSk.template);

    this._clients_to_counts = {};
    this._clients_map = {};

    this._state = {
      client: '',
      source: '',
      query: '',
    }
  }

  // Make get charts? or just put it in a different eleement?
  private static template = (el: BugsCentralSk) => html`
<h2>${el.getTitle()}</h2>
<br/><br/>
${el.displayOpenIssuesChart()}
<br/><br/>
${el.displayClientsTable()}
`;

  private displayOpenIssuesChart(): TemplateResult {
    return html`
    <div>
      <div>Open Issues</div>
      <google-chart
        id="open-issues-chart"
        options="${getChartOptions(OPEN_CHART_TYPE)}"
        data="/_/get_issues_data?client=${this._state.client}&source=${this._state.source}&query=${this._state.query}&type=${OPEN_CHART_TYPE}">
      </google-chart>
    </div>
   `;
  }

  private displayClientsTable(): TemplateResult {
    const rows = this.displayClientsRows();
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
        <th>P0/P1</th>
        <th>P2</th>
        <th>P3+</th>
        <th>Unassigned</th>
        <th>Total</th>
      </tr>
       ${rows}
    </table>
  `;
  }

  private getTitle(): string {
    if (!this._state.client) {
      return 'Displaying all clients'
    }
    return getClientKey(this._state.client, this._state.source, this._state.query);
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
          <td @click=${() => this._click(clientKey)}>
            <span class=client-link>${clientKey}</span>
            ${clientCounts.query_link
          ? html`<span class=query-link>[<a href="${clientCounts.query_link}" target=_blank>issues query</a>]</span>`
          : ''}
          </td>
          <td>${clientCounts.p0_count + clientCounts.p1_count}</td>
          <td>${clientCounts.p2_count}</td>
          <td>${clientCounts.p3_count + clientCounts.p4_count + clientCounts.p5_count + clientCounts.p6_count}</td>
          <td>${clientCounts.unassigned_count}</td>
          <td>${clientCounts.open_count}</td>
        </tr>
      `);
    }
    return rowsHTML;
  }

  // CLEAN THIS UP!
  private _click(clientKey: string) {
    const tokens = breakupClientKey(clientKey);
    this._state.client = tokens.client ? tokens.client : '';
    this._state.source = tokens.source ? tokens.source : '';
    this._state.query = tokens.query ? tokens.query : '';
    this._stateHasChanged();
    // statehashchanged does not call render during set.
    this._populateClientsAndRender();
    // this._render();
  }

  async _populateClientsAndRender() {
    this._doImpl('/_/get_clients_sources_queries', {}, async (json: ClientsResponse) => {
      this._clients_map = json.clients;
      this._clients_to_counts = {};
      const c = this._state.client;
      let s = this._state.source;
      const q = this._state.query;

      // Client is specified and there is only one source then directly display
      // it's queries.
      if (c && !s && !q) {
        const sources = Object.keys(this._clients_map[c]);
        if (sources.length === 1) {
          this._state.source = sources[0];
          s = this._state.source;
        }
      }

      if (!c) {
        await Promise.all(Object.keys(this._clients_map).map(async (c) => this._clients_to_counts[getClientKey(c, '', '')] = await this._getCounts(c, '', '')));
      } else if (!s) {
        await Promise.all(Object.keys(this._clients_map[c]).map(async (s) => this._clients_to_counts[getClientKey(c, s, '')] = await this._getCounts(c, s, '')));
      } else if (!q) {
        await Promise.all(Object.keys(this._clients_map[c][s]).map(async (q) => this._clients_to_counts[getClientKey(c, s, q)] = await this._getCounts(c, s, q)));
      } else {
        this._clients_to_counts[getClientKey(c, s, q)] = await this._getCounts(c, s, q);
      }

      this._render();  // look at all _renders and see if they make sense or not..
    });
  }

  // Something will be wrong here.
  private startStateReflector() {
    this._stateHasChanged = stateReflector(
      () => (this._state as unknown) as HintableObject,
      (state) => {
        this._state = (state as unknown) as State;
        this._populateClientsAndRender();
        // this._render();
      },
    );
  }

  // Call this anytime something in private state is changed. Will be replaced
  // with the real function once stateReflector has been setup.
  // eslint-disable-next-line @typescript-eslint/no-empty-function
  private _stateHasChanged = () => {};

  async connectedCallback() {
    super.connectedCallback();

    // From this point on reflect the state to the URL.
    this.startStateReflector();

    this._populateClientsAndRender();
    // this._render();
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
      console.log(msg);
      msg.resp.text().then(errorMessage);
    });
  }

  // Await and async.
  async _getCounts(client: string, source: string, query: string) {
    // Parse params...
    const detail = {
      'client': client,
      'source': source,
      'query': query,
    };
    let countsData = {} as CountsData;
    await this._doImpl('/_/get_issue_counts', detail, (json: CountsData) => {
      countsData = json
    });
    console.log("XXXXXXXX")
    console.log(countsData)
    return countsData;
  }
}

define('bugs-central-sk', BugsCentralSk);
