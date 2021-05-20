import {PageObject} from '../../../infra-sk/modules/page_object/page_object';
import {TriageSkPO} from '../triage-sk/triage-sk_po';
import {PageObjectElement, PageObjectElementList} from '../../../infra-sk/modules/page_object/page_object_element';
import {ImageCompareSkPO} from '../image-compare-sk/image-compare-sk_po';
import {DotsLegendSkPO} from '../dots-legend-sk/dots-legend-sk_po';

/** A page object for the DigestDetailsSk component. */
export class DigestDetailsSkPO extends PageObject {
  get triageSkPO(): TriageSkPO {
    return this.poBySelector('triage-sk', TriageSkPO);
  }

  get imageCompareSkPO(): ImageCompareSkPO {
    return this.poBySelector('image-compare-sk', ImageCompareSkPO);
  }

  get dotsLegendSkPO(): DotsLegendSkPO {
    return this.poBySelector('dots-legend-sk', DotsLegendSkPO);
  }

  private get testName(): PageObjectElement {
    return this.bySelector('.top_bar .grouping_name');
  }

  private get clusterLink(): PageObjectElement {
    return this.bySelector('a.cluster_link');
  }

  private get leftDigest(): PageObjectElement {
    return this.bySelector('.digest_label.left');
  }

  private get rightDigest(): PageObjectElement {
    return this.bySelector('.digest_label.right');
  }

  private get diffPageLink(): PageObjectElement {
    return this.bySelector('.metrics_and_triage .diffpage_link');
  }

  private get metrics(): PageObjectElementList {
    return this.bySelectorAll('.metrics_and_triage .metric');
  }

  private get sizeWarning(): PageObjectElement {
    return this.bySelector('.metrics_and_triage .size_warning');
  }

  private get triageHistory(): PageObjectElement {
    return this.bySelector('.metrics_and_triage .triage-history');
  }

  private get toggleReferenceBtn(): PageObjectElement {
    return this.bySelector('button.toggle_ref');
  }

  private get closestImageIsNegativeWarning(): PageObjectElement {
    return this.bySelector('.negative_warning');
  }

  private get zoomDialog(): PageObjectElement {
    return this.bySelector('dialog.zoom_dialog');
  }

  isSelected(): Promise<boolean> {
    return this.element.hasClassName('selected');
  }

  getTestName(): Promise<string> {
    return this.testName.innerText;
  }

  getClusterHref(): Promise<string | null> {
    return this.clusterLink.getAttribute('href');
  }

  getLeftDigest(): Promise<string> {
    return this.leftDigest.innerText;
  }

  async getRightDigest(): Promise<string | null> {
    // Not all DigestDetailsSk instances have a right digest.
    return (await this.rightDigest.isEmpty()) ? null : this.rightDigest.innerText;
  }

  getDiffPageLink(): Promise<string | null> {
    return this.diffPageLink.getAttribute('href');
  }

  getMetrics(): Promise<string[]> {
    return this.metrics.map((metric) => metric.innerText);
  }

  async isSizeWarningVisible(): Promise<boolean> {
    return !(await this.sizeWarning.hasAttribute('hidden'));
  }

  getTriageHistory(): Promise<string> {
    return this.triageHistory.innerText;
  }

  async clickToggleReferenceBtn() {
    await this.toggleReferenceBtn.click();
  }

  async isClosestImageIsNegativeWarningVisible(): Promise<boolean> {
    return !(await this.closestImageIsNegativeWarning.hasAttribute('hidden'));
  }

  isZoomDialogOpen() {
    return this.zoomDialog.hasAttribute('open');
  }
}