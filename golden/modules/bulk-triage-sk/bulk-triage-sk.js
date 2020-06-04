/**
 * @module module/bulk-triage-sk
 * @description <h2><code>bulk-triage-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import 'elements-sk/checkbox-sk';
import 'elements-sk/icon/cancel-icon-sk';
import 'elements-sk/icon/check-circle-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/icon/view-agenda-icon-sk';
import 'elements-sk/styles/buttons';
import { sendBeginTask, sendEndTask, sendFetchError } from '../common';

const POSITIVE = 'positive';
const NEGATIVE = 'negative';
const UNTRIAGED = 'untriaged';
const CLOSEST = 'closest';

const template = (el) => html`
<h2>Bulk Triage</h2>
<p>Assign the status to all images on this page at once.</p>
<p>${el.changeListID ? `This affects ChangeList ${el.changeListID}.` : ''}</p>
<div class=status>
  <button class="positive ${el.value === POSITIVE ? 'selected' : ''}"
          @click=${() => el._setDesiredLabel(POSITIVE)}>
    <check-circle-icon-sk></check-circle-icon-sk>
  </button>
  <button class="negative ${el.value === NEGATIVE ? 'selected' : ''}"
          @click=${() => el._setDesiredLabel(NEGATIVE)}>
    <cancel-icon-sk></cancel-icon-sk>
  </button>
  <button class="untriaged ${el.value === UNTRIAGED ? 'selected' : ''}"
          @click=${() => el._setDesiredLabel(UNTRIAGED)}>
    <help-icon-sk></help-icon-sk>
  </button>
  <button class="closest ${el.value === CLOSEST ? 'selected' : ''}"
          @click=${() => el._setDesiredLabel(CLOSEST)}>
    <view-agenda-icon-sk></view-agenda-icon-sk>
  </button>
</div>

<div>
  <checkbox-sk @change=${el._toggleAll} label="Triage all ${el._allDigestCount} digests"
     title='Choose whether to triage just the digests on this page or all that match the query'
     ?checked=${el._triageAll}></checkbox-sk>
</div>

<div class=controls>
    <button @click=${el._cancel}>
        Cancel (do nothing)
    </button>
    <button @click=${el._triage}>
        Triage ${el._triageAll ? el._allDigestCount : el._pageDigestCount} digests as ${el._value}.
    </button>
</div>
`;

function countDigests(map) {
  let count = 0;
  for (const testName of Object.keys(map)) {
    count += Object.keys(map[testName]).length;
  }
  return count;
}

define('bulk-triage-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._changeListID = '';
    this._value = CLOSEST;
    this._triageAll = false;

    this._pageDigests = {};
    this._pageDigestCount = 50;
    this._allDigests = {};
    this._allDigestCount = 117;
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  /** @prop value {string} One of "untriaged", "positive" or "negative". */
  get value() {
    return this._value;
  }

  set value(newValue) {
    if (![POSITIVE, NEGATIVE, UNTRIAGED, CLOSEST].includes(newValue)) {
      throw new RangeError(`Invalid bulk-triage-sk value: "${newValue}".`);
    }
    this._value = newValue;
    this._render();
  }

  /** @prop value {string} If not empty, the id of the ChangeList to which these expectations
   *     should belong */
  get changeListID() {
    return this._changeListID;
  }

  set changeListID(newValue) {
    this._changeListID = newValue;
    this._render();
  }


  setDigests(pageDigests, allDigests) {
    this._pageDigests = pageDigests;
    this._pageDigestCount = countDigests(pageDigests);
    this._allDigests = allDigests;
    this._allDigestCount = countDigests(allDigests);
  }

  _setDesiredLabel(newValue) {
    this.value = newValue;
  }

  _cancel(e) {
    this.dispatchEvent(new CustomEvent('bulk_triage_cancelled', { bubbles: true }));
  }

  _getTriageStatuses() {
    let baseDigests = this._pageDigests;
    if (this._triageAll) {
      baseDigests = this._allDigests;
    }
    if (this.value === CLOSEST) {
      return baseDigests;
    }
    const copyWithSameValue = {};
    for (const testName of Object.keys(baseDigests)) {
      copyWithSameValue[testName] = {};
      for (const digest of Object.keys(baseDigests[testName])) {
        copyWithSameValue[testName][digest] = this.value;
      }
    }
    return copyWithSameValue;
  }

  _triage(e) {
    const triageStatuses = this._getTriageStatuses();
    sendBeginTask(this);
    fetch('/json/triage', {
      method: 'POST',
      body: JSON.stringify({
        testDigestStatus: triageStatuses,
        issue: this.changeListID,
      }),
    }).then(() => {
      // Even if we get back a non-200 code, we want to say we finished.
      this.dispatchEvent(new CustomEvent('bulk_triage_finished', { bubbles: true }));
      sendEndTask(this);
    }).catch((e) => sendFetchError(this, e, 'bulk triaging'));
    this.dispatchEvent(new CustomEvent('bulk_triage_invoked', { bubbles: true }));
  }

  _toggleAll(e) {
    e.preventDefault();
    this._triageAll = !this._triageAll;
    this._render();
  }
});
