import 'elements-sk/error-toast-sk'
import { errorMessage } from 'elements-sk/errorMessage'
import { upgradeProperty } from 'elements-sk/upgradeProperty'

import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'
import { stateReflector } from 'common-sk/modules/stateReflector'

import { html, render } from 'lit-html/lib/lit-extended'

// Main template for this element
const template = (ele) => html`
<header>Code Coverage</header>

<main>
  <h2>${ele._query.job} @ ${ele._query.commit}</h2>
  <iframe class=innerframe src="/cov_html/${ele._query.commit}/${ele._query.job}/html/index.html"></iframe>
</main>
<footer>
  <error-toast-sk></error-toast-sk>
</footer>`;

// The <coverage-page-sk> custom element declaration.
//
//  This is the main page for coverage.skia.org.
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
window.customElements.define('coverage-page-sk', class extends HTMLElement {

  constructor() {
    super();
    upgradeProperty(this, 'job');
    upgradeProperty(this, 'commit');
    // Bits of state that get reflected to/from the URL query string.
    this._query = {
      job: '',
      commit: '',
    }
  }

  connectedCallback() {
    this._stateHasChanged = stateReflector(() => this._query, (query) => {
      this._query = query;
      this._render();
    });
    this._render();
  }

  get job() { return this._query.job; }
  set job(val) {
    this._query.job = val;
    this._stateHasChanged();
    this._render();
  }

  get commit() { return this._query.commit; }
  set commit(val) {
    this._query.commit = val;
    this._stateHasChanged();
    this._render();
  }

  _render() {
    render(template(this), this);
  }

});
