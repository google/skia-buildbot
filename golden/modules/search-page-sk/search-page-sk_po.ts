import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';
import { SearchControlsSkPO } from '../search-controls-sk/search-controls-sk_po';
import { ChangelistControlsSkPO } from '../changelist-controls-sk/changelist-controls-sk_po';
import { BulkTriageSkPO } from '../bulk-triage-sk/bulk-triage-sk_po';
import { TriageSkPO } from '../triage-sk/triage-sk_po';
import { Label } from '../rpc_types';
import { asyncFilter, asyncFind, asyncMap } from '../../../infra-sk/modules/async';

/**
 * A page object for the DigestDetailsSk component.
 *
 * TODO(lovisolo): Extract into //golden/modules/digest-details-sk/digest-details-sk_po.ts once
 *                 digest-details-sk is ported to TypeScript.
 */
export class DigestDetailsSkPO extends PageObject {
  get triageSkPO(): Promise<TriageSkPO> {
    return this.poBySelector('triage-sk', TriageSkPO);
  }

  // TODO(lovisolo): Use a less brittle selector (add a "left" CSS class).
  private get leftDigest(): Promise<PageObjectElement> {
    return this.bySelector('.digest_label:nth-child(1)');
  }

  // TODO(lovisolo): Use a less brittle selector (add a "right" CSS class).
  private get rightDigest(): Promise<PageObjectElement> {
    return this.bySelector('.digest_label:nth-child(2)');
  }

  private get diffPageLink(): Promise<PageObjectElement> {
    return this.bySelector('.diffpage_link');
  }

  private get zoomDialog(): Promise<PageObjectElement> {
    return this.bySelector('dialog.zoom_dialog');
  }

  async isSelected() { return this.element.hasClassName('selected'); }

  async getLeftDigest() { return (await this.leftDigest).innerText; }

  async getRightDigest() {
    // Not all DigestDetailsSk instances have a right digest.
    const rightDigest = await this.rightDigest;
    return (await rightDigest.isEmpty()) ? null : rightDigest.innerText;
  }

  async getDiffPageLink() { return (await this.diffPageLink).getAttribute('href'); }

  async isZoomDialogOpen() { return (await this.zoomDialog).hasAttribute('open'); }
}

/** A page object for the SearchPageSk component. */
export class SearchPageSkPO extends PageObject {
  get bulkTriageSkPO(): Promise<BulkTriageSkPO> {
    return this.poBySelector('bulk-triage-sk', BulkTriageSkPO);
  }

  get searchControlsSkPO(): Promise<SearchControlsSkPO> {
    return this.poBySelector('search-controls-sk', SearchControlsSkPO);
  }

  get changelistControlsSkPO(): Promise<ChangelistControlsSkPO> {
    return this.poBySelector('changelist-controls-sk', ChangelistControlsSkPO);
  }

  get digestDetailsSkPOs(): Promise<DigestDetailsSkPO[]> {
    return this.poBySelectorAll('digest-details-sk', DigestDetailsSkPO);
  }

  private get bulkTriageBtn(): Promise<PageObjectElement> {
    return this.bySelector('button.bulk-triage');
  }

  private get bulkTriageDialog(): Promise<PageObjectElement> {
    return this.bySelector('dialog.bulk-triage');
  }

  private get helpBtn(): Promise<PageObjectElement> {
    return this.bySelector('button.help');
  }

  private get helpDialog(): Promise<PageObjectElement> {
    return this.bySelector('dialog.help');
  }

  private get helpDialogCancelBtn(): Promise<PageObjectElement> {
    return this.bySelector('dialog.help button.cancel');
  }

  private get summary(): Promise<PageObjectElement> {
    return this.bySelector('p.summary');
  }

  async clickBulkTriageBtn() { await (await this.bulkTriageBtn).click(); }

  async isBulkTriageDialogOpen() { return (await this.bulkTriageDialog).hasAttribute('open'); }

  async clickHelpBtn() { await (await this.helpBtn).click(); }

  async clickHelpDialogCancelBtn() { await (await this.helpDialogCancelBtn).click(); }

  async isHelpDialogOpen() { return (await this.helpDialog).hasAttribute('open'); }

  async getSummary() { return (await this.summary).innerText; }

  async getSelectedDigest() {
    const selectedDigests =
        await asyncFilter(
            this.digestDetailsSkPOs, (digestDetailsSkPO) => digestDetailsSkPO.isSelected());

    if (selectedDigests.length > 1) {
      throw new Error(
          `found ${selectedDigests.length} selected digests, but at most 1 digest can be selected`);
    }

    return selectedDigests.length === 1 ? selectedDigests[0].getLeftDigest() : null;
  }

  getDigests() {
    return asyncMap(
        this.digestDetailsSkPOs, (digestDetailsSkPO) => digestDetailsSkPO.getLeftDigest());
  }

  getDiffDetailsHrefs() {
    return asyncMap(
        this.digestDetailsSkPOs, (digestDetailsSkPO) => digestDetailsSkPO.getDiffPageLink());
  }

  async getLabelForDigest(digest: string): Promise<Label | null> {
    const digestDetailsSkPO =
        await asyncFind(this.digestDetailsSkPOs, async (digestDetailsSkPO) => {
      const leftDigest = await digestDetailsSkPO.getLeftDigest();
      const rightDigest = await digestDetailsSkPO.getRightDigest();
      return leftDigest === digest || rightDigest === digest;
    })
    if (!digestDetailsSkPO) return null;

    return await (await digestDetailsSkPO.triageSkPO).getLabelOrEmpty() as Label;
  }

  async getDigestWithOpenZoomDialog() {
    const digestDetailsSkPOsWithOpenDialogs =
        await asyncFilter(
            this.digestDetailsSkPOs, (digestDetailsSkPO) => digestDetailsSkPO.isZoomDialogOpen());

    if (digestDetailsSkPOsWithOpenDialogs.length > 1) {
      throw new Error(
        'at most 1 digest can have its zoom dialog open, ' +
        `but found ${digestDetailsSkPOsWithOpenDialogs.length} such digests`);
    }

    return digestDetailsSkPOsWithOpenDialogs.length === 1
        ? digestDetailsSkPOsWithOpenDialogs[0].getLeftDigest() : null;
  }

  typeKey(key: string) {
    return this.element.typeKey(key);
  }
}
