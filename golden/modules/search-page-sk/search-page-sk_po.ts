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
  async getSelectedDigest() {
    const selectedDigest =
      await this.selectAllPOEThenMap(
        'digest-details-sk.selected .digest_label:nth-child(1)', (el) => el.innerText);

    if (selectedDigest.length > 1) {
      throw new Error(
        `found ${selectedDigest.length} selected digests, but at most 1 digest can be selected`);
    }

    return selectedDigest.length === 1 ? selectedDigest[0] : null;
  }


  // TODO(lovisolo): Replace with DigestDetailsSkPO when DigestDetailsSk is ported to TypeScript
  //                 and tested with a page object.
  getDigests() {
    return this.selectAllPOEThenMap('.digest_label:nth-child(1)', (el) => el.innerText);
  }

  // TODO(lovisolo): Replace with DigestDetailsSkPO when DigestDetailsSk is ported to TypeScript
  //                 and tested with a page object.
  getDiffDetailsHrefs() {
    return this.selectAllPOEThenMap('.diffpage_link', (el) => el.getAttribute('href'));
  }

  // TODO(lovisolo): Replace with DigestDetailsSkPO when DigestDetailsSk is ported to TypeScript
  //                 and tested with a page object.
  async getLabelForDigest(digest: string): Promise<Label | null> {
    const digestDetailsSk = await this.selectAllPOEThenFind('digest-details-sk', async (el) => {
      const currentDigest = await el.selectOnePOEThenApplyFn('.digest_label', (el) => el.innerText);
      return currentDigest.includes(digest);
    });
    if (!digestDetailsSk) return null;

    const selectedTriageBtnClassName =
      await digestDetailsSk.selectOnePOEThenApplyFn(
        'triage-sk button.selected', (el) => el.className);

    if (selectedTriageBtnClassName.includes('positive')) return 'positive';
    if (selectedTriageBtnClassName.includes('negative')) return 'negative';
    return 'untriaged';
  }

  // TODO(lovisolo): Replace with DigestDetailsSkPO when DigestDetailsSk is ported to TypeScript
  //                 and tested with a page object.
  async getDigestWithOpenZoomDialog() {
    const digests = await this.getDigests();
    if (digests.length === 0) return null;

    const openZoomDialogs =
      await this.selectAllPOEThenMap(
        'digest-details-sk dialog.zoom_dialog', (el) => el.hasAttribute('open'));
    const digestsWithOpenZoomDialogs = digests.filter((_, idx) => openZoomDialogs[idx]);

    if (digestsWithOpenZoomDialogs.length > 1) {
      throw new Error(
        'at most 1 digest can have its zoom dialog open, ' +
        `but found ${digestsWithOpenZoomDialogs.length} such digests`);
    }

    return digestsWithOpenZoomDialogs.length === 1 ? digestsWithOpenZoomDialogs[0] : null;
  }

  typeKey(key: string) {
    return this.element.typeKey(key);
  }

  // TODO(lovisolo): Remove after the legacy search page has been deleted.
  getLegacySearchPageHref() {
    return this.selectOnePOEThenApplyFn('a.legacy-search-page', (el) => el.getAttribute('href'));
  }
};
