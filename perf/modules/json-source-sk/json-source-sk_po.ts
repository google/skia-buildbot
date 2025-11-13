import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

export class JsonSourceSkPO extends PageObject {
  get viewJsonFileButton(): PageObjectElement {
    return this.bySelector('#view-source');
  }

  get viewShortJsonFileButton(): PageObjectElement {
    return this.bySelector('#load-source');
  }

  get pre(): PageObjectElement {
    return this.bySelector('pre');
  }

  async getJson(): Promise<string> {
    return await this.pre.innerText;
  }

  async clickViewJsonFileButton(): Promise<void> {
    await this.viewJsonFileButton.click();
  }

  async clickViewShortJsonFileButton(): Promise<void> {
    await this.viewShortJsonFileButton.click();
  }

  async getJsonFromDialog(): Promise<string> {
    return await this.bySelector('#json-dialog pre').innerText;
  }

  async isDialogVisible(): Promise<boolean> {
    return await this.bySelector('#json-dialog').hasAttribute('open');
  }
}
