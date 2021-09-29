import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { PageObjectElement, PageObjectElementList } from '../../../infra-sk/modules/page_object/page_object_element';

/** A page object for the CorpusSelectorSkPO component. */
export class CorpusSelectorSkPO extends PageObject {
  private get loadingMessage(): PageObjectElement {
    return this.bySelector('p');
  }

  private get selectedCorpus(): PageObjectElement {
    return this.bySelector('li.selected');
  }

  private get corpora(): PageObjectElementList {
    return this.bySelectorAll('li');
  }

  async isLoadingMessageVisible() { return !(await this.loadingMessage.isEmpty()); }

  async getCorpora() { return this.corpora.map((li) => li.innerText); }

  /** Returns the selected corpus, or null if none is selected. */
  async getSelectedCorpus() {
    return (await this.selectedCorpus.isEmpty()) ? null : this.selectedCorpus.innerText;
  }

  async clickCorpus(corpus: string) {
    const corpusLi = await this.corpora.find((li) => li.isInnerTextEqualTo(corpus));
    await corpusLi!.click();
  }
}
