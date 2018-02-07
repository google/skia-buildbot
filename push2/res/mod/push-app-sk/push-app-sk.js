import 'skia-elements/buttons'
import 'skia-elements/spinner-sk'
import { $ } from 'skia-elements/core'

import 'common/login-sk'
import 'common/error-toast-sk'
import 'common/systemd-unit-status-sk'
import { errorMessage } from 'common/errorMessage'

import { html, render } from 'lit-html/lib/lit-extended'

import '../push-selection-sk'

const UPDATE_MS = 5000;

const template = (ele) => html`
<header><h1>Push</h1> <login-sk></login-sk></header>
<section class=controls>
  <button id=refresh on-click=${e => ele._refreshPackages(e)}>Refresh Packages</button>
  <spinner-sk id=spinner></spinner-sk>
  <label>Filter servers/apps: <input type=text on-input=${e => ele._filterInput(e)}></input></label>
</section>
<main>

</main>
<footer>
  <error-toast-sk></error-toast-sk>
  <push-selection-sk></push-selection-sk>
</footer>
`;

const jsonOrThrow = (resp) => {
  if (resp.ok) {
    return resp.json();
  }
  throw 'Bad network response.';
}

// The <push-app-sk> custom element declaration.
//
//  Attributes:
//    None
//
//  Properties:
//    None
//
//  Events:
//    None
//
//  Methods:
//    None
//
window.customElements.define('push-app-sk', class extends HTMLElement {
  constructor() {
    super();
    this._state = {
      servers: [],
      packages: {},
      status: {},
    };
    this._status = {};
  }

  connectedCallback() {
    fetch('/_/state').then(jsonOrThrow).then(json => {
      this._state = json;
      this._render();
    }).catch(errorMessage);
    this._updateStatus();
    this._render();
    this._spinner = $('spinner');
  }

  _updateStatus() {
    fetch('/_/status').then(jsonOrThrow).then(json => {
      this._status = json;
      this._render();
      window.setTimeout(() => this._updateStatus(), UPDATE_MS);
    }).catch(err => {
      errorMessage(err)
      window.setTimeout(() => this._updateStatus(), UPDATE_MS);
    });
  }

  disconnectedCallback() {
  }

  _render() {
    render(template(this), this);
  }

  _refreshPackages(e) {

    this._spinner.active = true;
    fetch('/_/state?refresh=true').then(jsonOrThrow).then(json => {
      this._spinner.active = false;
      this._state = json;
      this._render();
    }).catch(err => {
      this._spinner.active = false;
      errorMessage(err);
    });
  }

  _filterInput(e) {
    console.log(e);
  }

});
