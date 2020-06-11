/**
 * @module module/query-chooser-sk
 * @description <h2><code>query-chooser-sk</code></h2>
 *
 * Displays the current value for a selection along with an edit button
 * that pops up a query-sk dialog to change the selection.
 *
 * Emits the same events as query-sk.
 *
 * @attr {string} current_query - The current query formatted as a URL formatted query string.
 *
 * @attr {string} count_url - The  URL to POST the query to, passed down to quuery-count-sk.
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { toParamSet } from 'common-sk/modules/query';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import '../../../infra-sk/modules/paramset-sk';
import '../../../infra-sk/modules/query-sk';

import '../query-count-sk';

import 'elements-sk/styles/buttons';

const template = (ele) => html`
  <div class=row>
    <button @click=${ele._editClick}>Edit</button>
    <paramset-sk id=summary .paramsets=${[toParamSet(ele.current_query)]}></paramset-sk>
  </div>
  <div id="dialog">
    <query-sk current_query=${ele.current_query} .paramset=${ele.paramset} .key_order=${ele.key_order} @query-change=${ele._queryChange}></query-sk>
    <div class=matches>Matches: <query-count-sk url=${ele.count_url} current_query=${ele.current_query}></query-count-sk></div>
    <button @click=${ele._closeClick}>Close</button>
  </div>
  `;

define('query-chooser-sk', class extends ElementSk {
  constructor() {
    super(template);
    this.current_query = '';
  }

  connectedCallback() {
    super.connectedCallback();
    this._upgradeProperty('paramset');
    this._upgradeProperty('current_query');
    this._upgradeProperty('key_order');
    this._upgradeProperty('count_url');
    this._render();
    this._dialog = this.querySelector('#dialog');
  }

  _editClick() {
    this._dialog.classList.add('display');
  }

  _closeClick() {
    this._dialog.classList.remove('display');
  }

  _queryChange(e) {
    this.current_query = e.detail.q;
    this._render();
  }

  /** @prop paramset {string} The paramset to make selections from. */
  get paramset() { return this._paramset; }

  set paramset(val) {
    this._paramset = val;
    this._render();
  }

  /** @prop key_order {string} An array of strings, passed down to
   * query-sk.key_order.
   */
  get key_order() { return this._key_order; }

  set key_order(val) {
    this._key_order = val;
    this._render();
  }

  static get observedAttributes() {
    return ['current_query', 'key_order'];
  }

  /** @prop current_query {string} Mirrors the current_query attribute.  */
  get current_query() { return this.getAttribute('current_query'); }

  set current_query(val) { this.setAttribute('current_query', val); }

  /** @prop count_url {string} Mirrors the count_url attribute. */
  get count_url() { return this.getAttribute('count_url'); }

  set count_url(val) { this.setAttribute('count_url', val); }

  attributeChangedCallback() {
    this._render();
  }
});
