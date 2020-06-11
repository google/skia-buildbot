/**
 * @module module/alerts-page-sk
 * @description <h2><code>alerts-page-sk</code></h2>
 *
 * A page for editing all the alert configs.
 */
import 'elements-sk/checkbox-sk';
import 'elements-sk/icon/build-icon-sk';
import 'elements-sk/icon/create-icon-sk';
import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/styles/buttons';
import dialogPolyfill from 'dialog-polyfill';
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { fromObject, toParamSet } from 'common-sk/modules/query';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { Login } from '../../../infra-sk/modules/login';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import '../../../infra-sk/modules/paramset-sk';

import '../alert-config-sk';

function _dryrunUrl(config) {
  return `/d/?${fromObject(config)}`;
}

function _ifNotActive(s) {
  return (s === 'ACTIVE') ? '' : 'Archived';
}

const _rows = (ele) => ele._alerts.map((item) => html`
    <tr>
      <td><create-icon-sk title='Edit' @click=${ele._edit} .__config=${item} ?disabled=${!ele._email}></create-icon-sk></td>
      <td>${item.display_name}</td>
      <td><paramset-sk .paramsets=${[toParamSet(item.query)]}></paramset-sk></td>
      <td>${item.alert}</td>
      <td>${item.owner}</td>
      <td><delete-icon-sk title='Delete' @click=${ele._delete} .__config=${item} ?disabled=${!ele._email}></delete-icon-sk></td>
      <td><a href=${_dryrunUrl(item)}><build-icon-sk title='Dry Run'></build-icon-sk></td>
      <td>${_ifNotActive(item.state)}</td>
    </tr>
    `);

const template = (ele) => html`
  <dialog>
    <alert-config-sk id=alertconfig .paramset=${ele._paramset} .config=${ele.cfg}></alert-config-sk>
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
  <button class=fab @click=${ele._add} ?disabled=${!ele._email}>+</button>
  <checkbox-sk ?checked=${ele._showDeleted} @change=${ele._showChanged} label='Show deleted configs.'></checkbox-sk>
`;

const okOrThrow = async (resp) => {
  if (!resp.ok) {
    const text = await resp.text();
    throw new Error(text);
  }
};

define('alerts-page-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._cfg = {};
    this._alerts = [];
    this._paramset = {};
    this._showDeleted = false;
    this._email = '';
    Login.then((status) => { this._email = status.Email; });
  }

  connectedCallback() {
    super.connectedCallback();
    const pInit = fetch('/_/initpage/').then(jsonOrThrow).then((json) => {
      this._paramset = json.dataframe.paramset;
    });
    const pList = this._listPromise().then((json) => {
      this._alerts = json;
    });
    Promise.all([pInit, pList]).then(() => {
      this._render();
      this._dialog = this.querySelector('dialog');
      this._alertconfig = this.querySelector('#alertconfig');
      dialogPolyfill.registerDialog(this.querySelector('dialog'));
      this._openOnLoad();
    }).catch(errorMessage);
  }

  _showChanged(e) {
    this._showDeleted = e.target.checked;
    this._list();
  }

  /**
   * Start a request to get all the alerts.
   *
   * @returns {Promise} The started fetch().
   */
  _listPromise() {
    return fetch(`/_/alert/list/${this._showDeleted}`).then(jsonOrThrow);
  }

  /**
   * Load all the alerts from the server.
   */
  _list() {
    this._listPromise().then((json) => {
      this._alerts = json;
      this._render();
      this._openOnLoad();
    }).catch(errorMessage);
  }

  /**
   * Display the modal dialog box for a specific alert if part of the URL.
   */
  _openOnLoad() {
    if (window.location.search.length === 0) {
      return;
    }
    const id = +window.location.search.slice(1);
    const matchingAlert = this._alerts.find((alert) => id === alert.id);
    if (matchingAlert) {
      this._startEditing(matchingAlert);
    }
    window.history.pushState(null, '', '/a/');
  }

  _add() {
    // Load an new Config from the server.
    fetch('/_/alert/new').then(jsonOrThrow).then((json) => {
      this._startEditing(json);
    }).catch(errorMessage);
  }

  _edit(e) {
    if (!this._email) {
      errorMessage('You must be logged in to edit alerts.');
      return;
    }
    this._startEditing(e.target.__config);
  }

  _startEditing(cfg) {
    this._orig_cfg = JSON.parse(JSON.stringify(this._cfg));
    this.cfg = cfg;
    this._dialog.showModal();
  }

  _cancel() {
    this._dialog.close();
  }

  _accept() {
    this._dialog.close();
    this.cfg = this._alertconfig.config;
    if (JSON.stringify(this.cfg) === JSON.stringify(this._orig_cfg)) {
      return;
    }
    // Post the config.
    fetch('/_/alert/update', {
      method: 'POST',
      body: JSON.stringify(this.cfg),
      headers: {
        'Content-Type': 'application/json',
      },
    }).then(okOrThrow).then(() => {
      this._list();
    }).catch(errorMessage);
  }

  _delete(e) {
    fetch(`/_/alert/delete/${e.target.__config.id}`, {
      method: 'POST',
    }).then(okOrThrow).then(() => {
      this._list();
    }).catch(errorMessage);
  }

  /** @prop cfg {string} The alert config being edited. */
  get cfg() { return this._cfg; }

  set cfg(val) {
    this._cfg = JSON.parse(JSON.stringify(val));
    if (this._cfg && !this._cfg.owner) {
      this._cfg.owner = this._email;
    }
    this._render();
  }
});
