import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { PageObjectElement, PageObjectElementList } from '../../../infra-sk/modules/page_object/page_object_element';
import { MultiZoomSkPO } from '../multi-zoom-sk/multi-zoom-sk_po';

/** A page object for the ImageCompareSk component. */
export class ImageCompareSkPO extends PageObject {
  get multiZoomSkPO(): MultiZoomSkPO {
    return this.poBySelector('multi-zoom-sk', MultiZoomSkPO);
  }

  private get zoomBtn(): PageObjectElement {
    return this.bySelector('button.zoom_btn');
  }

  private get closeZoomDialogBtn(): PageObjectElement {
    return this.bySelector('dialog.zoom_dialog button.close_btn');
  }

  private get images(): PageObjectElementList {
    return this.bySelectorAll('img');
  }

  private get imageAnchors(): PageObjectElementList {
    return this.bySelectorAll('figcaption a');
  }

  private get zoomDialog(): PageObjectElement {
    return this.bySelector('dialog.zoom_dialog');
  }

  getImageSrcs(): Promise<(string | null)[]> {
    return this.images.map((img) => img.getAttribute('src'));
  }

  getImageCaptionTexts(): Promise<string[]> {
    return this.imageAnchors.map((a) => a.innerText);
  }

  getImageCaptionHrefs(): Promise<(string | null)[]> {
    return this.imageAnchors.map((a) => a.getAttribute('href'));
  }

  async isZoomBtnVisible(): Promise<boolean> {
    return !(await this.zoomBtn.hasAttribute('hidden'));
  }

  async clickZoomBtn(): Promise<void> {
    await this.zoomBtn.click();
  }

  isZoomDialogVisible(): Promise<boolean> {
    return this.zoomDialog.hasAttribute('open');
  }

  async clickCloseZoomDialogBtn(): Promise<void> {
    await this.closeZoomDialogBtn.click();
  }

  async clickImage(index: number): Promise<void> {
    const image = await this.images.item(index);
    await image.click();
  }
}
