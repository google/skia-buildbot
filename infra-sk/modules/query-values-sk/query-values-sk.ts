/**
 * @module modules/query-values-sk
 * @description <h2><code>query-values-sk</code></h2>
 *
 * The right-hand side of the query-sk element, the values for a single key
 * in a query/paramset.
 *
 * @evt query-values-changed - Triggered only when the selections have actually
 *     changed. The selection is available in e.detail.
 *
 * @attr {boolean} hide_invert - If the option to invert a query should be made available to
 *       the user.
 * @attr {boolean} hide_regex - If the option to include regex in the query should be made
 *       available to the user.
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { CheckOrRadio } from '../../../elements-sk/modules/checkbox-sk/checkbox-sk';
import {
  MultiSelectSk,
  MultiSelectSkSelectionChangedEventDetail,
} from '../../../elements-sk/modules/multi-select-sk/multi-select-sk';
import { ElementSk } from '../ElementSk';

import '../../../elements-sk/modules/checkbox-sk';
import '../../../elements-sk/modules/multi-select-sk';

export interface QueryValuesSkQueryValuesChangedEventDetail {
  invert: boolean;
  regex: boolean;
  values: string[];
}

export class QueryValuesSk extends ElementSk {
  private static template = (ele: QueryValuesSk) => html`
    <checkbox-sk
      id="invert-${ele.uniqueId}"
      @change=${ele._invertChange}
      title="Match items not selected below."
      label="Invert"
      ?hidden=${ele.hide_invert}></checkbox-sk>
    <checkbox-sk
      id="regex-${ele.uniqueId}"
      class="regex"
      @change=${ele._regexChange}
      title="Match items via regular expression."
      label="Regex"
      ?hidden=${ele.hide_regex}></checkbox-sk>
    <input
      type="text"
      id="regexValue-${ele.uniqueId}"
      class="regexValue"
      @input=${ele._regexInputChange} />
    <div class="filtering">
      <input
        id="filter-${ele.uniqueId}"
        @input=${ele._fastFilter}
        placeholder="Filter Values"
        name="query-value-sk-filter-val"
        autocomplete="off" />
      ${QueryValuesSk.clearFilterButton(ele)}
    </div>
    <multi-select-sk
      id="values-${ele.uniqueId}"
      class="values"
      @selection-changed=${ele._selectionChange}>
      ${QueryValuesSk.valuesTemplate(ele)}
    </multi-select-sk>
  `;

  private static valuesTemplate = (ele: QueryValuesSk) =>
    ele.options.map(
      (v) => html` <div value=${v} ?selected=${ele._selected.indexOf(v) !== -1}>${v}</div> `
    );

  private static clearFilterButton(ele: QueryValuesSk) {
    if (!ele._filtering) {
      return html``;
    }
    return html`
      <button @click=${ele._clearFilter} class="clear_filters" title="Clear filter">&cross;</button>
    `;
  }

  private _clearFilter(): void {
    this._filterInput!.value = '';
    this._filtering = false;
    this._render();
  }

  public clearFilter(): void {
    this._clearFilter();
  }

  private _options: string[] = [];

  private _filteredOptions: string[] = [];

  private _selected: string[] = [];

  private _filtering: boolean = false;

  private _invert: CheckOrRadio | null = null;

  private _regex: CheckOrRadio | null = null;

  private _regexValue: HTMLInputElement | null = null;

  private _filterInput: HTMLInputElement | null = null;

  private _values: MultiSelectSk | null = null;

  private static nextUniqueId = 0;

  readonly uniqueId = `${QueryValuesSk.nextUniqueId++}`;

  constructor() {
    super(QueryValuesSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this._invert = this.querySelector(`#invert-${this.uniqueId}`);
    this._regex = this.querySelector(`#regex-${this.uniqueId}`);
    this._values = this.querySelector(`#values-${this.uniqueId}`);
    this._regexValue = this.querySelector(`#regexValue-${this.uniqueId}`);
    this._filterInput = this.querySelector(`#filter-${this.uniqueId}`);
    this._upgradeProperty('options');
    this._upgradeProperty('selected');
    this._upgradeProperty('hide_invert');
    this._upgradeProperty('hide_regex');
  }

  private _invertChange() {
    if (this._regex!.checked) {
      this._regex!.checked = false;
    }
    this._render();
    this._fireEvent();
  }

  private _regexChange() {
    if (this._invert!.checked) {
      this._invert!.checked = false;
    }

    this._render();
    this._fireEvent();
  }

  /**
   * Filter the options displayed based on text entered in the
   * filter text box
   */
  private _fastFilter(): void {
    const filterString = this._filterInput!.value.trim();
    const filters = filterString.toLowerCase().split(/\s+/);

    if (filterString) {
      this._filtering = true;
      this._filteredOptions = [];
      // Create a closure that returns true if the given label matches the filter.
      const matches = (s: string): boolean => {
        s = s.toLowerCase();
        return filters.filter((f) => s.indexOf(f) > -1).length > 0;
      };

      // Only add the values that match the filter text
      this._filteredOptions = Object.values(this._options).filter(matches);
    } else {
      this._filtering = false;
    }
    this._render();
  }

  private _regexInputChange() {
    this._fireEvent();
  }

  private _selectionChange(e: CustomEvent<MultiSelectSkSelectionChangedEventDetail>) {
    this._selected = e.detail.selection.map((i) => this.options[i]);
    this._render();
    this._fireEvent();
  }

  private _fireEvent() {
    const prefix = this._invert!.checked ? '!' : '';
    let selected = this._values!.selection.map((i) => prefix + this.options[i]);
    if (this._regex!.checked) {
      selected = [`~${this._regexValue!.value}`];
    }
    this.dispatchEvent(
      new CustomEvent<QueryValuesSkQueryValuesChangedEventDetail>('query-values-changed', {
        detail: {
          invert: this._invert!.checked,
          regex: this._regex!.checked,
          values: selected,
        },
        bubbles: true,
      })
    );
  }

  /** Mirrors the hide_invert attribute. */
  get hide_invert() {
    return this.hasAttribute('hide_invert');
  }

  set hide_invert(val) {
    if (val) {
      this.setAttribute('hide_invert', '');
    } else {
      this.removeAttribute('hide_invert');
    }
    this._render();
  }

  /** Mirrors the hide_regex attribute. */
  get hide_regex() {
    return this.hasAttribute('hide_regex');
  }

  set hide_regex(val) {
    if (val) {
      this.setAttribute('hide_regex', '');
    } else {
      this.removeAttribute('hide_regex');
    }
    this._render();
  }

  /** The available options. */
  get options() {
    if (this._filtering) {
      return this._filteredOptions;
    }

    return this._options;
  }

  set options(val) {
    this._options = val;
    this._selected = [];

    // Perform filtering for the values when
    // updating the options
    this._fastFilter();
  }

  /** Current selections. */
  get selected() {
    return this._selected;
  }

  set selected(val) {
    this._selected = val;
    this._invert!.checked = !!(this._selected.length >= 1 && this._selected[0][0] === '!');
    this._regex!.checked = !!(this._selected.length === 1 && this._selected[0][0] === '~');
    this._cleanSelected();
    if (this._selected!.length && this._regex!.checked) {
      this._regexValue!.value = this._selected[0];
    }
    this._render();
  }

  private _cleanSelected() {
    // Remove prefixes from _selected.
    this._selected = this._selected.map((val) => {
      if ('~!'.includes(val[0])) {
        return val.slice(1);
      }
      return val;
    });
  }

  static get observedAttributes() {
    return ['hide_invert', 'hide_regex'];
  }

  attributeChangedCallback() {
    this._render();
  }
}

define('query-values-sk', QueryValuesSk);
