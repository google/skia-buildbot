import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { CorpusSelectorSkPO } from '../corpus-selector-sk/corpus-selector-sk_po';
import { TraceFilterSkPO } from '../trace-filter-sk/trace-filter-sk_po';
import { FilterDialogSkPO } from '../filter-dialog-sk/filter-dialog-sk_po';
import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk';
import { SearchCriteria } from './search-controls-sk';

/** A page object for the SearchControlsSkPO component. */
export class SearchControlsSkPO extends PageObject {
  getCorpusSelectorPO() {
    return this.selectOnePOEThenApplyFn(
      'corpus-selector-sk', async (el) => new CorpusSelectorSkPO(el));
  }

  getTraceFilterSkPO() {
    return this.selectOnePOEThenApplyFn(
      '.traces > trace-filter-sk', async (el) => new TraceFilterSkPO(el));
  }

  getFilterDialogSkPO() {
    return this.selectOnePOEThenApplyFn(
      'filter-dialog-sk', async (el) => new FilterDialogSkPO(el));
  }

  isIncludePositiveDigestsCheckboxChecked() {
    return this.selectOneDOMNodeThenApplyFn(
      '.include-positive-digests', (c) => (c as CheckOrRadio).checked);
  }

  async clickIncludePositiveDigestsCheckbox() {
    await this.selectOnePOEThenApplyFn('.include-positive-digests', (el) => el.click());
  }

  isIncludeNegativeDigestsCheckboxChecked() {
    return this.selectOneDOMNodeThenApplyFn(
      '.include-negative-digests', (c) => (c as CheckOrRadio).checked);
  }

  async clickIncludeNegativeDigestsCheckbox() {
    await this.selectOnePOEThenApplyFn('.include-negative-digests', (el) => el.click());
  }

  isIncludeUntriagedDigestsCheckboxChecked() {
    return this.selectOneDOMNodeThenApplyFn(
      '.include-untriaged-digests', (c) => (c as CheckOrRadio).checked);
  }

  async clickIncludeUntriagedDigestsCheckbox() {
    await this.selectOnePOEThenApplyFn('.include-untriaged-digests', (el) => el.click());
  }

  isIncludeDigestsNotAtHeadCheckboxChecked() {
    return this.selectOneDOMNodeThenApplyFn(
      '.include-digests-not-at-head', (c) => (c as CheckOrRadio).checked);
  }

  async clickIncludeDigestsNotAtHeadCheckbox() {
    await this.selectOnePOEThenApplyFn('.include-digests-not-at-head', (el) => el.click());
  }

  isIncludeIgnoredDigestsCheckboxChecked() {
    return this.selectOneDOMNodeThenApplyFn(
      '.include-ignored-digests', (c) => (c as CheckOrRadio).checked);
  }

  async clickIncludeIgnoredDigestsCheckbox() {
    await this.selectOnePOEThenApplyFn('.include-ignored-digests', (el) => el.click());
  }

  async clickMoreFiltersBtn() {
    return this.selectOnePOEThenApplyFn('.more-filters', (btn) => btn.click());
  }

  /**
   * Gets the search criteria via simulated UI interactions.
   *
   * Analogous to the searchCriteria property getter.
   */
  async getSearchCriteria() {
    const searchCriteria: Partial<SearchCriteria> = {}
    searchCriteria.corpus = (await (await this.getCorpusSelectorPO()).getSelectedCorpus())!;
    searchCriteria.leftHandTraceFilter = (await (await this.getTraceFilterSkPO()).getSelection());
    searchCriteria.includePositiveDigests = await this.isIncludePositiveDigestsCheckboxChecked();
    searchCriteria.includeNegativeDigests = await this.isIncludeNegativeDigestsCheckboxChecked();
    searchCriteria.includeUntriagedDigests = await this.isIncludeUntriagedDigestsCheckboxChecked();
    searchCriteria.includeDigestsNotAtHead = await this.isIncludeDigestsNotAtHeadCheckboxChecked();
    searchCriteria.includeIgnoredDigests = await this.isIncludeIgnoredDigestsCheckboxChecked();

    // More filters dialog.
    await this.clickMoreFiltersBtn();
    const filterDialogSkPO = await this.getFilterDialogSkPO();
    const filters = await filterDialogSkPO.getSelectedFilters();
    searchCriteria.rightHandTraceFilter = filters.diffConfig;
    searchCriteria.minRGBADelta = filters.minRGBADelta;
    searchCriteria.maxRGBADelta = filters.maxRGBADelta;
    searchCriteria.mustHaveReferenceImage = filters.mustHaveReferenceImage;
    searchCriteria.sortOrder = filters.sortOrder;
    await filterDialogSkPO.clickCancelBtn();

    return searchCriteria as SearchCriteria;
  }

  /**
   * Sets the search criteria via simulated UI interactions.
   *
   * Analogous to the searchCriteria property setter.
   */
  async setSearchCriteria(searchCriteria: SearchCriteria) {
    const corpusSelectorSkPO = await this.getCorpusSelectorPO();
    if (await corpusSelectorSkPO.getSelectedCorpus() !== searchCriteria.corpus) {
      await corpusSelectorSkPO.clickCorpus(searchCriteria.corpus);
    }

    // Left-hand traces.
    const traceFilterSkPO = await this.getTraceFilterSkPO();
    await traceFilterSkPO.clickEditBtn();
    await traceFilterSkPO.setQueryDialogSkSelection(searchCriteria.leftHandTraceFilter);
    await traceFilterSkPO.clickQueryDialogSkShowMatchesBtn();

    // Include positive digests.
    if (await this.isIncludePositiveDigestsCheckboxChecked() !==
        searchCriteria.includePositiveDigests) {
      await this.clickIncludePositiveDigestsCheckbox();
    }

    // Include negative digests.
    if (await this.isIncludeNegativeDigestsCheckboxChecked() !==
        searchCriteria.includeNegativeDigests) {
      await this.clickIncludeNegativeDigestsCheckbox();
    }

    // Include untriaged digests.
    if (await this.isIncludeUntriagedDigestsCheckboxChecked() !==
        searchCriteria.includeUntriagedDigests) {
      await this.clickIncludeUntriagedDigestsCheckbox();
    }

    // Include digests not at head.
    if (await this.isIncludeDigestsNotAtHeadCheckboxChecked() !==
        searchCriteria.includeDigestsNotAtHead) {
      await this.clickIncludeDigestsNotAtHeadCheckbox();
    }

    // Include ignored digests.
    if (await this.isIncludeIgnoredDigestsCheckboxChecked() !==
    searchCriteria.includeIgnoredDigests) {
      await this.clickIncludeIgnoredDigestsCheckbox();
    }

    // More filters dialog.
    await this.clickMoreFiltersBtn();
    const filterDialogSkPO = await this.getFilterDialogSkPO();
    await filterDialogSkPO.setSelectedFilters({
      diffConfig: searchCriteria.rightHandTraceFilter,
      minRGBADelta: searchCriteria.minRGBADelta,
      maxRGBADelta: searchCriteria.maxRGBADelta,
      mustHaveReferenceImage: searchCriteria.mustHaveReferenceImage,
      sortOrder: searchCriteria.sortOrder
    });
    await filterDialogSkPO.clickFilterBtn();
  }
};
