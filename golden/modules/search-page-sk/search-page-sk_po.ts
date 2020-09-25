import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { SearchControlsSkPO } from '../search-controls-sk/search-controls-sk_po';
import { ChangelistControlsSkPO } from '../changelist-controls-sk/changelist-controls-sk_po';
import { BulkTriageSkPO } from '../bulk-triage-sk/bulk-triage-sk_po';
import { Label } from '../rpc_types';

/** A page object for the SearchPageSkPO component. */
export class SearchPageSkPO extends PageObject {
  getBulkTriageSkPO() {
    return this.selectOnePOEThenApplyFn('bulk-triage-sk', async (el) => new BulkTriageSkPO(el));
  }

  getSearchControlsSkPO() {
    return this.selectOnePOEThenApplyFn(
      'search-controls-sk', async (el) => new SearchControlsSkPO(el));
  }

  getChangelistControlsSkPO() {
    return this.selectOnePOEThenApplyFn(
      'changelist-controls-sk', async (el) => new ChangelistControlsSkPO(el));
  }

  async clickBulkTriageBtn() {
    await this.selectOnePOEThenApplyFn('button.bulk-triage', (el) => el.click());
  }

  isBulkTriageDialogOpen() {
    return this.selectOnePOEThenApplyFn('dialog.bulk-triage', (el) => el.hasAttribute('open'))
  }

  async clickHelpBtn() {
    await this.selectOnePOEThenApplyFn('button.help', (el) => el.click());
  }

  async clickHelpDialogCancelBtn() {
    await this.selectOnePOEThenApplyFn('dialog.help button.cancel', (el) => el.click());
  }

  isHelpDialogOpen() {
    return this.selectOnePOEThenApplyFn('dialog.help', (el) => el.hasAttribute('open'))
  }

  getSummary() {
    return this.selectOnePOEThenApplyFn('p.summary', (el) => el.innerText);
  }

  // TODO(lovisolo): Replace with DigestDetailsSkPO when DigestDetailsSk is ported to TypeScript
  //                 and tested with a page object.
  getSelectedDigest() {
    // By returning an array instead of a single digest (or null) we can assert in tests that there
    // is at most one selected digest at any given time.
    return this.selectAllPOEThenMap(
      'digest-details-sk.selected .digest_label:nth-child(1)', (el) => el.innerText);
  }


  // TODO(lovisolo): Replace with DigestDetailsSkPO when DigestDetailsSk is ported to TypeScript
  //                 and tested with a page object.
  getDigests() {
    return this.selectAllPOEThenMap('.digest_label:nth-child(1)', (el) => el.innerText);
  }

  // TODO(lovisolo): Replace with DigestDetailsSkPO when DigestDetailsSk is ported to TypeScript
  //                 and tested with a page object.
  getDiffDetailsHrefs() {
    return this.selectAllPOEThenMap('.diffpage_link', (el) => el.href);
  }

  // TODO(lovisolo): Replace with DigestDetailsSkPO when DigestDetailsSk is ported to TypeScript
  //                 and tested with a page object.
  async getDigestLabels() {
    return this.selectAllPOEThenMap<Label>(
        'digest-details-sk triage-sk button.selected', async (el) => {
      // The className attribute can be e.g. "positive selected", so we look for label substrings.
      const className = await el.className;
      if (className.includes('positive')) return 'positive';
      if (className.includes('negative')) return 'negative';
      return 'untriaged';
    });
  }

  // TODO(lovisolo): Replace with DigestDetailsSkPO when DigestDetailsSk is ported to TypeScript
  //                 and tested with a page object.
  async getDigestWithOpenZoomDialog() {
    // By returning an array instead of a single digest (or null) we can assert in tests that there
    // is at most one digest at any given time with its zoom dialog open.

    const digests = await this.getDigests();
    if (digests.length === 0) return [];

    const openZoomDialogs =
      await this.selectAllPOEThenMap(
        'digest-details-sk dialog.zoom_dialog', (el) => el.hasAttribute('open'));
    return digests.filter((_, idx) => openZoomDialogs[idx]);
  }

  typeKey(key: string) {
    return this.element.typeKey(key);
  }
};
