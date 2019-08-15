/**
 * @module module/query-sk
 * @description <h2><code>query-sk</code></h2>
 *
 * Starting from a serialized paramtools.ParamSet, this control allows the user
 * to build up a query, suitable for passing to query.New.
 *
 * @evt query-change - The 'query-sk' element will produce 'query-change' events when the query
 *      parameters chosen have changed. The event contains the current selections formatted as a URL query, found in e.detail.q.
 *
 * @evt query-change-delayed - The same exact event as query-change, but with a 500ms delay after
 *      the query stops changing.
 *
 * @attr {string} current_query - The current query formatted as a URL formatted query string.
 *
 */
import { html, render } from 'lit-html'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import '../query-values-sk'
import 'elements-sk/select-sk'
import { toParamSet, fromParamSet } from 'common-sk/modules/query'
import 'elements-sk/styles/buttons'

const _keys = (ele) => {
  return ele._keys.map((k) => html`<div>${k}</div>`);
}

const template = (ele) => html`
  <div>
    <label>Filter <input id=fast @input=${ele._fastFilter}></label>
    <button @click=${ele._clearFilter}>Clear Filter</button>
  </div>
  <div class=bottom>
    <div class=selection>
      <select-sk @selection-changed=${ele._keyChange}>
        ${_keys(ele)}
        </select-sk>
        <button @click=${ele._clear}>Clear Selections</button>
    </div>
    <query-values-sk id=values @query-values-changed=${ele._valuesChanged}></query-values-sk>
  </div>
`;

// The delay in ms before sending a delayed query-change event.
const DELAY_MS = 500;

window.customElements.define('query-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._paramset = {};
    this._originalParamset = {};
    this._key_order = [];

    // We keep the current_query as an object.
    this._query = {};

    // The full set of keys in the desired order.
    this._keys = [];

    // The id of a pending timeout func that will send a delayed query-change event.
    this._delayedTimeout = null;
  }

  connectedCallback() {
    super.connectedCallback();
    this._upgradeProperty('paramset');
    this._upgradeProperty('key_order');
    this._upgradeProperty('current_query');
    this._render();
    this._values = this.querySelector('#values');
    this._keySelect = this.querySelector('select-sk');
    this._fast = this.querySelector('#fast');
  }

  _valuesChanged(e) {
    const key = this._keys[this._keySelect.selection];
    this._query[key] = e.detail;
    this._queryChanged();
  }

  _keyChange() {
    if (this._keySelect.selection === -1) {
      return
    }
    const key = this._keys[this._keySelect.selection];
    this._values.options = this._paramset[key] || [];
    this._values.selected = this._query[key] || [];
    this._render();
  }

  _recalcKeys() {
    let keys = Object.keys(this._paramset);
    keys.sort();
    // Pull out all the keys that appear in _key_order to be pushed to the front of the list.
    let pre = this._key_order.filter(ordered => keys.indexOf(ordered) > -1);
    let post = keys.filter(key => pre.indexOf(key) === -1);
    this._keys = pre.concat(post);
  }

  _queryChanged() {
    const prev_query = this.current_query;
    // Rationalize the _query, i.e. remove keys that don't exist in the
    // paramset.
    const originalKeys = Object.keys(this._originalParamset);
    Object.keys(this._query).forEach((key) => {
      if (originalKeys.indexOf(key) === -1) {
        delete this._query[key];
      }
    });
    this.current_query = fromParamSet(this._query);
    if (prev_query !== this.current_query) {
      this.dispatchEvent(new CustomEvent('query-change', {
        detail: {q: this.current_query},
        bubbles: true,
      }));
      clearTimeout(this._delayedTimeout);
      this._delayedTimeout = setTimeout(() => {
        this.dispatchEvent(new CustomEvent('query-change-delayed', {
          detail: {q: this.current_query},
          bubbles: true,
        }));
      }, DELAY_MS);
    }
  }

  _clear() {
    this._query = {};
    this._recalcKeys();
    this._queryChanged();
    this._keyChange();
    this._render();
  }

  _fastFilter() {
    const filters = this._fast.value.trim().toLowerCase().split(/\s+/);

    // Create a closure that returns true if the given label matches the filter.
    const matches = (s) => {
      s = s.toLowerCase();
      return filters.filter(f => s.indexOf(f) > -1).length > 0;
    };

    // Loop over this._originalParamset.
    var filtered = {};
    Object.keys(this._originalParamset).forEach((paramkey) => {
      // If the param key matches, then all the values go over.
      if (matches(paramkey)) {
        filtered[paramkey] = this._originalParamset[paramkey];
      } else {
        // Look for matches in the param values.
        const valueMatches = [];
        this._originalParamset[paramkey].forEach((paramvalue) => {
          if (matches(paramvalue)) {
            valueMatches.push(paramvalue);
          }
        });
        if (valueMatches.length > 0) {
          filtered[paramkey] = valueMatches;
        }
      }
    });

    this._paramset = filtered;
    this._recalcKeys();
    this._keyChange();
    this._render();
  }

  _clearFilter() {
    this._fast.value = "";
    this.paramset = this._originalParamset;
    this._queryChanged();
  }

  /** @prop paramset {Object} A serialized paramtools.ParamSet. */
  get paramset() { return this._paramset }
  set paramset(val) {
    // Record the current key so we can restore it later.
    let prevSelectKey = '';
    if (this._keySelect && this._keySelect.selection) {
      prevSelectKey = this._keys[this._keySelect.selection];
    }

    this._paramset = val;
    this._originalParamset = val;
    this._recalcKeys();
    if (this._fast && this._fast.value.trim() !== '') {
      this._fastFilter();
    }
    this._render();

    // Now re-select the current key if it still exists post-filtering.
    if (this._keySelect && prevSelectKey && this._keys.indexOf(prevSelectKey) != -1) {
      this._keySelect.selection = this._keys.indexOf(prevSelectKey);
      this._keyChange();
    }
  }

  /** @prop key_order {string} An array of strings, the keys in the order they
   * should appear. All keys not in the key order will be present after and in
   * alphabetical order.
   */
  get key_order() { return this._key_order }
  set key_order(val) {
    this._key_order = val;
    this._recalcKeys();
    this._render();
  }

  static get observedAttributes() {
    return ['current_query'];
  }

  /** @prop current_query {string} Mirrors the current_query attribute.  */
  get current_query() { return this.getAttribute('current_query'); }
  set current_query(val) { this.setAttribute('current_query', val); }

  attributeChangedCallback(name, oldValue, newValue) {
    this._query = toParamSet(newValue)
    // Convert current_query the string into an object.
    this._render();
  }
});
