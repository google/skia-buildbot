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
import { Label, TriageRequest, TriageRequestData } from '../rpc_types';

/**
 * The label to apply to the selected digests via the bulk triage dialog, or 'closest' to apply the
 * label of the closest triaged reference digest.
 */
export type BulkTriageLabel = Label | 'closest';

export class BulkTriageSk extends ElementSk {

  private static template = (el: BulkTriageSk) => html`
    <h2>Bulk Triage</h2>
    <p>Assign the status to all images on this page at once.</p>
    ${el.changeListID ? html`<p>This affects ChangeList ${el.changeListID}.</p>` : ''}
    <div class=status>
      <button class="positive ${el.value === 'positive' ? 'selected' : ''}"
              @click=${() => el._setDesiredLabel('positive')}>
        <check-circle-icon-sk></check-circle-icon-sk>
      </button>
      <button class="negative ${el.value === 'negative' ? 'selected' : ''}"
              @click=${() => el._setDesiredLabel('negative')}>
        <cancel-icon-sk></cancel-icon-sk>
      </button>
      <button class="untriaged ${el.value === 'untriaged' ? 'selected' : ''}"
              @click=${() => el._setDesiredLabel('untriaged')}>
        <help-icon-sk></help-icon-sk>
      </button>
      <button class="closest ${el.value === 'closest' ? 'selected' : ''}"
              @click=${() => el._setDesiredLabel('closest')}>
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

  private _changeListID = '';
  private _crs = '';
  private _value: BulkTriageLabel = 'closest';
  private _triageAll = false;

  private _pageDigests: TriageRequestData = {};
  private _pageDigestCount = 0;
  private _allDigests: TriageRequestData = {};
  private _allDigestCount = 0;

  constructor() {
    super(BulkTriageSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  /**
   * The label to apply ("positive", "negative", "untriaged"), or "closest" to apply the label of
   * of the closest triaged reference digest in each case.
   */
  get value() {
    return this._value;
  }

  set value(newValue) {
    if (!['positive', 'negative', 'untriaged', 'closest'].includes(newValue)) {
      throw new RangeError(`Invalid bulk-triage-sk value: "${newValue}".`);
    }
    this._value = newValue;
    this._render();
  }

  /**
   * The ID of the changelist to which these expectations should belong, or the empty string if
   * none.
   */
  get changeListID() {
    return this._changeListID;
  }

  set changeListID(newValue) {
    this._changeListID = newValue;
    this._render();
  }

  /**
   * The Code Review System (e.g. "gerrit") associated with the provided changelist ID, or the empty
   * string if none.
   */
  get crs() {
    return this._crs;
  }

  set crs(c) {
    this._crs = c;
    this._render();
  }

  // Notes:
  //
  // Currently the /json/v1/search endpoint returns a SearchResponse struct where the
  // bulk_triage_data field is populated with the empty string instead of a valid expectations.Label
  // to indicate that a digest does not have a closest triaged reference digest:
  // https://github.com/google/skia-buildbot/blob/89e3a329bda8f377e24fd0d36dabd715f70bad38/golden/go/search/search.go#L225.
  //
  // An empty string is technically not a valid expectations.Label (nor the corresponding Label type
  // in rpc_types.ts) because the only allowed values are "positive", "negative" or "untriaged".
  //
  // The legacy, Polymer-based search-page-sk passes the contents of bulk_triage_data as-is to this
  // component as the allDigests argument to a call to the setDigests() method defined below:
  // https://github.com/google/skia-buildbot/blob/89e3a329bda8f377e24fd0d36dabd715f70bad38/golden/frontend/res/imp/search-page-sk.html#L346
  //
  // The legacy search page also builds the pageDigests argument to the setDigests() method using
  // the empty string as a Label in the same exact way as in the bulk_triage_data field:
  // https://github.com/google/skia-buildbot/blob/89e3a329bda8f377e24fd0d36dabd715f70bad38/golden/frontend/res/imp/search-page-sk.html#L432
  //
  // To preserve backwards-compatibility with the legacy search page, this component turns a blind
  // eye to said invalid Label values, and passes the labels as-is to the /json/v1/triage endpoint,
  // which ignores any digests for which the expectations.Label is set to the empty string:
  // https://github.com/google/skia-buildbot/blob/89e3a329bda8f377e24fd0d36dabd715f70bad38/golden/go/web/web.go#L970
  //
  // This is messy because neither the Golang nor the TypeScript types involved in the above RPCs
  // capture the possibility of empty labels. In other words, the types do not correctly describe
  // the actual data, which can be a source of confusion and potential bugs in the future.
  //
  // Once we delete the legacy search page, we can clean things up by making the following changes:
  //
  //   1. Change the search RPC to use "untriaged" instead of the empty string to indicate that a
  //      digest does not have a closest triaged reference digest.
  //
  //   2. Change bulk-triage-sk to exclude any such digests from the /json/v1/triage RPC when
  //      triaging by "closest".
  //
  //   3. Delete any code in the /json/v1/triage endpoint that handles empty labels.
  //
  // TODO(lovisolo): Execute the above plan after the legacy search page is deleted.

  /**
   * Deprecated. Use the currentPageDigests and allDigests property setters instead.
   *
   * TODO(lovisolo): Delete after the legacy search-page-sk is removed.
   */
  setDigests(pageDigests: TriageRequestData, allDigests: TriageRequestData) {
    this.currentPageDigests = pageDigests;
    this.allDigests = allDigests;
  }

  /**
   * The digests in the current page of search results, mapped to the labels of their closest
   * triaged reference digests.
   *
   * The labels will be applied when using the "closest" bulk triage option.
   */
  get currentPageDigests() { return this._pageDigests; }

  set currentPageDigests(digests: TriageRequestData) {
    this._pageDigests = digests;
    this._pageDigestCount = this._countDigests(digests);
    this._render();
  }

  /**
   * All the digests matching the current search (not just the ones in the current page of search
   * results), mapped to the labels of their closest triaged reference digests.
   *
   * The labels will be applied when using the "closest" bulk triage option.
   */
  get allDigests() { return this._allDigests; }

  set allDigests(digests: TriageRequestData) {
    this._allDigests = digests;
    this._allDigestCount = this._countDigests(digests);
    this._render();
  }

  private _countDigests(testDigestLabelMap: TriageRequestData) {
    let count = 0;
    for (const testName of Object.keys(testDigestLabelMap)) {
      count += Object.keys(testDigestLabelMap[testName]).length;
    }
    return count;
  }

  private _setDesiredLabel(newValue: BulkTriageLabel) {
    this.value = newValue;
  }

  private _cancel() {
    this.dispatchEvent(new CustomEvent('bulk_triage_cancelled', { bubbles: true }));
  }

  /**
   * This creates an object that can be sent to the triage RPC on the Gold server. The labels
   * will be set to match the current value. See frontend.TriageRequest for more.
   */
  private _getTriageStatuses() {
    let baseDigests = this._pageDigests;
    if (this._triageAll) {
      baseDigests = this._allDigests;
    }
    if (this.value === 'closest') {
      return baseDigests;
    }
    const copyWithSameValue: TriageRequestData = {};
    for (const testName of Object.keys(baseDigests)) {
      copyWithSameValue[testName] = {};
      for (const digest of Object.keys(baseDigests[testName])) {
        copyWithSameValue[testName][digest] = this.value;
      }
    }
    return copyWithSameValue;
  }

  private _triage() {
    const triageRequest: TriageRequest = {
      testDigestStatus: this._getTriageStatuses(),
      changelist_id: this.changeListID,
      crs: this.crs,
    }

    sendBeginTask(this);
    this.dispatchEvent(new CustomEvent('bulk_triage_invoked', { bubbles: true }));
    fetch('/json/v1/triage', {
      method: 'POST',
      body: JSON.stringify(triageRequest)
    }).then(() => {
      // Even if we get back a non-200 code, we want to say we finished.
      this.dispatchEvent(new CustomEvent('bulk_triage_finished', { bubbles: true }));
      sendEndTask(this);
    }).catch((e) => sendFetchError(this, e, 'bulk triaging'));
  }

  private _toggleAll(e: Event) {
    e.preventDefault();
    this._triageAll = !this._triageAll;
    this._render();
  }
};

define('bulk-triage-sk', BulkTriageSk);
