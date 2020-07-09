import { PageObject } from '../../../infra-sk/modules/page_object/page_object';

/** A page object for the CorpusSelectorSkPO component. */
export class CorpusSelectorSkPO extends PageObject {
  async isLoadingMessageVisible() {
    return (await this.selectOnePOE('p')) !== null;
  }

  async getCorpora() {
    return this.selectAllPOEThenMap('li', (li) => li.innerText);
  }

  /** Returns the selected corpus, or null if none is selected. */
  async getSelectedCorpus() {
    const selectedCorpora = await this.selectAllPOEThenMap('li.selected', (li) => li.innerText);

    // There can be at most one selected corpora.
    if (selectedCorpora.length > 1) {
      throw new Error('there are more than one selected corpora');
    }

    if (selectedCorpora.length) {
      return selectedCorpora[0];
    }
    return null;
  }

  async clickCorpus(corpus: string) {
    const li = await this.selectAllPOEThenFind('li', async (li) => (await li.innerText) === corpus);
    await li!.click();
  }
}
