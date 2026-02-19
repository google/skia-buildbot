import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

export class RegressionPageSkPO extends PageObject {
  get sheriffSelect(): PageObjectElement {
    return this.bySelector('select[id^="filter-"]');
  }

  async selectSheriff(sheriff: string) {
    await this.sheriffSelect.enterValue(sheriff);
  }
}
