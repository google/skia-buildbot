import {BySelector, PageObject, POBySelector} from '../../../infra-sk/modules/page_object/page_object';
import { CorpusSelectorSkPO } from '../corpus-selector-sk/corpus-selector-sk_po';
import { TraceFilterSkPO } from '../trace-filter-sk/trace-filter-sk_po';
import { FilterDialogSkPO } from '../filter-dialog-sk/filter-dialog-sk_po';
import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk';
import { SearchCriteria } from './search-controls-sk';
import {PageObjectElement} from '../../../infra-sk/modules/page_object/page_object_element';

/** A page object for the SearchControlsSkPO component. */
export class SearchControlsSkPO extends PageObject {
  @POBySelector('corpus-selector-sk', CorpusSelectorSkPO)
  corpusSelectorSkPO?: Promise<CorpusSelectorSkPO>;

  @POBySelector('.traces > trace-filter-sk', TraceFilterSkPO)
  traceFilterSkPO?: Promise<TraceFilterSkPO>;

  @POBySelector('filter-dialog-sk', FilterDialogSkPO)
  filterDialogSkPO?: Promise<FilterDialogSkPO>

  @BySelector('.include-positive-digests')
  private includePositiveDigestsCheckBox?: Promise<PageObjectElement>

  @BySelector('.include-negative-digests')
  private includeNegativeDigestsCheckBox?: Promise<PageObjectElement>

  @BySelector('.include-untriaged-digests')
  private includeUntriagedDigestsCheckBox?: Promise<PageObjectElement>

  @BySelector('.include-digests-not-at-head')
  private includeDigestsNotAtHeadCheckBox?: Promise<PageObjectElement>

  @BySelector('.include-ignored-digests')
  private includeIgnoredDigestsCheckBox?: Promise<PageObjectElement>

  @BySelector('.more-filters')
  private moreFiltersBtn?: Promise<PageObjectElement>

  async isIncludePositiveDigestsCheckboxChecked() {
    return (await this.includePositiveDigestsCheckBox)
        ?.applyFnToDOMNode((c) => (c as CheckOrRadio).checked);
  }

  async clickIncludePositiveDigestsCheckbox() {
    await (await this.includePositiveDigestsCheckBox)?.click();
  }

  async isIncludeNegativeDigestsCheckboxChecked() {
    return (await this.includeNegativeDigestsCheckBox)
        ?.applyFnToDOMNode((c) => (c as CheckOrRadio).checked);
  }

  async clickIncludeNegativeDigestsCheckbox() {
    await (await this.includeNegativeDigestsCheckBox)?.click();
  }

  async isIncludeUntriagedDigestsCheckboxChecked() {
    return (await this.includeUntriagedDigestsCheckBox)
        ?.applyFnToDOMNode((c) => (c as CheckOrRadio).checked);
  }

  async clickIncludeUntriagedDigestsCheckbox() {
    await (await this.includeUntriagedDigestsCheckBox)?.click();
  }

  async isIncludeDigestsNotAtHeadCheckboxChecked() {
    return (await this.includeDigestsNotAtHeadCheckBox)
        ?.applyFnToDOMNode((c) => (c as CheckOrRadio).checked);
  }

  async clickIncludeDigestsNotAtHeadCheckbox() {
    await (await this.includeDigestsNotAtHeadCheckBox)?.click();
  }

  async isIncludeIgnoredDigestsCheckboxChecked() {
    return (await this.includeIgnoredDigestsCheckBox)
        ?.applyFnToDOMNode((c) => (c as CheckOrRadio).checked);
  }

  async clickIncludeIgnoredDigestsCheckbox() {
    await (await this.includeIgnoredDigestsCheckBox)?.click();
  }

  async clickMoreFiltersBtn() { await (await this.moreFiltersBtn)?.click(); }

  /**
   * Gets the search criteria via simulated UI interactions.
   *
   * Analogous to the searchCriteria property getter.
   */
  async getSearchCriteria() {
    const searchCriteria: Partial<SearchCriteria> = {}
    searchCriteria.corpus = (await (await this.corpusSelectorSkPO)?.getSelectedCorpus())!;
    searchCriteria.leftHandTraceFilter = (await (await this.traceFilterSkPO)?.getSelection());
    searchCriteria.includePositiveDigests = await this.isIncludePositiveDigestsCheckboxChecked();
    searchCriteria.includeNegativeDigests = await this.isIncludeNegativeDigestsCheckboxChecked();
    searchCriteria.includeUntriagedDigests = await this.isIncludeUntriagedDigestsCheckboxChecked();
    searchCriteria.includeDigestsNotAtHead = await this.isIncludeDigestsNotAtHeadCheckboxChecked();
    searchCriteria.includeIgnoredDigests = await this.isIncludeIgnoredDigestsCheckboxChecked();

    // More filters dialog.
    await this.clickMoreFiltersBtn();
    const filterDialogSkPO = await this.filterDialogSkPO!;
    const filters = await filterDialogSkPO?.getSelectedFilters();
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
    const corpusSelectorSkPO = await this.corpusSelectorSkPO;
    if (await corpusSelectorSkPO?.getSelectedCorpus() !== searchCriteria.corpus) {
      await corpusSelectorSkPO?.clickCorpus(searchCriteria.corpus);
    }

    // Left-hand traces.
    const traceFilterSkPO = await this.traceFilterSkPO;
    await traceFilterSkPO?.clickEditBtn();
    await traceFilterSkPO?.setQueryDialogSkSelection(searchCriteria.leftHandTraceFilter);
    await traceFilterSkPO?.clickQueryDialogSkShowMatchesBtn();

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
    const filterDialogSkPO = await this.filterDialogSkPO;
    await filterDialogSkPO?.setSelectedFilters({
      diffConfig: searchCriteria.rightHandTraceFilter,
      minRGBADelta: searchCriteria.minRGBADelta,
      maxRGBADelta: searchCriteria.maxRGBADelta,
      mustHaveReferenceImage: searchCriteria.mustHaveReferenceImage,
      sortOrder: searchCriteria.sortOrder
    });
    await filterDialogSkPO?.clickFilterBtn();
  }
};
