import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

export class CommitRangeSkPO extends PageObject {
  get link(): PageObjectElement {
    return this.bySelector('a');
  }

  async getHref(): Promise<string> {
    return (await this.link.getAttribute('href')) ?? '';
  }
}
