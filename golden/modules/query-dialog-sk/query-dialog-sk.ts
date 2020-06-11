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
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { QuerySk, QuerySkQueryChangeEventDetail } from '../../../infra-sk/modules/query-sk/query-sk';
import dialogPolyfill from 'dialog-polyfill';
import { $$ } from 'common-sk/modules/dom';
import { ParamSet, toParamSet } from 'common-sk/modules/query';

import '../../../infra-sk/modules/query-sk';
import '../../../infra-sk/modules/paramset-sk';
import 'elements-sk/styles/buttons'

export class QueryDialogSk extends ElementSk {

  private static _template = (el: QueryDialogSk) => html`
    <dialog @click=${el._dialogClick}>
      <div class=content>
        <query-sk @query-change=${el._queryChange}
                  hide_invert
                  hide_regex></query-sk>
        <div class=selection-summary>
          ${el._isSelectionEmpty()
            ? html`<p class=empty-selection>No items selected.</p>`
            : html`<paramset-sk .paramsets=${[el._currentSelection]}></paramset-sk>`}
        </div>
      </div>

      <div class=buttons>
        <button class="show-matches action" @click=${el._showMatches}>
          ${el._submitButtonLabel}
        </button>
        <button class=cancel @click=${el._close}>Cancel</button>
      </div>
    </dialog>`;

  private _dialog: HTMLDialogElement | null = null;
  private _querySk: QuerySk | null = null;
  private _submitButtonLabel: string = 'Show Matches';

  constructor() {
    super(QueryDialogSk._template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this._dialog = $$('dialog', this);
    this._querySk = $$('query-sk', this);
    dialogPolyfill.registerDialog(this._dialog!);
  }

  open(paramSet: ParamSet, selection: string) {
    this._querySk!.paramset = paramSet;
    this._querySk!.current_query = selection;

    this._render();
    this._dialog!.showModal();
    this.dispatchEvent(new CustomEvent('query-dialog-open', {bubbles: true}));
  }

  /** Can be used to customize the label of the submit button. */
  get submitButtonLabel() { return this._submitButtonLabel; }
  set submitButtonLabel(label: string) {
    // This is used by filter-dialog-sk to change the button label to "Submit".
    this._submitButtonLabel = label;
    this._render();
  }

  private _queryChange(e: CustomEvent<QuerySkQueryChangeEventDetail>) {
    // This updates the paramset-sk with the new selection from the query-sk component.
    this._render();
  }

  private _showMatches() {
    this._dialog!.close();
    this.dispatchEvent(new CustomEvent<string>('edit', {
      bubbles: true,
      detail: this._querySk!.current_query
    }));
    this.dispatchEvent(new CustomEvent('query-dialog-close', {bubbles: true}));
  }

  private _close() {
    this._dialog!.close();
    this.dispatchEvent(new CustomEvent('query-dialog-close', {bubbles: true}));
  }

  // This prevents the Polymer filter-dialog-sk component from closing when its nested
  // query-dialog-sk receives a click.
  //
  // TODO(lovisolo): Delete after filter-dialog-sk is ported to lit-html.
  private _dialogClick(e: Event) {
    e.stopPropagation();
  }

  private get _currentSelection() {
    return this._querySk ? toParamSet(this._querySk.current_query) : {};
  }

  private _isSelectionEmpty() {
    return Object.keys(this._currentSelection).length === 0;
  }
}

define('query-dialog-sk', QueryDialogSk);
