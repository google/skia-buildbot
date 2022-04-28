import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

/** A page object for the PaginationSk component. */
export class PaginationSkPO extends PageObject {
  private get prevBtn(): PageObjectElement {
    return this.bySelector('button.prev');
  }

  private get nextBtn(): PageObjectElement {
    return this.bySelector('button.next');
  }

  private get skipBtn(): PageObjectElement {
    return this.bySelector('button.skip');
  }

  private get counter(): PageObjectElement {
    return this.bySelector('div.counter');
  }

  async clickPrevBtn() { await this.prevBtn.click(); }

  async clickNextBtn() { await this.nextBtn.click(); }

  async clickSkipBtn() { await this.skipBtn.click(); }

  async isPrevBtnDisabled() { return this.prevBtn.hasAttribute('disabled'); }

  async isNextBtnDisabled() { return this.nextBtn.hasAttribute('disabled'); }

  async isSkipBtnDisabled() { return this.skipBtn.hasAttribute('disabled'); }

  async getCurrentPage(): Promise<number> {
    const counterText = await this.counter.innerText;
    const prefix = 'page ';
    if (!counterText.startsWith(prefix)) {
      throw new Error(`expected counter to begin with "${prefix}", but was "${counterText}"`);
    }
    return parseInt(counterText.substring(prefix.length));
  }
}
