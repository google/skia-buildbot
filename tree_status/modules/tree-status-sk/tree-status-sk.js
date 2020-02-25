/**
 * @module tree-status-sk
 * @description <h2><code>tree-status-sk</code></h2>
 *
 *   The main application element for am.skia.org.
 *
 */

import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import { define } from 'elements-sk/define'
import { html, render } from 'lit-html'
import { errorMessage } from 'elements-sk/errorMessage'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'

import '../tree-status-sk'
import '../display-tree-status-sk'

import { $$ } from 'common-sk/modules/dom'
import 'elements-sk/error-toast-sk'
import 'elements-sk/spinner-sk'

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

  _poll() {
    this._getStatuses();
    this._getAutorollers();
    window.setTimeout(() => this._poll(), 5000);
  }

  // MOVE TO COMMON JS TO USE FROM DIFFERENT COMPONENTS>
  // Common work done for all fetch requests.
  _doImpl(url, detail, action=json => this._incidentAction(json)) {
    // this._busy.active = true;
    fetch(url, {
      body: JSON.stringify(detail),
      headers: {
        'content-type': 'application/json',
      },
      credentials: 'include',
      method: 'POST',
    }).then(jsonOrThrow).then(json => {
      action(json)
      this._render();
      // this._busy.active = false;
    }).catch(msg => {
      console.log("ERROR");
      console.log(msg);
      console.log(msg.resp);
      // this._busy.active = false;
      msg.resp.text().then(errorMessage);
    });
  }

  connectedCallback() {
    super.connectedCallback();

    this.addEventListener('new-tree-status', e => this._saveStatus(e));
    this.addEventListener('set-tree-status', e => this._setTreeStatus(e));

    this._poll();
    this._render();
  }

  _saveStatus(e) {
    console.log("_saveStatus");
    // this._doImpl('/_/recent_statuses', {}, json => {this._statuses = json});
    // Call something that refreshes the get statuses!!!!
    this._doImpl('/_/add_tree_status', e.detail, json => {this._statuses = json});
  }

  _getStatuses() {
    this._doImpl('/_/recent_statuses', {}, json => {this._statuses = json});
  }

  _setTreeStatus(e) {
    console.log("HERE HERE");
    console.log($$('enter-tree-status-sk').status_value);
    console.log(e.detail);
    $$('enter-tree-status-sk').status_value = e.detail;
  }

  _getAutorollers() {
    this._doImpl('/_/get_autorollers', {}, json => {this._autorollers = json});
  }

});
