import { BySelector, PageObject, POBySelector, POBySelectorAll } from '../../../infra-sk/modules/page_object/page_object';
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
  @POBySelector('triage-sk', TriageSkPO)
  triageSkPO!: Promise<TriageSkPO>;

  // TODO(lovisolo): Use a less brittle selector (add a "left" CSS class).
  @BySelector('.digest_label:nth-child(1)')
  private leftDigest!: Promise<PageObjectElement>;

  // TODO(lovisolo): Use a less brittle selector (add a "right" CSS class).
  @BySelector('.digest_label:nth-child(2)')
  private rightDigest!: Promise<PageObjectElement>;

  @BySelector('.diffpage_link')
  private diffPageLink!: Promise<PageObjectElement>;

  @BySelector('dialog.zoom_dialog')
  private zoomDialog!: Promise<PageObjectElement>;

  async isSelected() { return this.element.hasClassName('selected'); }

  async getLeftDigest() { return (await this.leftDigest).innerText; }

  async getRightDigest() {
    // Not all DigestDetailsSk instances have a right digest.
    const rightDigest = await this.rightDigest;
    return rightDigest.empty ? null : rightDigest.innerText;
  }

  async getDiffPageLink() { return (await this.diffPageLink).getAttribute('href'); }

  async isZoomDialogOpen() { return (await this.zoomDialog).hasAttribute('open'); }
}

/** A page object for the SearchPageSk component. */
export class SearchPageSkPO extends PageObject {
  @POBySelector('bulk-triage-sk', BulkTriageSkPO)
  bulkTriageSkPO!: Promise<BulkTriageSkPO>;

  @POBySelector('search-controls-sk', SearchControlsSkPO)
  searchControlsSkPO!: Promise<SearchControlsSkPO>;

  @POBySelector('changelist-controls-sk', ChangelistControlsSkPO)
  changelistControlsSkPO!: Promise<ChangelistControlsSkPO>;

  @POBySelectorAll('digest-details-sk', DigestDetailsSkPO)
  digestDetailsSkPOs!: Promise<DigestDetailsSkPO[]>;

  @BySelector('button.bulk-triage')
  bulkTriageBtn!: Promise<PageObjectElement>;

  @BySelector('dialog.bulk-triage')
  bulkTriageDialog!: Promise<PageObjectElement>;

  @BySelector('button.help')
  helpBtn!: Promise<PageObjectElement>;

  @BySelector('dialog.help')
  helpDialog!: Promise<PageObjectElement>;

  @BySelector('dialog.help button.cancel')
  helpDialogCancelBtn!: Promise<PageObjectElement>;

  @BySelector('p.summary')
  summary!: Promise<PageObjectElement>;

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
