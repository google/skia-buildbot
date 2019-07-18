/**
 * @module modules/query-sk
 * @description <h2><code>query-sk</code></h2>
 *
 * @evt query-change - The 'query2-sk' element will produce 'query-change' events when the query
 *     parameters chosen have changed. The event contains the current
 *     selections formatted as a URL query, found in e.detail.q.
 *
 * @evt query-change-delayed - The same exact event as query-change, but with a 500ms delay after
 *      the query stops changing.
 *
 */
import { html, render } from 'lit-html'
import { ElementSk } from '../../infra-sk/ElementSk'

const template = (ele) => html``;

window.customElements.define('query-sk', class extends ElementSk {
  constructor() {
    super(template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  /** @prop current_query {string} The selection of the control formatted as a URL query string.
   */
  get current_query() { return this._current_query }
  set current_query(val) { this._current_query = val;
    this._render();
  }

  /** @prop paramset {string} A serialized paramtools.ParamSet, the source
   * of options for the query.
   */
  get paramset() { return this._paramset }
  set paramset(val) {
    this._paramset = val;
    this._render();
  }

  /** @prop key_order {Array} The keys in the order they should appear. All
   * keys not in the key order will be present after and in
   * alphabetical order.
   */
  get key_order() { return this._key_order }
  set key_order(val) {
    this._key_order = val;
    this._render();
  }

});
