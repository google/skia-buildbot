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
import { html, LitElement } from 'lit';
import { customElement, property, state, query } from 'lit/decorators.js';
import { ParamSet, toParamSet, fromParamSet } from '../query';
import { SelectSk } from '../../../elements-sk/modules/select-sk/select-sk';
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

@customElement('query-sk')
export class QuerySk extends LitElement {
  @property({ attribute: false })
  get paramset(): ParamSet {
    return this._paramset;
  }

  set paramset(val: ParamSet) {
    this._paramset = val;
    this._originalParamset = val;
    this._recalcKeys();
    if (this._fastInput && this._fastInput.value.trim() !== '') {
      this._fastFilter(this._fastInput.value);
    }
  }

  @property({ attribute: 'current_query', reflect: true })
  get current_query(): string {
    return fromParamSet(this._query);
  }

  set current_query(val: string) {
    const oldVal = this.current_query;
    if (val === oldVal) return;
    this._query = toParamSet(val);
    this._rationalizeQuery();
    this.requestUpdate('current_query', oldVal);
  }

  @property({ attribute: false })
  get key_order(): string[] {
    return this._key_order;
  }

  set key_order(val: string[]) {
    this._key_order = val;
    this._recalcKeys();
    this.requestUpdate();
  }

  @property({ type: Boolean, reflect: true })
  hide_invert: boolean = false;

  @property({ type: Boolean, reflect: true })
  hide_regex: boolean = false;

  @property({ type: Boolean, reflect: true })
  values_only: boolean = false;

  @state()
  private _paramset: ParamSet = {};

  @state()
  private _originalParamset: ParamSet = {};

  @state()
  private _keys: string[] = [];

  private _query: ParamSet = {};

  private _key_order: string[] = [];

  @state()
  private _filtering: boolean = false;

  private _delayedTimeout: number | null = null;

  private _lastQuery: string = '';

  @state()
  private _selectedKey: string = '';

  @query('select-sk')
  private _keySelect!: SelectSk;

  @query('query-values-sk')
  private _values!: QueryValuesSk;

  @query('input[name="query-sk-filter"]')
  private _fastInput!: HTMLInputElement;

  createRenderRoot() {
    return this;
  }

  private _recalcKeys(): void {
    const keys = Object.keys(this._paramset);
    keys.sort();
    const pre = this._key_order.filter((ordered) => keys.indexOf(ordered) > -1);
    const post = keys.filter((key) => pre.indexOf(key) === -1);
    this._keys = pre.concat(post);
    if (this._keys.indexOf(this._selectedKey) === -1) {
      this._selectedKey = '';
    }
  }

  private _rationalizeQuery(): void {
    const originalKeys = Object.keys(this._originalParamset);
    Object.keys(this._query).forEach((key) => {
      if (originalKeys.indexOf(key) === -1) {
        delete this._query[key];
      } else {
        this._query[key] = this._query[key].filter(
          (val) =>
            this._originalParamset[key].includes(val) || val.startsWith('~') || val.startsWith('!')
        );
      }
    });
  }

  private _valuesChanged(e: CustomEvent<QueryValuesSkQueryValuesChangedEventDetail>) {
    e.stopPropagation();
    const key = this._selectedKey;
    if (!key) return;

    if (this._fastInput && this._fastInput.value.trim() !== '') {
      // Things get complicated if the user has entered a filter. The user may
      // have selections in this._query[key] which don't appear in e.detail
      // because they have been filtered out, so we should only add/remove
      // values from this._query[key] that appear in this._paramset[key], while
      // being careful because value(s) may be prefixed with either a '!' or a
      // '~' if they are invert or regex queries.
      // When we toggle from a regex to a non-regex we need to clear the values.
      if (
        !e.detail.regex &&
        this._query[key] &&
        this._query[key].length &&
        this._query[key][0].startsWith('~')
      ) {
        this._query[key] = [];
      }

      if (e.detail.regex) {
        this._query[key] = e.detail.values;
      }

      this._applyInvert(key, e.detail.invert);

      const valuesDisplayed = new Set(this._paramset[key]);
      const currentQueryForKey = new Set(this._query[key]);
      const unprefixedSelectionsFromEvent = new Set(e.detail.values.map(removePrefix));

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
      if (e.detail.regex) {
        this._query[key] = e.detail.values;
      } else {
        const merged = new Set(this._query[key] || []);
        e.detail.values.forEach((v) => merged.add(v));
        this._query[key] = [...merged];
      }
    }
    this._queryChanged();
  }

  private _applyInvert(key: string, invert: boolean): void {
    const values = this._query[key];
    if (!values || !values.length) {
      return;
    }
    const valuesHaveInvert = values[0].startsWith('!');
    if (invert === valuesHaveInvert) {
      return;
    }
    if (invert && !valuesHaveInvert) {
      this._query[key] = values.map((v) => `!${v}`);
    } else {
      this._query[key] = values.map(removePrefix);
    }
  }

  private _queryChanged(): void {
    this._rationalizeQuery();
    const new_query = this.current_query;

    if (this._lastQuery !== new_query) {
      this._lastQuery = new_query;
      this.requestUpdate();
      this.dispatchEvent(
        new CustomEvent<QuerySkQueryChangeEventDetail>('query-change', {
          detail: { q: new_query },
          bubbles: true,
        })
      );

      window.clearTimeout(this._delayedTimeout!);
      this._delayedTimeout = window.setTimeout(() => {
        this.dispatchEvent(
          new CustomEvent<QuerySkQueryChangeEventDetail>('query-change-delayed', {
            detail: { q: new_query },
            bubbles: true,
          })
        );
      }, DELAY_MS);
    }
  }

  private _keyChange(e: CustomEvent) {
    const index = e.detail.selection;
    if (index >= 0 && index < this._keys.length) {
      this._selectedKey = this._keys[index];
    } else {
      this._selectedKey = '';
    }
    if (this._values) {
      this._values.clearFilter();
    }
  }

  private _clear(): void {
    this._query = {};
    this._recalcKeys();
    this._queryChanged();
  }

  private _fastFilterInput(e: Event) {
    const val = (e.target as HTMLInputElement).value;
    this._fastFilter(val);
  }

  private _fastFilter(val?: string): void {
    let filterString = '';
    if (val !== undefined) {
      filterString = val.trim();
    } else if (this._fastInput) {
      filterString = this._fastInput.value.trim();
    }
    const filters = filterString
      .toLowerCase()
      .split(/\s+/)
      .filter((f) => f !== '');

    if (filters.length > 0) {
      this._filtering = true;
    } else {
      this._filtering = false;
    }

    const matches = (s: string) => {
      s = s.toLowerCase();
      if (filters.length === 0) return true;
      return filters.filter((f) => s.indexOf(f) > -1).length > 0;
    };

    const filtered: ParamSet = {};
    Object.keys(this._originalParamset).forEach((paramkey) => {
      if (!this._filtering) {
        filtered[paramkey] = this._originalParamset[paramkey];
        return;
      }

      if (matches(paramkey)) {
        filtered[paramkey] = this._originalParamset[paramkey];
      } else {
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
  }

  private _clearFilterButton() {
    if (!this._filtering) return html``;
    return html`<button @click=${this._clearFilter} class="clear_filters" title="Clear filter">
      &cross;
    </button>`;
  }

  private _clearFilter(): void {
    if (this._fastInput) this._fastInput.value = '';
    this._paramset = this._originalParamset;
    this._filtering = false;
    this._recalcKeys();
  }

  render() {
    const key = this._selectedKey;
    const selectedKeyIndex = this._keys.indexOf(key);

    const options = key ? this._paramset[key] || [] : [];
    const selected = key ? this._query[key] || [] : [];

    return html`
      <div class="filtering">
        <input
          @input=${this._fastFilterInput}
          placeholder="Search Parameters and Values"
          name="query-sk-filter"
          autocomplete="off" />
        ${this._clearFilterButton()}
      </div>
      <div class="bottom">
        <div class="selection" ?hidden=${this.values_only}>
          <select-sk .selection=${selectedKeyIndex} @selection-changed=${this._keyChange}>
            ${this._keys.map((k) => html`<div>${k}</div>`)}
          </select-sk>
          <button @click=${this._clear} class="clear_selections">Clear Selections</button>
        </div>
        <query-values-sk
          class=${!key ? 'hidden' : ''}
          .options=${options}
          .selected=${selected}
          @query-values-changed=${this._valuesChanged}
          ?hide_invert=${this.hide_invert}
          ?hide_regex=${this.hide_regex}></query-values-sk>
      </div>
    `;
  }

  public selectKey(key: string): void {
    if (this._keys.includes(key)) {
      this._selectedKey = key;
    }
  }

  public removeKeyValue(key: string, value: string): void {
    // Remove from _originalParamset to persist the removal through filters.
    if (this._originalParamset[key]) {
      const idx = this._originalParamset[key].indexOf(value);
      if (idx !== -1) {
        this._originalParamset[key].splice(idx, 1);

        // Refilter if needed, or just sync _paramset if not filtering.
        if (this._filtering) {
          this._fastFilter();
        } else {
          this._paramset = this._originalParamset;
          this._recalcKeys();
        }

        this._rationalizeQuery();
        this._queryChanged();
        this.requestUpdate();
      }
    }
  }
}
