/**
 * @module modules/trace-filter-sk
 * @description <h2><code>trace-filter-sk</code></h2>
 *
 * An elements that allows the user to select a set of key/value pairs that traces must match.
 *
 * Events:
 *
 *   trace-filter-sk-change: Emitted when the user changes the trace filter.
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { ParamSet, fromParamSet, toParamSet } from 'common-sk/modules/query';
import { $$ } from 'common-sk/modules/dom';
import { QueryDialogSk } from '../query-dialog-sk/query-dialog-sk';

import 'elements-sk/styles/buttons'
import '../../../infra-sk/modules/query-sk';
import '../../../infra-sk/modules/paramset-sk';
import '../query-dialog-sk';

export class TraceFilterSk extends ElementSk {
  private static template = (el: TraceFilterSk) => html`
    <div class=selection>
      ${Object.keys(el._selection).length === 0
        ? html`<div class=empty-placeholder>Empty.</div>`
        : html`<paramset-sk .paramsets=${[el._selection]}></paramset-sk>`}
    </div>
    <button class=edit-query @click=${el._onEditQueryBtnClick}>Edit query</button>

    <query-dialog-sk .submitButtonLabel=${'Select'}
                     @edit=${el._onQueryDialogEdit}>
    </query-dialog-sk>`;

  private _paramSet: ParamSet = {};
  private _selection: ParamSet = {};

  private _queryDialogSk: QueryDialogSk | null = null;

  constructor() {
    super(TraceFilterSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this._queryDialogSk = $$('query-dialog-sk', this);
  }

  get paramSet() { return this._paramSet; }

  set paramSet(value) {
    this._paramSet = value;
    this._render();
  }

  get selection() { return this._selection; }

  set selection(value) {
    this._selection = value;
    this._render();
  }

  private _onEditQueryBtnClick() {
    this._queryDialogSk!.open(this._paramSet, fromParamSet(this._selection));
  }

  private _onQueryDialogEdit(e: CustomEvent<string>) {
    e.stopPropagation();
    this._selection = toParamSet(e.detail);
    this._render();
    this.dispatchEvent(new CustomEvent<ParamSet>('trace-filter-sk-change', {
      detail: this._selection,
      bubbles: true
    }));
  }
};

define('trace-filter-sk', TraceFilterSk);
