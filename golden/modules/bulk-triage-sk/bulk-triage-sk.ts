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
import {
  BulkTriageDeltaInfo, Label, TriageDelta, TriageRequestV3,
} from '../rpc_types';

/**
 * The label to apply to the selected digests via the bulk triage dialog, or 'closest' to apply the
 * label of the closest triaged reference digest.
 */
export type BulkTriageLabel = Label | 'closest';

export class BulkTriageSk extends ElementSk {
  private static template = (el: BulkTriageSk) => html`
    <h2>Bulk Triage</h2>
    <p>Assign the status to all images on this page at once.</p>
    ${el._changeListID ? html`<p class=cl>This affects Changelist ${el._changeListID}.</p>` : ''}
    <div class=status>
      <button class="positive ${el.label === 'positive' ? 'selected' : ''}"
              @click=${() => el.onTriageLabelBtnClick('positive')}
              title="Triage all the left-hand images as positive."  >
        <check-circle-icon-sk></check-circle-icon-sk>
      </button>
      <button class="negative ${el.label === 'negative' ? 'selected' : ''}"
              @click=${() => el.onTriageLabelBtnClick('negative')}
              title="Triage all the left-hand images as negative.">
        <cancel-icon-sk></cancel-icon-sk>
      </button>
      <button class="untriaged ${el.label === 'untriaged' ? 'selected' : ''}"
              @click=${() => el.onTriageLabelBtnClick('untriaged')}
              title="Unset the triage status of all left-hand images.">
        <help-icon-sk></help-icon-sk>
      </button>
      <button class="closest ${el.label === 'closest' ? 'selected' : ''}"
              @click=${() => el.onTriageLabelBtnClick('closest')}
              title="Triage all the left-hand images the same as the closest image.">
        <view-agenda-icon-sk></view-agenda-icon-sk>
      </button>
    </div>

    <div>
      <checkbox-sk @change=${el.onToggleAllCheckboxClick}
                   label="Triage all ${el.numTotalDigests()} digests"
                   title="${
                     'Choose whether to triage just the digests on this page '
                     + 'or all that match the query.'
                    }"
                   ?checked=${el.triageAll}
                   class=triage_all>
      </checkbox-sk>
    </div>

    <div class=controls>
      <button @click=${el.onCancelBtnClick} class=cancel>
        Cancel (do nothing)
      </button>
      <button @click=${el.onTriageBtnClick} class="action triage">
        Triage ${el.triageAll ? el.numTotalDigests() : el.numDigestsInPage()} digests as
        ${el.label}
      </button>
    </div>
  `;

  private _changeListID = '';

  private _crs = '';

  private _bulkTriageDeltaInfos: BulkTriageDeltaInfo[] = [];

  private label: BulkTriageLabel = 'closest';

  private triageAll = false;

  constructor() {
    super(BulkTriageSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  /**
   * The ID of the changelist to which these expectations should belong, or the empty string if
   * none.
   */
  set changeListID(changeListID: string) {
    this._changeListID = changeListID;
    this._render();
  }

  /**
   * The Code Review System (e.g. "gerrit") associated with the provided changelist ID, or the
   * empty string if none.
   */
  set crs(crs: string) {
    this._crs = crs;
    this._render();
  }

  set bulkTriageDeltaInfos(bulkTriageDeltaInfos: BulkTriageDeltaInfo[]) {
    this._bulkTriageDeltaInfos = bulkTriageDeltaInfos;
    this._render();
  }

  private numDigestsInPage() {
    return this._bulkTriageDeltaInfos
      .filter((triageDeltaInfo) => triageDeltaInfo.in_current_search_results_page)
      .length;
  }

  private numTotalDigests() {
    return this._bulkTriageDeltaInfos.length;
  }

  private onTriageLabelBtnClick(label: BulkTriageLabel) {
    this.label = label;
    this._render();
  }

  private onToggleAllCheckboxClick(e: Event) {
    e.preventDefault();
    this.triageAll = !this.triageAll;
    this._render();
  }

  private onCancelBtnClick() {
    this.dispatchEvent(new CustomEvent('bulk_triage_cancelled', { bubbles: true }));
  }

  private onTriageBtnClick() {
    sendBeginTask(this);
    this.dispatchEvent(new CustomEvent('bulk_triage_invoked', { bubbles: true }));
    const url = '/json/v3/triage';
    fetch(url, {
      method: 'POST',
      body: JSON.stringify(this.makeTriageRequest()),
    }).then(() => {
      // Even if we get back a non-200 code, we want to say we finished.
      this.dispatchEvent(new CustomEvent('bulk_triage_finished', { bubbles: true }));
      sendEndTask(this);
    }).catch((e) => sendFetchError(this, e, 'bulk triaging'));
  }

  private makeTriageRequest(): TriageRequestV3 {
    const triageDeltas: TriageDelta[] = this._bulkTriageDeltaInfos
      .filter((triageDeltaInfo) => {
        if (this.label === 'closest' && triageDeltaInfo.closest_diff_label === 'none') {
          return false;
        }
        return this.triageAll || triageDeltaInfo.in_current_search_results_page;
      })
      .map((triageDeltaInfo) => ({
        grouping: triageDeltaInfo.grouping,
        digest: triageDeltaInfo.digest,
        label_before: triageDeltaInfo.label_before,
        label_after: this.label === 'closest'
          ? triageDeltaInfo.closest_diff_label as Label // We've already checked this isn't "none".
          : this.label as Label,
      }));

    if (this._changeListID && this._crs) {
      return {
        deltas: triageDeltas,
        changelist_id: this._changeListID,
        crs: this._crs,
      };
    }
    return { deltas: triageDeltas };
  }
}

define('bulk-triage-sk', BulkTriageSk);
