/**
 * @module module/alerts-page-sk
 * @description <h2><code>alerts-page-sk</code></h2>
 *
 * A page for editing all the alert configs.
 */
import 'elements-sk/checkbox-sk'
import 'elements-sk/icon/build-icon-sk'
import 'elements-sk/icon/create-icon-sk'
import 'elements-sk/icon/delete-icon-sk'
import 'elements-sk/styles/buttons'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import { Login } from '../../../infra-sk/modules/login.js'
import dialogPolyfill from 'dialog-polyfill'
import { errorMessage } from 'elements-sk/errorMessage.js'
import { fromObject } from 'common-sk/modules/query'
import { html, render } from 'lit-html'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'

import '../alert-config-sk'
import '../query-summary-sk'

function _ifNotActive(s) {
  return (s === 'ACTIVE') ? '' : 'Archived';
}

const _rows = (ele) => {
  return ele._alerts.map((item) =>  html`
    <tr>
      <td><create-icon-sk title='Edit' @click=${ele._edit} .__config=${item}></create-icon-sk></td>
      <td>${item.display_name}</td>
      <td><query-summary-sk selection=${item.query}></query-summary-sk></td>
      <td>${item.alert}</td>
      <td>${item.owner}</td>
      <td><delete-icon-sk title='Delete' @click=${ele._delete} .__config=${item}></delete-icon-sk></td>
      <td><a href=${ele._dryrunUrl(item)}><build-icon-sk title='Dry Run'></build-icon-sk></td>
      <td>${_ifNotActive(item.state)}</td>
    </tr>
    `);
}

const template = (ele) => html`
  <dialog>
    <alert-config-sk id=alertconfig .paramset=${ele._paramset} .config=${ele._cfg}></alert-config-sk>
    <div class=buttons>
      <button @click=${ele._cancel}>Cancel</button>
      <button @click=${ele._accept}>Accept</button>
    </div>
  </dialog>
  <table>
    <tr>
      <th></th>
      <th>Name</th>
      <th>Query</th>
      <th>Alert</th>
      <th>Owner</th>
      <th></th>
      <th></th>
      <th></th>
    </tr>
    ${_rows(ele)}
  </table>
  <div class=warning ?hidden=${!!ele._alerts.length} >
    No alerts have been configured.
  </div>
  <button class=fab @click=${ele._add}>+</button>
  <checkbox-sk ?checked=${ele._showDeleted} @change=${ele._showChanged} label='Show deleted configs.'></checkbox-sk>
`;

window.customElements.define('alerts-page-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._cfg = {};
    this._alerts = [];
    this._paramset = {};
    this._showDeleted = false;
    this._email = '';
    Login.then((status) => { this._email = status.Email});
  }

  connectedCallback() {
    super.connectedCallback();
    let pInit =  fetch('/_/initpage/').then(jsonOrThrow).then((json) => {
      this._paramset = json.dataframe.paramset;
    });
    let pList = fetch(`/_/alert/list/${this._showDeleted}`).then(jsonOrThrow).then((json) => {
      this._alerts = json;
    });
    Promise.all([pInit, pList]).then(() => {
      this._render();
      this._dialog = this.querySelector('dialog');
      this._alertconfig = this.querySelector('#alertconfig');
      dialogPolyfill.registerDialog(this.querySelector('dialog'));
      this._openOnLoad();
    }).catch(errorMessage);;
  }

  _showChanged(e) {
    this._showDeleted = e.target.checked;
    this._list();
  }

  _list() {
    fetch(`/_/alert/list/${this._showDeleted}`).then(jsonOrThrow).then((json) => {
      this._alerts = json;
      this._render();
      this._openOnLoad();
    }).catch(errorMessage);
  }

  _openOnLoad() {
    if (window.location.search.length == 0) {
      return;
    }
    let id = +window.location.search.slice(1);
    for (let i = 0; i < this._alerts.length; i++) {
      if (id === this._alerts[i].id) {
        this._cfg =  JSON.parse(JSON.stringify(this._alerts[i]));
        this._dialog.showModal();
        break
      }
    }
    history.pushState(null, '', '/a/');
  }

  _dryrunUrl(config) {
    return '/d/?' + fromObject(config);
  }

  _add() {
    // Load an new Config from the server.
    fetch('/_/alert/new').then(jsonOrThrow).then((json) => {
      this._cfg = json;
      this._render();
      // Pop up edit dialog using the new Config.
      this._dialog.showModal();
    }).catch(errorMessage);
  }

  _edit(e) {
    this._cfg = JSON.parse(JSON.stringify(e.target.__config));
    this._render();
    this._dialog.showModal();
  }

  _cancel(e) {
    this._dialog.close();
  }

  _accept(e) {
    this._dialog.close();
    this._cfg = this._alertconfig.config;
    if (JSON.stringify(this._cfg) === JSON.stringify(this._orig_cfg)) {
      return;
    }
    // Post the config.
    fetch('/_/alert/update', {
      method: 'POST',
      body: JSON.stringify(this._cfg),
      headers:{
        'Content-Type': 'application/json'
      }
    }).then(jsonOrThrow).then((json) => this._list()).catch(errorMessage);
  }

  _delete(e) {
    if (!window.confirm('Are you sure you want to delete this alert?')) {
      return;
    }
    fetch(`/_/alert/delete/${e.target.__config.id}`, {
      method: 'POST',
    }).then(() => this._list()).catch(errorMessage);
  }

});
