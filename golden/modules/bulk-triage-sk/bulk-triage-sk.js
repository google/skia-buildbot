/**
 * @module module/bulk-triage-sk
 * @description <h2><code>bulk-triage-sk</code></h2>
 *
 * An element (meant for use in a dialog) which facilitates triaging multiple digests
 * at once. It supports two modes - all the digests on this page of results or all
 * digests that match the search results.
 *
 * @evt bulk_triage_cancelled - if the cancel button is clicked.
 * @evt bulk_triage_invoked - Sent just before the triage RPC is hit.
 * @evt bulk_triage_finished - Sent if the triage RPC returns success.
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
${el.changeListID ? html`<p>This affects ChangeList ${el.changeListID}.</p>` : ''}
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
     ?checked=${el._triageAll} class=toggle_all></checkbox-sk>
</div>

<div class=controls>
    <button @click=${el._cancel} class=cancel>
        Cancel (do nothing)
    </button>
    <button @click=${el._triage} class="action triage">
        Triage ${el._triageAll ? el._allDigestCount : el._pageDigestCount} digests as ${el._value}
    </button>
</div>
`;

function countDigests(testDigestLabelMap) {
  let count = 0;
  for (const testName of Object.keys(testDigestLabelMap)) {
    count += Object.keys(testDigestLabelMap[testName]).length;
  }
  return count;
}

define('bulk-triage-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._changeListID = '';
    this._crs = '';
    this._value = CLOSEST;
    this._triageAll = false;

    this._pageDigests = {};
    this._pageDigestCount = 0;
    this._allDigests = {};
    this._allDigestCount = 0;
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

  /**
   * @prop crs {string} The Code Review System (e.g. "gerrit") if changeListID is set.
   */
  get crs() { return this._crs; }

  set crs(c) {
    this._crs = c;
    this._render();
  }

  /**
   * Sets the data that would be used to make bulk triage requests. These the same shape as
   * to frontend.TriageRequest.TestDigestStatus. Note that the labels should be set to
   * the closest matching reference digest, so we can apply the "closest" bulk triage.
   * @param pageDigests {Object} - the digests that match this page's data.
   * @param allDigests {Object} - the digests that match the entire search query.
   */
  setDigests(pageDigests, allDigests) {
    this._pageDigests = pageDigests;
    this._pageDigestCount = countDigests(pageDigests);
    this._allDigests = allDigests;
    this._allDigestCount = countDigests(allDigests);
    this._render();
  }

  _setDesiredLabel(newValue) {
    this.value = newValue;
  }

  _cancel() {
    this.dispatchEvent(new CustomEvent('bulk_triage_cancelled', { bubbles: true }));
  }

  /**
   * This creates an object that can be sent to the triage RPC on the Gold server. The labels
   * will be set to match the current value. See frontend.TriageRequest for more.
   * @return {Object}
   */
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

  _triage() {
    const triageStatuses = this._getTriageStatuses();
    sendBeginTask(this);
    this.dispatchEvent(new CustomEvent('bulk_triage_invoked', { bubbles: true }));

    fetch('/json/v1/triage', {
      method: 'POST',
      body: JSON.stringify({
        testDigestStatus: triageStatuses,
        changelist_id: this.changeListID,
        crs: this.crs,
      }),
    }).then(() => {
      // Even if we get back a non-200 code, we want to say we finished.
      this.dispatchEvent(new CustomEvent('bulk_triage_finished', { bubbles: true }));
      sendEndTask(this);
    }).catch((e) => sendFetchError(this, e, 'bulk triaging'));
  }

  _toggleAll(e) {
    e.preventDefault();
    this._triageAll = !this._triageAll;
    this._render();
  }
});
