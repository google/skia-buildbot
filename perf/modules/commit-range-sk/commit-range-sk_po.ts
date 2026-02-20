import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

export class CommitRangeSkPO extends PageObject {
  get link(): PageObjectElement {
    return this.bySelector('a');
  }

  get text(): PageObjectElement {
    return this.bySelector('a, span');
  }

  async getHref(): Promise<string> {
    return (await this.link.getAttribute('href')) ?? '';
  }

  async getText(): Promise<string> {
    return (await this.text.innerText) ?? '';
  }
}
