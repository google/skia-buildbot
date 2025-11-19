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

  private get multiTraceTitle(): PageObjectElement {
    return this.bySelector('h1');
  }

  private get columns(): PageObjectElementList {
    return this.bySelectorAll('.column');
  }

  private get showMoreButton(): PageObjectElement {
    return this.bySelector('.showMore');
  }

  async isContainerHidden(): Promise<boolean> {
    return this.container.hasAttribute('hidden');
  }

  async getMultiTraceTitle(): Promise<string> {
    return this.multiTraceTitle.innerText;
  }

  async getParamAndValuePairs(): Promise<{ param: string; value: string }[]> {
    const pairs: { param: string; value: string }[] = [];
    const len = await this.columns.length;
    for (let i = 0; i < len; i++) {
      const col = await this.columns.item(i);
      const param = await col.bySelector('.param').innerText;
      const value = await col.bySelector('.hover-to-show-text').innerText;
      pairs.push({ param, value });
    }
    return pairs;
  }

  async isShowMoreButtonVisible(): Promise<boolean> {
    return !(await this.showMoreButton.isEmpty());
  }

  async clickShowMoreButton(): Promise<void> {
    await this.showMoreButton.click();
  }
}
