import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import {
  PageObjectElement,
  PageObjectElementList,
} from '../../../infra-sk/modules/page_object/page_object_element';

export class BugTooltipSkPO extends PageObject {
  get bugCountContainer(): PageObjectElement {
    return this.bySelector('.bug-count-container');
  }

  get bugTooltip(): PageObjectElement {
    return this.bySelector('.bug-tooltip');
  }

  get bugLinks(): PageObjectElementList {
    return this.bugTooltip.bySelectorAll('ul li a');
  }

  async hoverOverBugCountContainer(): Promise<void> {
    await this.bugCountContainer.hover();
  }

  async isTooltipVisible(): Promise<boolean> {
    return this.bugTooltip.applyFnToDOMNode(
      (el) => window.getComputedStyle(el).visibility === 'visible'
    );
  }

  async isBugContainerVisible(): Promise<boolean> {
    return !(await this.bugCountContainer.hasAttribute('hidden'));
  }

  async getBugLinks(): Promise<(string | null)[]> {
    return this.bugLinks.map((link) => link.getAttribute('href'));
  }

  async getContent(): Promise<string> {
    return this.bugTooltip.innerText;
  }

  async isScrollable(): Promise<boolean> {
    return this.bugTooltip.applyFnToDOMNode((el) => el.scrollHeight > el.clientHeight);
  }
}
