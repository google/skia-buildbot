/**
 * @module modules/search-controls-sk
 * @description <h2><code>search-controls-sk</code></h2>
 *
 * A component that allows the user to view and edit a digest search criteria.
 *
 * Events:
 *   search-controls-sk-change: Emitted when the user changes the search criteria.
 */
import { html } from 'lit-html';
import { live } from 'lit-html/directives/live';
import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { deepCopy } from 'common-sk/modules/object';
import { fromParamSet, toParamSet, ParamSet } from 'common-sk/modules/query';
import { HintableObject } from 'common-sk/modules/hintable';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { FilterDialogSk, Filters } from '../filter-dialog-sk/filter-dialog-sk';
import { defaultCorpus } from '../settings';

import 'elements-sk/checkbox-sk';
import 'elements-sk/styles/buttons';
import '../corpus-selector-sk';
import '../filter-dialog-sk';
import '../trace-filter-sk';

/** A digest search criteria.  */
export interface SearchCriteria {
  corpus: string;

  leftHandTraceFilter: ParamSet;
  rightHandTraceFilter: ParamSet;

  // Left-hand digest settings.
  includePositiveDigests: boolean;
  includeNegativeDigests: boolean;
  includeUntriagedDigests: boolean;
  includeDigestsNotAtHead: boolean;
  includeIgnoredDigests: boolean;

  // Right-hand digest settings.
  minRGBADelta: number; // Valid values are integers from 0 to 255.
  maxRGBADelta: number; // Valid values are integers from 0 to 255.
  mustHaveReferenceImage: boolean;

  sortOrder: 'ascending' | 'descending';
}

/**
 * Describes the HintableObject used by the SearchCriteria(From/To)HintableObject() functions below.
 */
export interface SearchCriteriaHintableObject {
  corpus?: string;
  left_filter?: string;
  right_filter?: string;
  positive?: boolean;
  negative?: boolean;
  untriaged?: boolean;
  not_at_head?: boolean;
  include_ignored?: boolean;
  min_rgba?: number;
  max_rgba?: number;
  reference_image_required?: boolean;
  sort?: 'ascending' | 'descending';
}

/**
 * Returns a SearchCriteria built from a HintableObject with sensible defaults in case of any
 * missing values. This is intended to be used with common-sk's stateReflector() function.
 */
export function SearchCriteriaFromHintableObject(
    hintObj: SearchCriteriaHintableObject | HintableObject): SearchCriteria {
  return {
    corpus: (hintObj.corpus as string) || defaultCorpus(),

    leftHandTraceFilter: toParamSet(hintObj.left_filter as string),
    rightHandTraceFilter: toParamSet(hintObj.right_filter as string),

    includePositiveDigests: !!hintObj.positive,
    includeNegativeDigests: !!hintObj.negative,
    includeUntriagedDigests: !!hintObj.untriaged,
    includeDigestsNotAtHead: !!hintObj.not_at_head,
    includeIgnoredDigests: !!hintObj.include_ignored,

    minRGBADelta: +(hintObj.min_rgba || 0),
    maxRGBADelta: +(hintObj.max_rgba || 255),
    mustHaveReferenceImage: !!hintObj.reference_image_required,

    sortOrder: (hintObj.sort as 'ascending' | 'descending') || 'descending',
  }
}

/**
 * Returns a HintableObject built from a SearchCriteria, suitable to be used with common-sk's
 * searchReflector() function.
 *
 * If the given SearchCriteria is partial, it will be filled out with falsey values before the
 * conversion to HintableObject takes place.
 */
export function SearchCriteriaToHintableObject(
    sc: SearchCriteria | Partial<SearchCriteria>): SearchCriteriaHintableObject {
  return {
    corpus: sc.corpus || '',

    left_filter: fromParamSet(sc.leftHandTraceFilter!), // Note: this is converted to a string.
    right_filter: fromParamSet(sc.rightHandTraceFilter!), // Note this is converted to a string.

    positive: sc.includePositiveDigests || false,
    negative: sc.includeNegativeDigests || false,
    untriaged: sc.includeUntriagedDigests || false,
    not_at_head: sc.includeDigestsNotAtHead || false,
    include_ignored: sc.includeIgnoredDigests || false,

    min_rgba: sc.minRGBADelta || 0,
    max_rgba: sc.maxRGBADelta || 0,
    reference_image_required: sc.mustHaveReferenceImage || false,

    sort: sc.sortOrder || 'descending',
  }
}

/** A component that allows the user to view and edit a digest search criteria. */
export class SearchControlsSk extends ElementSk {
  private static _template = (el: SearchControlsSk) => html`
    <corpus-selector-sk .corpora=${el._corpora}
                        .selectedCorpus=${live(el._searchCriteria.corpus)}
                        @corpus-selected=${el._onCorpusSelected}>
    </corpus-selector-sk>

    <div class=digests>
      <span class=legend>Digests:</span>

      ${SearchControlsSk._checkBoxTemplate(
          el,
          /* label= */ 'Positive',
          /* cssClass= */ 'include-positive-digests',
          /* fieldName= */ 'includePositiveDigests')}

      ${SearchControlsSk._checkBoxTemplate(
          el,
          /* label= */ 'Negative',
          /* cssClass= */ 'include-negative-digests',
          /* fieldName= */ 'includeNegativeDigests')}

      ${SearchControlsSk._checkBoxTemplate(
          el,
          /* label= */ 'Untriaged',
          /* cssClass= */ 'include-untriaged-digests',
          /* fieldName= */ 'includeUntriagedDigests')}

      ${SearchControlsSk._checkBoxTemplate(
          el,
          /* label= */ 'Include not at HEAD',
          /* cssClass= */ 'include-digests-not-at-head',
          /* fieldName= */ 'includeDigestsNotAtHead')}

      ${SearchControlsSk._checkBoxTemplate(
          el,
          /* label= */ 'Include ignored',
          /* cssClass= */ 'include-ignored-digests',
          /* fieldName= */ 'includeIgnoredDigests')}

      <button class=more-filters @click=${el._openFilterDialog}>More filters</button>
    </div>

    <div class=traces>
      <span class=legend>Traces:</span>
      <trace-filter-sk .selection=${el._searchCriteria.leftHandTraceFilter}
                      .paramSet=${el._paramSet}
                      @trace-filter-sk-change=${el._onTraceFilterSkChange}>
      </trace-filter-sk>
    </div>

    <filter-dialog-sk @edit=${el._onFilterDialogSkEdit}></filter-dialog-sk>`;

  private static _checkBoxTemplate =
      (el: SearchControlsSk,
       label: string,
       cssClass: string,
       fieldName: keyof SearchCriteria) => {
    const onChange = (e: Event) => {
      (el._searchCriteria[fieldName] as boolean) = (e.target as HTMLInputElement).checked;
      el._emitChangeEvent();
    };
    return html`
      <checkbox-sk label="${label}"
                   class="${cssClass}"
                   ?checked=${live(el.searchCriteria[fieldName])}
                   @change=${onChange}>
      </checkbox-sk>`;
  };

  private _filterDialog: FilterDialogSk | null = null;

  private _corpora: string[] = [];
  private _paramSet: ParamSet = {};

  private _searchCriteria: SearchCriteria = {
    corpus: '',
    leftHandTraceFilter: {},
    rightHandTraceFilter: {},
    includePositiveDigests: false,
    includeNegativeDigests: false,
    includeUntriagedDigests: false,
    includeDigestsNotAtHead: false,
    includeIgnoredDigests: false,
    minRGBADelta: 0,
    maxRGBADelta: 0,
    mustHaveReferenceImage: false,
    sortOrder: 'ascending'
  };

  constructor() {
    super(SearchControlsSk._template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this._filterDialog = $$<FilterDialogSk>('filter-dialog-sk', this);
  }

  /** The available corpora. */
  get corpora() { return this._corpora; }

  set corpora(value) {
    this._corpora = value;
    this._render();
  }

  /** The set of parameters from all available traces. */
  get paramSet() { return this._paramSet; }

  set paramSet(value) {
    this._paramSet = value;
    this._render();
  }

  /** The digest search criteria. */
  get searchCriteria() { return deepCopy(this._searchCriteria); }

  set searchCriteria(value) {
    this._searchCriteria = value;
    this._render();
  }

  private _openFilterDialog() {
    // TODO(lovisolo): Make filter-dialog-sk use SearchCriteria directly once we delete the Polymer
    //                 version of search-controls-sk.
    const filters: Filters = {
      diffConfig: this._searchCriteria.rightHandTraceFilter,
      minRGBADelta: this._searchCriteria.minRGBADelta,
      maxRGBADelta: this._searchCriteria.maxRGBADelta,
      sortOrder: this._searchCriteria.sortOrder,
      mustHaveReferenceImage: this._searchCriteria.mustHaveReferenceImage,
    };

    this._filterDialog!.open(this._paramSet, filters);
  }

  private _onCorpusSelected(e: Event) {
    this._searchCriteria.corpus = (e as CustomEvent<string>).detail;
    this._emitChangeEvent();
  }

  private _onTraceFilterSkChange(e: CustomEvent<ParamSet>) {
    this._searchCriteria.leftHandTraceFilter = e.detail;
    this._render();
    this._emitChangeEvent();
  }

  private _onFilterDialogSkEdit(e: CustomEvent<Filters>) {
    const filters = e.detail;
    this._searchCriteria.rightHandTraceFilter = filters.diffConfig;
    this._searchCriteria.minRGBADelta = filters.minRGBADelta;
    this._searchCriteria.maxRGBADelta = filters.maxRGBADelta;
    this._searchCriteria.sortOrder = filters.sortOrder;
    this._searchCriteria.mustHaveReferenceImage = filters.mustHaveReferenceImage;
    this._emitChangeEvent();
  }

  private _emitChangeEvent() {
    this.dispatchEvent(new CustomEvent<SearchCriteria>('search-controls-sk-change', {
      detail: this._searchCriteria,
      bubbles: true
    }));
  }
}

define('search-controls-sk', SearchControlsSk);
