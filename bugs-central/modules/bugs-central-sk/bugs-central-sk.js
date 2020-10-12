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

// <enter-bugs-central-sk .autorollers=${ele._autorollers}></enter-bugs-central-sk>
// <display-bugs-central-sk .statuses=${ele._statuses}></display-bugs-central-sk>

const template = (ele) => html`
${ele._open_count}
<br/>
${ele._unassigned_count}
`;

define('bugs-central-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._open_count = 0;
    this._unassigned_count = 0;

    this._clientToSourceToQueries = {};

    this._state = {
      client: '',
      source: '',
      query: '',
    }
  }

  async connectedCallback() {
    super.connectedCallback();

    this._stateHasChanged = stateReflector(
      () => this._state,
      (state) => {
        this._state = state;
        this._render();
      },
    );

    let { open_count, unassigned_count } = await this._getCounts();
    console.log("FINAL");
    console.log(open_count);
    console.log(unassigned_count);
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
      this._render();
    }).catch((msg) => {
      console.log(msg);
      msg.resp.text().then(errorMessage);
    });
  }

  // Await and async.
  async _getCounts(client, source, query) {
    // Parse params...
    console.log("CALLING NOW!")
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
        console.log("DONE");
        console.log(json);
        this._render();
    });
    return {open_count, unassigned_count};
  }
});
