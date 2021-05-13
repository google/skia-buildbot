import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';
import { asyncFind, asyncMap } from '../../../infra-sk/modules/async';

/** A page object for the CorpusSelectorSkPO component. */
export class CorpusSelectorSkPO extends PageObject {
  private get loadingMessage(): Promise<PageObjectElement> {
    return this.bySelector('p');
  }

  private get selectedCorpus(): Promise<PageObjectElement> {
    return this.bySelector('li.selected');
  }

  private get corpora(): Promise<PageObjectElement[]> {
    return this.bySelectorAll('li');
  }

  async isLoadingMessageVisible() { return !(await this.loadingMessage).isEmpty(); }

  async getCorpora() { return asyncMap(this.corpora, (li) => li.innerText); }

  /** Returns the selected corpus, or null if none is selected. */
  async getSelectedCorpus() {
    const selectedCorpus = await this.selectedCorpus;
    return selectedCorpus.isEmpty() ? null : selectedCorpus.innerText;
  };

  async clickCorpus(corpus: string) {
    const corpusLi = await asyncFind(this.corpora, (li) => li.isInnerTextEqualTo(corpus));
    await corpusLi!.click();
  }
}
