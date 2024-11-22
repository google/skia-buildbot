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
 * @attr {boolean} hide_invert - If the option to invert a query should be made available to
 *       the user.
 * @attr {boolean} hide_regex - If the option to include regex in the query should be made
 *       available to the user.
 * @attr {boolean} values_only - If true then only display the values selection and hide the key selection.
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ParamSet, toParamSet, fromParamSet } from '../query';
import { SelectSk } from '../../../elements-sk/modules/select-sk/select-sk';
import { ElementSk } from '../ElementSk';
import {
  QueryValuesSk,
  QueryValuesSkQueryValuesChangedEventDetail,
} from '../query-values-sk/query-values-sk';

import '../query-values-sk';
import '../../../elements-sk/modules/select-sk';

// The delay in ms before sending a delayed query-change event.
const DELAY_MS = 500;

export interface QuerySkQueryChangeEventDetail {
  readonly q: string;
}

/**
 * Removes the prefix, if any, from a query value.
 *
 * TODO(jcgregorio) - The fact that query values can have a prefix of either '!' or '~'
 * is just something you have to know about them. We need a way to share the knowledge
 * of all possible valid prefixes with the Go code.
 */
export const removePrefix = (s: string): string => {
  if (s.length === 0) {
    return s;
  }
  if ('~!'.includes(s[0])) {
    return s.slice(1);
  }
  return s;
};

export class QuerySk extends ElementSk {
  private static template = (ele: QuerySk) => html`
    <div class="filtering">
      <input
        id="fast"
        @input=${ele._fastFilter}
        placeholder="Search Parameters and Values"
        name="query-sk-filter"
        autocomplete="off" />
      ${QuerySk.clearFilterButton(ele)}
    </div>
    <div class="bottom">
      <div class="selection">
        <select-sk @selection-changed=${ele._keyChange}> ${QuerySk.keysTemplate(ele)} </select-sk>
        <button @click=${ele._clear} class="clear_selections">Clear Selections</button>
      </div>
      <query-values-sk
        id="values"
        class=${ele._keySelect?.selection === -1 ? 'hidden' : ''}
        @query-values-changed=${ele._valuesChanged}
        ?hide_invert=${ele.hide_invert}
        ?hide_regex=${ele.hide_regex}></query-values-sk>
    </div>
  `;

  private static clearFilterButton(ele: QuerySk) {
    if (!ele._filtering) {
      return html``;
    }
    return html`
      <button @click=${ele._clearFilter} class="clear_filters" title="Clear filter">&cross;</button>
    `;
  }

  private static keysTemplate = (ele: QuerySk) => ele._keys.map((k) => html` <div>${k}</div> `);

  private _paramset: ParamSet = {};

  private _originalParamset: ParamSet = {};

  // True if there is text in the fitler input.
  private _filtering: boolean = false;

  // We keep the current_query as an object.
  private _query: ParamSet = {};

  private _key_order: string[] = [];

  // The full set of keys in the desired order.
  private _keys: string[] = [];

  // The id of a pending timeout func that will send a delayed query-change event.
  private _delayedTimeout: number | null = null;

  private _keySelect: SelectSk | null = null;

  private _values: QueryValuesSk | null = null;

  private _fast: HTMLInputElement | null = null;

  constructor() {
    super(QuerySk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._upgradeProperty('paramset');
    this._upgradeProperty('key_order');
    this._upgradeProperty('current_query');
    this._upgradeProperty('hide_invert');
    this._upgradeProperty('hide_regex');
    this._render();
    this._values = this.querySelector('#values');
    this._keySelect = this.querySelector('select-sk');
    this._fast = this.querySelector('#fast');
  }

  private _valuesChanged(e: CustomEvent<QueryValuesSkQueryValuesChangedEventDetail>) {
    const key = this._keys[this._keySelect!.selection as number];
    if (this._fast!.value.trim() !== '') {
      // Things get complicated if the user has entered a filter. The user may
      // have selections in this._query[key] which don't appear in e.detail
      // because they have been filtered out, so we should only add/remove
      // values from this._query[key] that appear in this._paramset[key], while
      // being careful because value(s) may be prefixed with either a '!' or a
      // '~' if they are invert or regex queries.

      // When we toggle from a regex to a non-regex we need to clear the values.
      if (!e.detail.regex && this._query[key] && this._query[key][0][0] === '~') {
        this._query[key] = [];
      }

      if (e.detail.regex) {
        this._query[key] = e.detail.values;
      }

      // The user might have toggled the invert checkbox, which means we need to
      // make sure that the current values for the current key are also inverted
      // appropriately even if not displayed due to the filter.
      this._applyInvert(key, e.detail.invert);

      // Make everything into Sets to make our lives easier.
      const valuesDisplayed = new Set(this._paramset[key]);
      const currentQueryForKey = new Set(this._query[key]);
      const unprefixedSelectionsFromEvent = new Set(e.detail.values.map(removePrefix));

      // Loop over valuesDisplayed, if the value appears in selectionsFromEvent
      // then add it to currentQueryForKey, otherwise remove it from
      // currentQueryForKey.
      valuesDisplayed.forEach((value) => {
        const prefix = e.detail.invert ? '!' : '';
        const prefixedValue = prefix + value;
        if (unprefixedSelectionsFromEvent.has(value)) {
          currentQueryForKey.add(prefixedValue);
        } else {
          currentQueryForKey.delete(prefixedValue);
        }
      });
      this._query[key] = [...currentQueryForKey];
    } else {
      this._query[key] = e.detail.values;
    }
    this._queryChanged();
  }

  /**
   * Set or clear the invery prefix ('!') on all the values for the given key in
   * this._query, based on the value of 'invert'.
   */
  private _applyInvert(key: string, invert: boolean): void {
    const values = this._query[key];
    if (!values || !values.length) {
      return;
    }
    const valuesHaveInvert = values[0][0] === '!';
    if (invert === valuesHaveInvert) {
      // eslint-disable-next-line no-useless-return
      return;
    }
    if (invert && !valuesHaveInvert) {
      this._query[key] = values.map((v) => `!${v}`);
    } else {
      this._query[key] = values.map(removePrefix);
    }
  }

  private _keyChange(): void {
    if (this._keySelect!.selection === -1) {
      return;
    }
    const key = this._keys[this._keySelect!.selection as number];
    this._values!.options = this._paramset[key] || [];
    this._values!.selected = this._query[key] || [];
    this._values!.clearFilter();
    this._render();
  }

  private _recalcKeys(): void {
    const keys = Object.keys(this._paramset);
    keys.sort();
    // Pull out all the keys that appear in _key_order to be pushed to the front of the list.
    const pre = this._key_order.filter((ordered) => keys.indexOf(ordered) > -1);
    const post = keys.filter((key) => pre.indexOf(key) === -1);
    this._keys = pre.concat(post);
  }

  private _queryChanged(): void {
    const prev_query = this.current_query;
    this._rationalizeQuery();
    if (prev_query !== this.current_query) {
      this.dispatchEvent(
        new CustomEvent<QuerySkQueryChangeEventDetail>('query-change', {
          detail: { q: this.current_query },
          bubbles: true,
        })
      );
      window.clearTimeout(this._delayedTimeout!);
      this._delayedTimeout = window.setTimeout(() => {
        this.dispatchEvent(
          new CustomEvent<QuerySkQueryChangeEventDetail>('query-change-delayed', {
            detail: { q: this.current_query },
            bubbles: true,
          })
        );
      }, DELAY_MS);
    }
  }

  // Rationalize the _query, i.e. remove keys and values that don't exist in the ParamSet.
  private _rationalizeQuery(): void {
    // We will use this to determine whether we've made any changes to the original query.
    const originalCurrentQuery = this.current_query;

    const originalKeys = Object.keys(this._originalParamset);
    Object.keys(this._query).forEach((key) => {
      if (originalKeys.indexOf(key) === -1) {
        // Filter out invalid keys.
        delete this._query[key];
      } else {
        // Filter out invalid values.
        this._query[key] = this._query[key].filter(
          (val) =>
            this._originalParamset[key].includes(val) || val.startsWith('~') || val.startsWith('!')
        );
      }
    });

    // _rationalizeQuery is called when current_query is set. This avoids an infinite recursion.
    const newCurrentQuery = fromParamSet(this._query);
    if (newCurrentQuery !== originalCurrentQuery) {
      this.current_query = fromParamSet(this._query);
    }
  }

  private _clear(): void {
    this._query = {};
    this._recalcKeys();
    this._queryChanged();
    this._keyChange();
    this._render();
  }

  private _fastFilter(): void {
    const filterString = this._fast!.value.trim();
    const filters = filterString.toLowerCase().split(/\s+/);

    if (filterString) {
      this._filtering = true;
    }

    // Create a closure that returns true if the given label matches the filter.
    const matches = (s: string) => {
      s = s.toLowerCase();
      return filters.filter((f) => s.indexOf(f) > -1).length > 0;
    };

    // Loop over this._originalParamset.
    const filtered: ParamSet = {};
    Object.keys(this._originalParamset).forEach((paramkey) => {
      // If the param key matches, then all the values go over.
      if (matches(paramkey)) {
        filtered[paramkey] = this._originalParamset[paramkey];
      } else {
        // Look for matches in the param values.
        const valueMatches: string[] = [];
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

  private _clearFilter(): void {
    this._fast!.value = '';
    this.paramset = this._originalParamset;
    this._filtering = false;
    this._queryChanged();
    this._render();
  }

  /** @prop paramset {Object} A serialized paramtools.ParamSet. */
  get paramset(): ParamSet {
    return this._paramset;
  }

  set paramset(val: ParamSet) {
    // Record the current key so we can restore it later.
    this._paramset = val;
    this._originalParamset = val;
    this._recalcKeys();
    if (this._fast && this._fast.value.trim() !== '') {
      this._fastFilter();
    }
    this._render();
  }

  /** Selects a key as if the user had pressed the given key. */
  public selectKey(key: string): void {
    if (!this._keySelect) {
      return;
    }
    this._keySelect.selection = this._keys.indexOf(key);
    this._keyChange();
  }

  /**
   * Removes the value from the paramset at the specified key
   * @param key paramset key
   * @param value value to remove
   */
  public removeKeyValue(key: string, value: string): void {
    const paramSet = this.paramset;
    const valIndex = paramSet[key].indexOf(value);
    if (valIndex > -1) {
      // Value is present in the paramset, so let's remove it
      paramSet[key].splice(valIndex, 1);
      this.paramset = paramSet;
      this._queryChanged();
    }
  }

  /**
   * The keys in the order they should appear. All keys not in the key order will be present after
   * and in alphabetical order.
   */
  get key_order(): string[] {
    return this._key_order;
  }

  set key_order(val: string[]) {
    this._key_order = val;
    this._recalcKeys();
    this._render();
  }

  /** Mirrors the hide_invert attribute.  */
  get hide_invert(): boolean {
    return this.hasAttribute('hide_invert');
  }

  set hide_invert(val: boolean) {
    if (val) {
      this.setAttribute('hide_invert', '');
    } else {
      this.removeAttribute('hide_invert');
    }
    this._render();
  }

  /**  Mirrors the hide_regex attribute.  */
  get hide_regex(): boolean {
    return this.hasAttribute('hide_regex');
  }

  set hide_regex(val: boolean) {
    if (val) {
      this.setAttribute('hide_regex', '');
    } else {
      this.removeAttribute('hide_regex');
    }
    this._render();
  }

  /** Mirrors the current_query attribute.  */
  get current_query(): string {
    return this.getAttribute('current_query') || '';
  }

  set current_query(val: string) {
    this.setAttribute('current_query', val);
  }

  /** Mirrors the values_only attribute.  */
  get values_only(): boolean {
    return this.hasAttribute('values_only');
  }

  set values_only(val: boolean) {
    if (val) {
      this.setAttribute('values_only', '');
    } else {
      this.removeAttribute('values_only');
    }
  }

  static get observedAttributes(): string[] {
    return ['current_query', 'hide_invert', 'hide_regex', 'values_only'];
  }

  attributeChangedCallback(name: string, _: string, newValue: string): void {
    if (name === 'current_query') {
      // Convert the current_query string into an object.
      this._query = toParamSet(newValue);

      // Remove invalid key/value pairs from the new query.
      this._rationalizeQuery();

      // This updates query-value-sk with the new selection and renders the template.
      if (this._connected) {
        this._keyChange();
      }
    } else {
      this._render();
    }
  }
}

define('query-sk', QuerySk);
