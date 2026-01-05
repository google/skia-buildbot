import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import {
  PageObjectElement,
  PageObjectElementList,
} from '../../../infra-sk/modules/page_object/page_object_element';

/** A page object for the GraphTitleSk component. */
export class GraphTitleSkPO extends PageObject {
  private get container(): PageObjectElement {
    return this.bySelector('#container');
  }

  private get showMoreButton(): PageObjectElement {
    return this.bySelector('md-text-button.showMore');
  }

  private get paramNames(): PageObjectElementList {
    return this.bySelectorAll('.param');
  }

  private get paramValues(): PageObjectElementList {
    return this.bySelectorAll('.hover-to-show-text');
  }

  async isHidden(): Promise<boolean> {
    return await this.container.hasAttribute('hidden');
  }

  async getTitleText(): Promise<string> {
    return await this.element.innerText;
  }

  async getParamCount(): Promise<number> {
    return this.paramNames.length;
  }

  async getParamName(index: number): Promise<string> {
    const el = await this.paramNames.item(index);
    return el.innerText;
  }

  async getParamValue(index: number): Promise<string> {
    const el = await this.paramValues.item(index);
    return el.innerText;
  }

  async isShowMoreVisible(): Promise<boolean> {
    return !(await this.showMoreButton.isEmpty());
  }

  async clickShowMore(): Promise<void> {
    await this.showMoreButton.click();
  }
}
