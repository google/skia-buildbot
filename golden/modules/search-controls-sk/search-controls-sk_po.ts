import { PageObject } from '../../../infra-sk/modules/page_object/page_object';
import { CorpusSelectorSkPO } from '../corpus-selector-sk/corpus-selector-sk_po';
import { TraceFilterSkPO } from '../trace-filter-sk/trace-filter-sk_po';
import { FilterDialogSkPO } from '../filter-dialog-sk/filter-dialog-sk_po';
import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk';
import { SearchCriteria } from './search-controls-sk';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

/** A page object for the SearchControlsSkPO component. */
export class SearchControlsSkPO extends PageObject {
  get corpusSelectorSkPO(): CorpusSelectorSkPO {
    return this.poBySelector('corpus-selector-sk', CorpusSelectorSkPO);
  }

  get traceFilterSkPO(): TraceFilterSkPO {
    return this.poBySelector('.traces > trace-filter-sk', TraceFilterSkPO);
  }

  get filterDialogSkPO(): FilterDialogSkPO {
    return this.poBySelector('filter-dialog-sk', FilterDialogSkPO);
  }

  private get includePositiveDigestsCheckBox(): PageObjectElement {
    return this.bySelector('.include-positive-digests');
  }

  private get includeNegativeDigestsCheckBox(): PageObjectElement {
    return this.bySelector('.include-negative-digests');
  }

  private get includeUntriagedDigestsCheckBox(): PageObjectElement {
    return this.bySelector('.include-untriaged-digests');
  }

  private get includeDigestsNotAtHeadCheckBox(): PageObjectElement {
    return this.bySelector('.include-digests-not-at-head');
  }

  private get includeIgnoredDigestsCheckBox(): PageObjectElement {
    return this.bySelector('.include-ignored-digests');
  }

  private get moreFiltersBtn(): PageObjectElement {
    return this.bySelector('.more-filters');
  }

  async isIncludePositiveDigestsCheckboxChecked() {
    return this.includePositiveDigestsCheckBox.applyFnToDOMNode(
        (c) => (c as CheckOrRadio).checked);
  }

  async clickIncludePositiveDigestsCheckbox() {
    await this.includePositiveDigestsCheckBox.click();
  }

  async isIncludeNegativeDigestsCheckboxChecked() {
    return this.includeNegativeDigestsCheckBox
        .applyFnToDOMNode((c) => (c as CheckOrRadio).checked);
  }

  async clickIncludeNegativeDigestsCheckbox() {
    await this.includeNegativeDigestsCheckBox.click();
  }

  async isIncludeUntriagedDigestsCheckboxChecked() {
    return this.includeUntriagedDigestsCheckBox.applyFnToDOMNode(
        (c) => (c as CheckOrRadio).checked);
  }

  async clickIncludeUntriagedDigestsCheckbox() {
    await this.includeUntriagedDigestsCheckBox.click();
  }

  async isIncludeDigestsNotAtHeadCheckboxChecked() {
    return this.includeDigestsNotAtHeadCheckBox.applyFnToDOMNode(
        (c) => (c as CheckOrRadio).checked);
  }

  async clickIncludeDigestsNotAtHeadCheckbox() {
    await this.includeDigestsNotAtHeadCheckBox.click();
  }

  async isIncludeIgnoredDigestsCheckboxChecked() {
    return this.includeIgnoredDigestsCheckBox.applyFnToDOMNode(
        (c) => (c as CheckOrRadio).checked);
  }

  async clickIncludeIgnoredDigestsCheckbox() {
    await this.includeIgnoredDigestsCheckBox.click();
  }

  async clickMoreFiltersBtn() { await this.moreFiltersBtn.click(); }

  /**
   * Gets the search criteria via simulated UI interactions.
   *
   * Analogous to the searchCriteria property getter.
   */
  async getSearchCriteria() {
    const searchCriteria: Partial<SearchCriteria> = {}
    searchCriteria.corpus = (await this.corpusSelectorSkPO.getSelectedCorpus())!;
    searchCriteria.leftHandTraceFilter = await this.traceFilterSkPO.getSelection();
    searchCriteria.includePositiveDigests = await this.isIncludePositiveDigestsCheckboxChecked();
    searchCriteria.includeNegativeDigests = await this.isIncludeNegativeDigestsCheckboxChecked();
    searchCriteria.includeUntriagedDigests = await this.isIncludeUntriagedDigestsCheckboxChecked();
    searchCriteria.includeDigestsNotAtHead = await this.isIncludeDigestsNotAtHeadCheckboxChecked();
    searchCriteria.includeIgnoredDigests = await this.isIncludeIgnoredDigestsCheckboxChecked();

    // More filters dialog.
    await this.clickMoreFiltersBtn();
    const filters = await this.filterDialogSkPO.getSelectedFilters();
    searchCriteria.rightHandTraceFilter = filters.diffConfig;
    searchCriteria.minRGBADelta = filters.minRGBADelta;
    searchCriteria.maxRGBADelta = filters.maxRGBADelta;
    searchCriteria.mustHaveReferenceImage = filters.mustHaveReferenceImage;
    searchCriteria.sortOrder = filters.sortOrder;
    await this.filterDialogSkPO.clickCancelBtn();

    return searchCriteria as SearchCriteria;
  }

  /**
   * Sets the search criteria via simulated UI interactions.
   *
   * Analogous to the searchCriteria property setter.
   */
  async setSearchCriteria(searchCriteria: SearchCriteria) {
    if (await this.corpusSelectorSkPO.getSelectedCorpus() !== searchCriteria.corpus) {
      await this.corpusSelectorSkPO.clickCorpus(searchCriteria.corpus);
    }

    // Left-hand traces.
    await this.traceFilterSkPO.clickEditBtn();
    await this.traceFilterSkPO.setQueryDialogSkSelection(searchCriteria.leftHandTraceFilter);
    await this.traceFilterSkPO.clickQueryDialogSkShowMatchesBtn();

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
    await this.filterDialogSkPO.setSelectedFilters({
      diffConfig: searchCriteria.rightHandTraceFilter,
      minRGBADelta: searchCriteria.minRGBADelta,
      maxRGBADelta: searchCriteria.maxRGBADelta,
      mustHaveReferenceImage: searchCriteria.mustHaveReferenceImage,
      sortOrder: searchCriteria.sortOrder
    });
    await this.filterDialogSkPO.clickFilterBtn();
  }
}
