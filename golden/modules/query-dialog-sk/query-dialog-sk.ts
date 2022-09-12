/**
 * @module modules/query-dialog-sk
 * @description <h2><code>query-dialog-sk</code></h2>
 *
 * A dialog that shows a query-sk element.
 *
 * Events:
 *
 *   query-dialog-open: Emitted when the dialog is opened.
 *
 *   query-dialog-close: Emitted when the dialog is closed.
 *
 *   edit: Emitted when user clicks the "Show matches" button (and closes the
 *         dialog in the process). The "detail" field of the event contains
 *         the url-encoded selections.
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { $$ } from 'common-sk/modules/dom';
import { ParamSet, toParamSet } from 'common-sk/modules/query';
import { QuerySk, QuerySkQueryChangeEventDetail } from '../../../infra-sk/modules/query-sk/query-sk';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import '../../../infra-sk/modules/query-sk';
import '../../../infra-sk/modules/paramset-sk';
import 'elements-sk/styles/buttons';

export class QueryDialogSk extends ElementSk {
  private static _template = (el: QueryDialogSk) => html`
    <dialog>
      <div class=content>
        <query-sk @query-change=${el.queryChange}
                  hide_invert
                  hide_regex></query-sk>
        <div class=selection-summary>
          ${el.isSelectionEmpty()
    ? html`<p class=empty-selection>No items selected.</p>`
    : html`<paramset-sk .paramsets=${[el.currentSelection]}></paramset-sk>`}
        </div>
      </div>

      <div class=buttons>
        <button class="show-matches action" @click=${el.showMatches}>
          ${el._submitButtonLabel}
        </button>
        <button class=cancel @click=${el.close}>Cancel</button>
      </div>
    </dialog>`;

  private dialog: HTMLDialogElement | null = null;

  private querySk: QuerySk | null = null;

  private _submitButtonLabel: string = 'Show Matches';

  constructor() {
    super(QueryDialogSk._template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this.dialog = $$('dialog', this);
    this.querySk = $$('query-sk', this);
  }

  open(paramSet: ParamSet, selection: string) {
    this.querySk!.paramset = paramSet;
    this.querySk!.current_query = selection;

    this._render();
    this.dialog!.showModal();
    this.dispatchEvent(new CustomEvent('query-dialog-open', { bubbles: true }));
  }

  /** Can be used to customize the label of the submit button. */
  get submitButtonLabel() { return this._submitButtonLabel; }

  set submitButtonLabel(label: string) {
    // This is used by filter-dialog-sk to change the button label to "Submit".
    this._submitButtonLabel = label;
    this._render();
  }

  private queryChange(e: CustomEvent<QuerySkQueryChangeEventDetail>) {
    // This updates the paramset-sk with the new selection from the query-sk component.
    this._render();
  }

  private showMatches() {
    this.dialog!.close();
    this.dispatchEvent(new CustomEvent<string>('edit', {
      bubbles: true,
      detail: this.querySk!.current_query,
    }));
    this.dispatchEvent(new CustomEvent('query-dialog-close', { bubbles: true }));
  }

  private close() {
    this.dialog!.close();
    this.dispatchEvent(new CustomEvent('query-dialog-close', { bubbles: true }));
  }

  private get currentSelection() {
    return this.querySk ? toParamSet(this.querySk.current_query) : {};
  }

  private isSelectionEmpty() {
    return Object.keys(this.currentSelection).length === 0;
  }
}

define('query-dialog-sk', QueryDialogSk);
