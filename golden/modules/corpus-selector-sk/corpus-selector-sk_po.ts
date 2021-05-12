import { BySelector, BySelectorAll, PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';
import { asyncFind, asyncMap } from '../../../infra-sk/modules/async';

/** A page object for the CorpusSelectorSkPO component. */
export class CorpusSelectorSkPO extends PageObject {
  @BySelector('p')
  private loadingMessage!: Promise<PageObjectElement>;

  @BySelector('li.selected')
  private selectedCorpus!: Promise<PageObjectElement>;

  @BySelectorAll('li')
  private corpora!: Promise<PageObjectElement[]>;

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
