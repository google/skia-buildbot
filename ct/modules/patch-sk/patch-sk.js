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
import '../../../infra-sk/modules/confirm-dialog-sk';
import '../../../infra-sk/modules/expandable-textarea-sk';

import { $$ } from 'common-sk/modules/dom';
import { fromObject } from 'common-sk/modules/query';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { html } from 'lit-html';
import * as ctfe_utils from '../ctfe_utils';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../input-sk';

const template = (ele) => html`
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

define('patch-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._upgradeProperty('patchType');
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this._spinner = $$('spinner-sk', this);
  }

  _clChanged(e) {
    const newValue = e.target.value;
    this.cl = newValue;
    if (!newValue || newValue.length < 3) {
      this._clData = null;
      this._clDescription = this._formattedClDescription();
      this._spinner.active = false;
      this._render();
      return;
    }
    this._spinner.active = true;
    const queryParams = { cl: newValue };
    const url = '/_/cl_data?' + `${fromObject(queryParams)}`;

    fetch(url, { method: 'POST' })
      .then(jsonOrThrow)
      .then((json) => {
        // If the response is for the value still present in the input we
        // apply it.
        if (this.cl === newValue) {
          if (json.cl) {
            this._clData = json;
            const patch = this._clData[`${this.patchType}_patch`];
            if (!patch) {
              this._clData = { error: { message: `This is not a ${this.patchType} CL.` } };
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
          this._clData = { error: err };
          this._clLoadError();
        }
      })
      .finally(() => {
        if (this.cl === newValue) {
          this.clDescription = this._formattedClDescription();
          this._spinner.active = false;
        }
        this._render();
      });
  }

  /**
   * @returns {boolean} True if a patch is successfully loaded.
   * Trigger errorMessage event otherwise.
   */
  validate() {
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
   * @prop {string} cl - Raw value of the CL input.
   */
  get cl() {
    return this._cl || '';
  }

  set cl(val) {
    this._cl = val;
  }

  /**
   * @prop {string} clDescription - Humnan readable description of CL.  Fires
   * 'cl-desc-changed' with detail { clDescription: <new desc> } event on
   * change.
   */
  get clDescription() {
    return this._clDescription || '';
  }

  set clDescription(val) {
    this._clDescription = val;
    // shadow dom, do we need composed: true?
    this.dispatchEvent(new CustomEvent('cl-description-changed',
      { bubbles: true, detail: { clDescription: val } }));
  }

  /**
   * @prop {string} patch - The patch, either retrieved from the CL or
   * manually entered/modified.
   */
  get patch() {
    return $$('expandable-textarea-sk', this).value || '';
  }

  set patch(val) {
    $$('expandable-textarea-sk', this).value = val;
    this._patchChanged();
  }

  /**
   * @prop {string} patchType - Specifies the project for the patch. Must be
   * set. Possible values include "chromium" and "skia". Mirrors the attribute.
   */
  get patchType() {
    return this.getAttribute('patchType');
  }

  set patchType(val) {
    this.setAttribute('patchType', val);
  }

  _clUrl() {
    if (this._clData && !this._clData.error) {
      return this._clData.url;
    }
    return 'javascript:void(0);';
  }

  _formattedClData() {
    if (this._clData && !this._clData.error) {
      return `${this._clData.subject} (modified `
        + `${ctfe_utils.getFormattedTimestamp(this._clData.modified)})`;
    }
    return '';
  }

  _formattedClError() {
    if (this._clData && this._clData.error) {
      return this._clData.error.message || JSON.stringify(this._clData.error);
    }
    return '';
  }

  _formattedClDescription() {
    if (this._clData && !this._clData.error) {
      return `${this._clUrl()} (${this._clData.subject})`;
    }
    return '';
  }

  _clLoadError() {
    errorMessage(`Unable to load ${this.patchType} CL ${this.cl}`
    + '. Please specify patches manually.');
  }

  _patchFetchError() {
    errorMessage(`Unable to fetch ${this.patchType} patch from CL ${this.cl}`
    + '. Please specify patches manually.');
  }

  _patchChanged() {
    this.dispatchEvent(new CustomEvent('patch-changed',
      { bubbles: true, detail: { patch: this.patch } }));
  }
});
