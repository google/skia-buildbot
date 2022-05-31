/**
 * @fileoverview A custom element that loads a patch based on provided CL or
 * manually entered git diff.
 *
 * @attr {string} patchType - Specifies the project for the patch. Must be
 * set. Supported values include "chromium" and "skia".
 *
 * @event cl-description-changed - Any time a different (or invalid) CL is
 * loaded
 *
 * @event patch-changed - Any time the patch changed, either due to loading a
 * new CL patchset, or manual editing of the patch field.
 */

import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/cancel-icon-sk';
import 'elements-sk/icon/check-circle-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/spinner-sk';
import 'elements-sk/toast-sk';
import '../../../infra-sk/modules/expandable-textarea-sk';

import { SpinnerSk } from 'elements-sk/spinner-sk/spinner-sk';
import { $$ } from 'common-sk/modules/dom';
import { fromObject } from 'common-sk/modules/query';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { html } from 'lit-html';
import * as ctfe_utils from '../ctfe_utils';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../input-sk';
import { CLDataResponse } from '../json';
import { ExpandableTextArea } from '../pageset-selector-sk/pageset-selector-sk';

export class PatchSk extends ElementSk {
  private _spinner: SpinnerSk | null = null;

  private _cl: string = '';

  private _clData: CLDataResponse | null = null;

  private _clDescription: string = '';

  private _clError: Error | null = null;

  constructor() {
    super(PatchSk.template);
    this._upgradeProperty('patchType');
  }

  private static template = (ele: PatchSk) => html`
  <table>
    <tr>
      <td>CL:</td>
      <td>
        <input-sk @input=${ele._clChanged}
          label="Please paste a complete Gerrit URL"></input-sk>
      </td>
      <td>
        <div class=cl-detail-container>
          <div class="cl-detail">
            <spinner-sk alt="Loading CL details"></spinner-sk>
          </div>
          <div class="cl-detail">
            <a href=${ele._clUrl()} target=_blank>${ele._formattedClData()}</a>
            <span class="cl-error">${ele._formattedClError()}</span>
          </div>
        </div>
      </td>
    </tr>
    <tr>
      <td colspan=3 class=patch-manual>
        <expandable-textarea-sk displaytext="Specify Patch Manually" @input=${ele._patchChanged}>
        </expandable-textarea-sk>
      </td>
    </tr>
  </table>
  `;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this._spinner = $$('spinner-sk', this);
  }

  _clChanged(e: Event): void {
    const newValue = (e.target as HTMLInputElement).value;
    this.cl = newValue;
    if (!newValue || newValue.length < 3) {
      this._clData = null;
      this._clError = null;
      this._clDescription = this._formattedClDescription();
      this._spinner!.active = false;
      this._render();
      return;
    }
    this._spinner!.active = true;
    const queryParams = { cl: newValue };
    const url = `/_/cl_data?${fromObject(queryParams)}`;

    fetch(url, { method: 'POST' })
      .then(jsonOrThrow)
      .then((json: CLDataResponse) => {
        // If the response is for the value still present in the input we
        // apply it.
        if (this.cl === newValue) {
          if (json.cl) {
            this._clData = json;
            const patch = this._clData![`${this.patchType}_patch` as keyof CLDataResponse];
            if (!patch) {
              this._clError = new Error(`This is not a ${this.patchType} CL.`);
              this._patchFetchError();
            } else {
              this.patch = patch;
            }
          } else {
            this._clData = null;
          }
        }
      })
      .catch((err) => {
        if (this.cl === newValue) {
          this._clError = err;
          this._clLoadError();
        }
      })
      .finally(() => {
        if (this.cl === newValue) {
          this.clDescription = this._formattedClDescription();
          this._spinner!.active = false;
        }
        this._render();
      });
  }

  /**
   * @returns {boolean} True if a patch is successfully loaded.
   * Trigger errorMessage event otherwise.
   */
  validate(): boolean {
    if (this.cl && !this._clData) {
      this._clLoadError();
      return false;
    }
    if (this.cl && !this.patch) {
      this._patchFetchError();
      return false;
    }
    return true;
  }

  /**
   * Expands the text area if it is collapsed.
   */
  expandTextArea(): void {
    const exTextarea = $$('expandable-textarea-sk', this) as ExpandableTextArea;
    if (!exTextarea.open) {
      ($$('button', exTextarea) as HTMLElement).click();
    }
  }

  /**
   * @prop {string} cl - Raw value of the CL input.
   */
  get cl(): string {
    return this._cl || '';
  }

  set cl(val: string) {
    this._cl = val;
  }

  /**
   * @prop {string} clDescription - Human readable description of CL.  Fires
   * 'cl-description-changed' with detail { clDescription: <new desc> } event on
   * change.
   */
  get clDescription(): string {
    return this._clDescription || '';
  }

  set clDescription(val: string) {
    this._clDescription = val;
    // shadow dom, do we need composed: true?
    this.dispatchEvent(new CustomEvent('cl-description-changed',
      { bubbles: true, detail: { clDescription: val } }));
  }

  /**
   * @prop {string} patch - The patch, either retrieved from the CL or
   * manually entered/modified.
   */
  get patch(): string {
    return ($$('expandable-textarea-sk', this) as HTMLInputElement).value || '';
  }

  set patch(val: string) {
    ($$('expandable-textarea-sk', this) as HTMLInputElement).value = val;
    this._patchChanged();
  }

  /**
   * @prop {string} patchType - Specifies the project for the patch. Must be
   * set. Possible values include "chromium" and "skia". Mirrors the attribute.
   */
  get patchType(): string {
    return this.getAttribute('patchType')!;
  }

  set patchType(val: string) {
    this.setAttribute('patchType', val!);
  }

  _clUrl(): string {
    if (this._clData && !this._clError) {
      return this._clData.url;
    }
    return 'javascript:void(0);';
  }

  _formattedClData(): string {
    if (this._clData && !this._clError) {
      return `${this._clData.subject} (modified `
        + `${ctfe_utils.getFormattedTimestamp(+this._clData.modified)})`;
    }
    return '';
  }

  _formattedClError(): string {
    if (this._clData && this._clError) {
      return this._clError.message || JSON.stringify(this._clError);
    }
    return '';
  }

  _formattedClDescription(): string {
    if (this._clData && !this._clError) {
      return `${this._clUrl()} (${this._clData.subject})`;
    }
    return '';
  }

  _clLoadError(): void {
    errorMessage(`Unable to load ${this.patchType} CL ${this.cl}`
    + '. Please specify patches manually.');
  }

  _patchFetchError(): void {
    errorMessage(`Unable to fetch ${this.patchType} patch from CL ${this.cl}`
    + '. Please specify patches manually.');
  }

  _patchChanged(): void {
    this.dispatchEvent(new CustomEvent('patch-changed',
      { bubbles: true, detail: { patch: this.patch } }));
  }
}

define('patch-sk', PatchSk);
