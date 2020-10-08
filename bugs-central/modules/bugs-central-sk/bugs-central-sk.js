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

// import '../display-bugs-central-sk';
// import '../enter-bugs-central-sk';

import { $$ } from 'common-sk/modules/dom';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

// <enter-bugs-central-sk .autorollers=${ele._autorollers}></enter-bugs-central-sk>
// <display-bugs-central-sk .statuses=${ele._statuses}></display-bugs-central-sk>

const template = (ele) => html`
HERE HERE
`;

define('bugs-central-sk', class extends ElementSk {
  constructor() {
    super(template);
    //this._statuses = [];
  }

  connectedCallback() {
    super.connectedCallback();

    this._render();
  }

  // Common work done for all fetch requests.
  _doImpl(url, detail, action) {
    fetch(url, {
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
      msg.resp.text().then(errorMessage);
    });
  }

  // _saveStatus(e) {
  //   this._doImpl('/_/add_tree_status', e.detail, (json) => { this._statuses = json; });
  // }

});
