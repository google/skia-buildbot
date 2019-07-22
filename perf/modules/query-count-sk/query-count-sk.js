/**
 * @module modules/query-count-sk
 * @description <h2><code>query-count-sk</code></h2>
 *
 * Reports the number of matches for a given query.
 *
 * @attr {string} current_query - The current query to count against.
 *
 * @attr {string} url - The URL to POST the query to.
 *
 */
import { html, render } from 'lit-html'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import { errorMessage } from 'elements-sk/errorMessage'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'
import 'elements-sk/spinner-sk'

const template = (ele) => html`
  <div>
    <span>${ele._count}</span>
    <spinner-sk ?active=${ele._requestInProgress}></spinner-sk>
  </div>
  `;

window.customElements.define('query-count-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._last_query = '';
    this._count = '';
    this._requestInProgress = false;
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this._fetch();
  }

  static get observedAttributes() {
    return ['current_query', 'url'];
  }

  _fetch() {
    if (!this._connected) {
      return;
    }
    if (!this.url || !this.current_query) {
      return;
    }
    if (this._requestInProgress) {
      return;
    }
    this._requestInProgress = true;
    this._last_query = this.current_query;
    let now = Math.floor(Date.now()/1000);
    let body = {
      q: this.current_query,
      end: now,
      begin: now - 24*60*60,
    };
    this._render();
    fetch(this.url, {
      method: 'POST',
      body: JSON.stringify(body),
      headers:{
        'Content-Type': 'application/json'
      }
    }).then(jsonOrThrow).then((json) => {
      this._count = '' + json.count;
      this._requestInProgress = false;
      this._render();
      if (this._last_query != this.current_query) {
        this._fetch();
      }
    }).catch((msg) => {
      this._requestInProgress = false;
      this._render();
      errorMessage(msg);
    });
  }

  attributeChangedCallback(name, oldValue, newValue) {
    this._fetch();
  }

  /** @prop url {string}  */
  get url() { return this.getAttribute('url'); }
  set url(val) { this.setAttribute('url', val); }

  /** @prop current_query {string}  */
  get current_query() { return this.getAttribute('current_query'); }
  set current_query(val) { this.setAttribute('current_query', val); }

});
