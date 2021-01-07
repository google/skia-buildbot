/**
 * @module tree-status-sk
 * @description <h2><code>tree-status-sk</code></h2>
 *
 * <p>
 *   Displays the enter-tree-status-sk and display-tree-status-sk elements.
 *   Handles calls to the backend from events originating from those elements.
 * </p>
 *
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { errorMessage } from 'elements-sk/errorMessage';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';

import '../display-tree-status-sk';
import '../enter-tree-status-sk';

import { $$ } from 'common-sk/modules/dom';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

const template = (ele) => html`
<enter-tree-status-sk .autorollers=${ele._autorollers}></enter-tree-status-sk>
<display-tree-status-sk .statuses=${ele._statuses}></display-tree-status-sk>
`;

define('tree-status-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._statuses = [];
    this._autorollers = [];
  }

  connectedCallback() {
    super.connectedCallback();

    this.addEventListener('new-tree-status', (e) => this._saveStatus(e));
    this.addEventListener('set-tree-status', (e) => this._setTreeStatus(e));

    this._poll();
    this._render();
  }

  _poll() {
    this._getStatuses();
    this._getAutorollers();
    window.setTimeout(() => this._poll(), 10000);
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

  _saveStatus(e) {
    this._doImpl('/_/add_tree_status', e.detail, (json) => { this._statuses = json; });
  }

  _getStatuses() {
    this._doImpl('/_/recent_statuses', {}, (json) => { this._statuses = json; });
  }

  _getAutorollers() {
    this._doImpl('/_/get_autorollers', {}, (json) => { this._autorollers = json; });
  }

  _setTreeStatus(e) {
    $$('enter-tree-status-sk').status_value = e.detail;
  }
});
