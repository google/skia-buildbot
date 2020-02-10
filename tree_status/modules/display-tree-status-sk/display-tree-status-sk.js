/**
 * @module display-tree-status-sk
 * @description <h2><code>display-tree-status-sk</code></h2>
 *
 *   The main application element for am.skia.org.
 *
 */

import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import { define } from 'elements-sk/define'
import { html, render } from 'lit-html'
import { Login } from '../../../infra-sk/modules/login'
import { errorMessage } from 'elements-sk/errorMessage'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'

import 'elements-sk/error-toast-sk'
import 'elements-sk/spinner-sk'

// States of the tree.
const OPEN = 'open';
const CAUTION = 'caution';
const CLOSED = 'closed';


function recentStatuses(ele) {
  return ele._statuses.map(status => html`<tr class=${getStatusClass(status.message)}><td>${status.username}</td><td>${getLocalDate(status.date)}</td><td>${status.message}</td></tr>`);
}

function getLocalDate(timestamp) {
  return new Date(timestamp).toLocaleString();
}

function getStatusClass(message) {
  let treeState = '';
  const lowerCaseMessage = message.toLowerCase();
  if (lowerCaseMessage.includes(OPEN)) {
    treeState = OPEN;
  } else if (lowerCaseMessage.includes(CAUTION)) {
    treeState = CAUTION;
  } else if (lowerCaseMessage.includes(CLOSED)) {
    treeState = CLOSED;
  }
  return treeState;
}

const template = (ele) => html`
<div>hello world3</div>
<table class="recent_statuses">
  <tr>
    <th>Who</th>
    <th>When</th>
    <th>Message</th>
  </tr>
  ${recentStatuses(ele)}
</table>
`;

define('display-tree-status-sk', class extends ElementSk {
  constructor() {
    super(template);
    console.log("AAA");
    this._user = '';
    this._statuses = [];
    this._getStatuses();
    Login.then(loginstatus => {
      this._user = loginstatus.Email;
      // this._render();
    });
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

  _getStatuses() {
    console.log("IN _GETSTATUSES");
    this._doImpl('/_/recent_statuses', {}, json => {console.log("GOT THIS STUFF" + json); this._statuses = json});
    console.log(this._statuses);
    console.log(this._statuses[0]);
  }

  connectedCallback() {
    super.connectedCallback();
    // upgradeProperty(this, 'cid');
    this._render();
  }

  /*
  _render() {
    render(template(this), this, {eventContext: this});
  }
  */

});
