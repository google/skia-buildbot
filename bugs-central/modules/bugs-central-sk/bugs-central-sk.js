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

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { errorMessage } from 'elements-sk/errorMessage';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { stateReflector } from 'common-sk/modules/stateReflector';

// import '../display-bugs-central-sk';
// import '../enter-bugs-central-sk';

import { $$ } from 'common-sk/modules/dom';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

const CLIENT_KEY_DELIMITER = ' > ';

// Make this take state instead.
function getClientKey(c, s, q) {
  if (!c) {
    return '';
  } else if (!s) {
    return `${c}`;
  } else if (!q) {
    return `${c}${CLIENT_KEY_DELIMITER}${s}`;
  } else {
    return `${c}${CLIENT_KEY_DELIMITER}${s}${CLIENT_KEY_DELIMITER}"${q}"`;
  }
}

function breakupClientKey(clientKey) {
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


// <enter-bugs-central-sk .autorollers=${ele._autorollers}></enter-bugs-central-sk>
// <display-bugs-central-sk .statuses=${ele._statuses}></display-bugs-central-sk>

const template = (ele) => html`
<h2>${getTitle(ele)}</h2>
<br/>
Open Count: ${ele._open_count}
<br/>
Unassigned Count: ${ele._unassigned_count}
<br/><br/>
${displayClientsTable(ele)}
`;

function getTitle(ele) {
  if (!ele._state.client) {
    return 'Displaying all clients'
  }
  return getClientKey(ele._state.client, ele._state.source, ele._state.query);
}

function displayClientsRows(ele) {
  const rowsHTML = [];
  const clientKeys = Object.keys(ele._clients_to_counts);
  clientKeys.sort();
  for (let i=0; i<clientKeys.length; i++) {
    const clientKey = clientKeys[i];
    const tokens = breakupClientKey(clientKey);
    const c = tokens.client;
    const s = tokens.source;
    const q = tokens.query;
    rowsHTML.push(html`
      <tr>
        <td class=client-link @click=${() => ele._click(clientKey)}>${clientKey}</td>
        <td>${ele._clients_to_counts[clientKey].open_count}</td>
        <td>${ele._clients_to_counts[clientKey].unassigned_count}</td>
      </tr>
    `);
  }
  return rowsHTML;
}

function displayClientsTable(ele) {
  const rows = displayClientsRows(ele);
  return html`
    <table class=client-counts>
      <tr>
        <th>Client</th>
        <th>Open</th>
        <th>Unassigned</th>
      </tr>
       ${rows}
    </table>
  `;
}

define('bugs-central-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._open_count = 0;
    this._unassigned_count = 0;

    this._clients_to_counts = {};
    this._clients_map = {};

    this._state = {
      client: '',
      source: '',
      query: '',
    }
  }

  // CLEAN THIS UP!
  _click(clientKey) {
    const tokens = breakupClientKey(clientKey);
    this._state.client = tokens.client ? tokens.client : '';
    this._state.source = tokens.source ? tokens.source : '';
    this._state.query = tokens.query ? tokens.query : '';
    this._stateHasChanged();
    // statehashchanged does not call render during set.
    this._populateClients();
   //  this._render();
  }

  async _populateClients() {
    this._doImpl('/_/get_clients_sources_queries', {}, async (json) => {
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
          await Promise.all(Object.keys(this._clients_map).map(async (c) => this._clients_to_counts[getClientKey(c)] = await this._getCounts(c)));
        } else if (!s) {
          await Promise.all(Object.keys(this._clients_map[c]).map(async (s) => this._clients_to_counts[getClientKey(c, s)] = await this._getCounts(c, s)));
        } else if (!q) {
          await Promise.all(Object.keys(this._clients_map[c][s]).map(async (q) => this._clients_to_counts[getClientKey(c, s, q)] = await this._getCounts(c, s, q)));
        } else {
          this._clients_to_counts[getClientKey(c, s, q)] = await this._getCounts(c, s, q);
        }

        this._render();  // look at all _renders and see if they make sense or not..
    });
  }

  async connectedCallback() {
    super.connectedCallback();

    this._stateHasChanged = stateReflector(
      /* getState */ () => this._state,
      /* setState */ (state) => {
        this._state = state;
        this._populateClients();
        this._render();
      },
    );

    this._populateClients();

    let { open_count, unassigned_count } = await this._getCounts();
    this._render();
  }

  // Common work done for all fetch requests.
  async _doImpl(url, detail, action) {
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
  async _getCounts(client, source, query) {
    // Parse params...
    const detail = {
      'client': client,
      'source': source,
      'query': query,
    };
    let open_count = 0;
    let unassigned_count = 0;
    await this._doImpl('/_/get_issue_counts', detail, (json) => {
        this._open_count = json.open_count;
        open_count = json.open_count;
        this._unassigned_count = json.unassigned_count;
        unassigned_count = json.unassigned_count;
    });
    return {open_count, unassigned_count};
  }
});
