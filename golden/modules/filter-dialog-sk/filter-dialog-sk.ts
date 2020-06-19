/**
 * @module modules/filter-dialog-sk
 * @description <h2><code>filter-dialog-sk</code></h2>
 *
 * A dialog that provides input elements to filter search results by metric values.
 *
 * Events:
 *   edit: Emitted when the user clicks the "Filter" button (and closes the dialog in the process).
 *         The "detail" field of the event contains the filter values entered by the user.
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { live } from 'lit-html/directives/live';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import dialogPolyfill from 'dialog-polyfill';
import { $$ } from 'common-sk/modules/dom';
import { ParamSet, fromParamSet, toParamSet } from 'common-sk/modules/query';
import { QueryDialogSk } from '../query-dialog-sk/query-dialog-sk';

import 'elements-sk/styles/buttons';
import 'elements-sk/checkbox-sk';
import '../query-dialog-sk';
import '../../../infra-sk/modules/paramset-sk';


export interface FilterDialogSkValue {
  diffConfig: ParamSet;
  minRGBADelta: number;
  maxRGBADelta: number;
  maxDiff: number;
  metric: 'pixel' | 'percent' | 'combined';
  sortOrder: 'ascending' | 'descending';
  mustHaveReferenceImage: boolean;
};

// This template produces an <input type=range> (a "slider") and an <input type=number> that
// reflect each other's values. When one changes, the other is updated with the new value, and
// viceversa.
const numericParamTemplate = (id: string,
                              label: string,
                              setterFn: (value: number) => void,
                              value: number = 0,
                              min: number,
                              max: number,
                              step: number) => {
  const onInput = (e: InputEvent) => {
    const target = e.target as HTMLInputElement;

    // Set the corresponding field in the FilterDialogSk instance.
    setterFn(parseFloat(target.value));

    // Sync up the range and number inputs.
    const otherSelector = `input[type="${target.type === 'range' ? 'number' : 'range'}"]`;
    const other = target.parentElement!.querySelector<HTMLInputElement>(otherSelector)!;
    other.value = target.value;
  };
  return html`
    <label for="${id}">${label}</label>
    <div class=numeric-param
         id=${id}-numeric-param>
      <input type=range
             id=${id}
             .value=${live(value)}
             min=${min}
             max=${max}
             step=${step}
             @input=${onInput}/>
      <input type=number
             .value=${live(value)}
             min=${min}
             max=${max}
             step=${step}
             @input=${onInput}/>
    </div>`;
}

export class FilterDialogSk extends ElementSk {

  private static _template = (el: FilterDialogSk) => html`
    <query-dialog-sk .submitButtonLabel=${'Select'}
                     @edit=${el._queryDialogEdit}>
    </query-dialog-sk>

    <dialog class=filter-dialog>
      <div class=content>
        <span class=label>Right hand query:</span>
        <div class=right-hand-query>
          <div class=query>
            ${!el._value || Object.keys(el._value!.diffConfig).length === 0
              ? html`<div class=empty-placeholder>Empty.</div>`
              : html`<paramset-sk .paramsets=${[el._value?.diffConfig]}></paramset-sk>`}
          </div>
          <button class=edit-query @click=${el._openQueryDialog}>Edit query</button>
        </div>

        ${numericParamTemplate(
          'min-rgba-delta',
          'Min RGBA delta:',
          /* setterFn= */ (val) => el._value!.minRGBADelta = val,
          /* value= */ el._value?.minRGBADelta,
          /* min= */ 0,
          /* max= */ 255,
          /* step= */ 1)}

        ${numericParamTemplate(
          'max-rgba-delta',
          'Max RGBA delta:',
          /* setterFn= */ (val) => el._value!.maxRGBADelta = val,
          /* value= */ el._value?.maxRGBADelta,
          /* min= */ 0,
          /* max= */ 255,
          /* step= */ 1)}

        ${numericParamTemplate(
          'max-diff',
          'Max diff:',
          /* setterFn= */ (val) => el._value!.maxDiff = val,
          /* value= */ el._value?.maxDiff,
          /* min= */ -1,
          /* max= */ 1,
          /* step= */ 0.05)}

        <label for=metric>Metric:</label>
        <select id=metric
                .value=${el._value?.metric}
                @change=${el._metricChanged}>
          <option value=combined>Combined</option>
          <option value=percent>Percent</option>
          <option value=pixel>Pixel</option>
        </select>

        <label for=sort-order>Sort order:</label>
        <select id=sort-by
                .value=${el._value?.sortOrder}
                @change=${el._sortOrderChanged}>
          <option value=ascending>Ascending</option>
          <option value=descending>Descending</option>
        </select>

        <checkbox-sk id=must-have-reference-image
                     label="Must have a reference image."
                     ?checked=${live(el._value?.mustHaveReferenceImage)}
                     @change=${el._mustHaveReferenceImageChanged}>
        </checkbox-sk>
      </div>

      <div class=buttons>
        <button class="filter action" @click=${el._filterBtnClicked}>Filter</button>
        <button class=cancel @click=${el._cancelBtnClicked}>Cancel</button>
      </div>
    </dialog>`;

  private _dialog: HTMLDialogElement | null = null;
  private _queryDialogSk: QueryDialogSk | null = null;

  private _paramSet: ParamSet | null = null;
  private _value: FilterDialogSkValue | null = null;

  constructor() {
    super(FilterDialogSk._template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this._dialog = $$('dialog.filter-dialog', this);
    this._queryDialogSk = $$('query-dialog-sk', this);
    dialogPolyfill.registerDialog(this._dialog!);
  }

  open(paramSet: ParamSet, value: FilterDialogSkValue) {
    this._paramSet = paramSet;
    this._value = value;
    this._render();
    this._dialog?.showModal();
  }

  private _openQueryDialog() {
    this._queryDialogSk!.open(this._paramSet!, fromParamSet(this._value!.diffConfig));
  }

  private _queryDialogEdit(e: CustomEvent<string>) {
    e.stopPropagation(); // Necessary because filter-dialog-sk also emits an "edit" event.
    this._value!.diffConfig = toParamSet(e.detail);
    this._render();
  }

  private _metricChanged(e: InputEvent) {
    const value = (e.target as HTMLSelectElement).value as 'pixel' | 'percent' | 'combined';
    this._value!.metric = value;
  }

  private _sortOrderChanged(e: InputEvent) {
    const value = (e.target as HTMLSelectElement).value as 'ascending' | 'descending';
    this._value!.sortOrder = value;
  }

  private _mustHaveReferenceImageChanged(e: InputEvent) {
    const value = (e.target as HTMLInputElement).checked;
    this._value!.mustHaveReferenceImage = value;
  }

  private _filterBtnClicked() {
    this._dialog!.close();
    this.dispatchEvent(new CustomEvent<FilterDialogSkValue>('edit', {
      bubbles: true,
      detail: this._value!
    }));
  }

  private _cancelBtnClicked() {
    this._dialog!.close();
  }
}

define('filter-dialog-sk', FilterDialogSk);
