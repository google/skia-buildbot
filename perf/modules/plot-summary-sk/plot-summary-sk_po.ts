import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

/**
 * Page Object for the PlotSummarySk component.
 */
export class PlotSummarySkPO extends PageObject {
  get leftLoadButton(): PageObjectElement {
    return this.bySelectorShadow('.load-btn:first-of-type');
  }

  get rightLoadButton(): PageObjectElement {
    return this.bySelectorShadow('.load-btn:last-of-type');
  }

  get googleChart(): PageObjectElement {
    return this.bySelectorShadow('google-chart');
  }

  private _boundingBox: any;

  get boundingBox(): any {
    return this._boundingBox;
  }

  set boundingBox(boundingBox: any) {
    this._boundingBox = boundingBox;
  }

  async clickLeftLoad(): Promise<void> {
    await this.leftLoadButton.click();
  }

  async clickRightLoad(): Promise<void> {
    await this.rightLoadButton.click();
  }
}
