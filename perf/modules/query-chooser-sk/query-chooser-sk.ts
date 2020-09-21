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
import { ParamSet, toParamSet } from 'common-sk/modules/query';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { QuerySkQueryChangeEventDetail } from '../../../infra-sk/modules/query-sk/query-sk';

import '../../../infra-sk/modules/paramset-sk';
import '../../../infra-sk/modules/query-sk';

import '../query-count-sk';

import 'elements-sk/styles/buttons';

export class QueryChooserSk extends ElementSk {
  private _dialog: HTMLDivElement | null;

  private _paramset: ParamSet;

  private _key_order: string[];

  constructor() {
    super(QueryChooserSk.template);
    this.current_query = '';
    this._dialog = null;
    this._paramset = {};
    this._key_order = [];
  }

  private static template = (ele: QueryChooserSk) => html`
    <div class="row">
      <button @click=${ele._editClick}>Edit</button>
      <paramset-sk
        id="summary"
        .paramsets=${[toParamSet(ele.current_query)]}
      ></paramset-sk>
    </div>
    <div id="dialog">
      <query-sk
        current_query=${ele.current_query}
        .paramset=${ele.paramset}
        .key_order=${ele.key_order}
        @query-change=${ele._queryChange}
      ></query-sk>
      <div class="matches"
        >Matches:
        <query-count-sk
          url=${ele.count_url}
          current_query=${ele.current_query}
        ></query-count-sk
      ></div>
      <button @click=${ele._closeClick}>Close</button>
    </div>
  `;


  connectedCallback(): any {
    super.connectedCallback();
    this._upgradeProperty('paramset');
    this._upgradeProperty('current_query');
    this._upgradeProperty('key_order');
    this._upgradeProperty('count_url');
    this._render();
    this._dialog = this.querySelector('#dialog');
  }

  attributeChangedCallback(): void {
    this._render();
  }

  private _editClick() {
    this._dialog!.classList.add('display');
  }

  private _closeClick() {
    this._dialog!.classList.remove('display');
  }

  private _queryChange(e: CustomEvent<QuerySkQueryChangeEventDetail>) {
    this.current_query = e.detail.q;
    this._render();
  }

  /** @prop The paramset to make selections from. */
  get paramset(): ParamSet {
    return this._paramset;
  }

  set paramset(val: ParamSet) {
    this._paramset = val;
    this._render();
  }

  /** @prop An array of strings, passed down to
   * query-sk.key_order.
   */
  get key_order(): string[] {
    return this._key_order;
  }

  set key_order(val: string[]) {
    this._key_order = val;
    this._render();
  }

  static get observedAttributes(): string[] {
    return ['current_query', 'key_order'];
  }

  /** @prop Mirrors the current_query attribute.  */
  get current_query(): string {
    return this.getAttribute('current_query') || '';
  }

  set current_query(val: string) {
    this.setAttribute('current_query', val);
  }

  /** @prop Mirrors the count_url attribute. */
  get count_url(): string {
    return this.getAttribute('count_url') || '';
  }

  set count_url(val: string) {
    this.setAttribute('count_url', val);
  }
}

define('query-chooser-sk', QueryChooserSk);
