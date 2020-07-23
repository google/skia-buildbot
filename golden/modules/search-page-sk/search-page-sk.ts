/**
 * @module modules/search-page-sk
 * @description <h2><code>search-page-sk</code></h2>
 *
 */
import { html } from 'lit-html';
import { live } from 'lit-html/directives/live';
import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { deepCopy } from 'common-sk/modules/object';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { ParamSet, fromParamSet, fromObject } from 'common-sk/modules/query';
import { HintableObject } from 'common-sk/modules/hintable';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { SearchControlsSk, SearchCriteria, SearchCriteriaToHintableObject, SearchCriteriaFromHintableObject, SearchCriteriaHintableObject } from '../search-controls-sk/search-controls-sk';
import { sendBeginTask, sendEndTask, sendFetchError } from '../common';
import { defaultCorpus } from '../settings';
import { SearchResponse, StatusResponse, ParamSetResponse, ChangeListSummaryResponse, SearchResult } from '../rpc_types';

import 'elements-sk/checkbox-sk';
import 'elements-sk/styles/buttons';
import '../search-controls-sk';
import '../digest-details-sk';

// Used to include/exclude the corpus field from the various ParamSets being passed around.
const CORPUS_KEY = 'source_type';

/**
 * Counterpart to SearchRespose (declared in rpc_types.ts).
 *
 * Contains the query string arguments to the /json/search RPC. Intended to be used with common-sk's
 * fromObject() function.
 *
 * This type cannot be generated from Go because there is no counterpart Go struct.
 *
 * TODO(lovisolo): Consider reworking the /json/search RPC to take arguments via POST, so that we're
 *                 able to unmarshal the JSON arguments into a SearchRequest Go struct. That struct
 *                 can then be converted into TypeScript via go2ts and used here, instead of the
 *                 ad-hoc SearchRequest interface defined below.
 * TODO(lovisolo): Consider generating the SearchCriteria struct from the above Go struct so we can
 *                 use the same type across the whole stack, much like a protocol buffer.
 */
export interface SearchRequest {
  query: string;
  rquery: string;
  pos: boolean;
  neg: boolean;
  unt: boolean;
  head: boolean;  // At head only.
  include: boolean;  // Include ignored.
  frgbamin: number;
  frgbamax: number;
  fref: boolean;
  sort: 'asc' | 'desc';
}

export class SearchPageSk extends ElementSk {
  private static _template = (el: SearchPageSk) => html`
    <search-controls-sk .corpora=${el._corpora}
                        .searchCriteria=${el._searchCriteria}
                        .paramSet=${el._paramSet}
                        @search-controls-sk-change=${el._onSearchControlsChange}>
    </search-controls-sk>

    <p class=summary>${SearchPageSk._summary(el)}</p>

    ${el._searchResponse?.digests?.map((result) => SearchPageSk._resultTemplate(el, result!))}`;

  private static _summary = (el: SearchPageSk) => {
    if (!el._searchResponse) {
      return ''; // No results have been loaded yet. It's OK not to show anything at this point.
    }

    if (el._searchResponse.size === 0) {
      return 'No results matched your search criteria.';
    }

    const first = el._searchResponse.offset + 1;
    const last = el._searchResponse.offset + el._searchResponse.digests!.length;
    const total = el._searchResponse.size;
    return `Showing results ${first} to ${last} (out of ${total}).`;
  }

  private static _resultTemplate = (el: SearchPageSk, result: SearchResult) => html`
    <digest-details-sk .commits=${el._searchResponse?.commits}
                       .details=${result}
                       .issue=${''}>
    </digest-details-sk>
  `;

  private _searchCriteria: SearchCriteria = {
    corpus: defaultCorpus(),
    leftHandTraceFilter: {},
    rightHandTraceFilter: {},
    includePositiveDigests: false,
    includeNegativeDigests: false,
    includeUntriagedDigests: false,
    includeDigestsNotAtHead: false,
    includeIgnoredDigests: false,
    minRGBADelta: 0,
    maxRGBADelta: 255,
    mustHaveReferenceImage: false,
    sortOrder: 'ascending'
  };

  private _corpora: string[] = [];
  private _paramSet: ParamSet = {};
  private _searchResponse: SearchResponse | null = null;

  private _stateChanged: (() => void) | null = null;
  private _searchResultsFetchController: AbortController | null = null;

  constructor() {
    super(SearchPageSk._template);

    this._stateChanged = stateReflector(
      /* getState */ () => SearchCriteriaToHintableObject(this._searchCriteria) as HintableObject,
      /* setState */ (newState) => {
        if (!this._connected) {
          return;
        }
        this._searchCriteria = SearchCriteriaFromHintableObject(newState);
        this._render();
        this._fetchSearchResults();
      },
    );
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();

    // It suffices to fetch the corpora and paramset once. We assume they don't change during the
    // lifecycle of the page. Worst case, the user will have to reload to get any new parameters.
    this._fetchCorpora();
    this._fetchParamSet();
  }

  private async _fetchCorpora() {
    try {
      sendBeginTask(this);
      const statusResponse: StatusResponse =
        await fetch('/json/trstatus', {method: 'GET'}).then(jsonOrThrow);
      this._corpora = statusResponse.corpStatus!.map((corpus) => corpus!.name);
      this._render();
      sendEndTask(this);
    } catch(e) {
      sendFetchError(this, e, 'fetching the available corpora');
    }
  }

  private async _fetchParamSet(changeListId?: number) {
    try {
      sendBeginTask(this);
      const paramSetResponse: ParamSetResponse =
        await fetch(
            '/json/paramset' + (changeListId ? '?changelist_id='+  changeListId : ''),
            {method: 'GET'})
          .then(jsonOrThrow);

      // TODO(lovisolo): Type ParamSetResponse is generated by go2ts as
      //                 { [key: string]: string[] | null }, but the real ParamSet type used here is
      //                 { [key: string]: string[] }. Instead of blindly typecasing, perform an
      //                 explicit check that no values are nul, then convert to ParamSet.
      this._paramSet = paramSetResponse as ParamSet;

      // Remove the corpus to prevent it from showing up in the search controls left- and right-hand
      // trace filter selectors.
      delete this._paramSet[CORPUS_KEY];

      this._render();
      sendEndTask(this);
    } catch(e) {
      sendFetchError(this, e, 'fetching the available digest parameters');
    }
  }

  private async _fetchSearchResults() {
    // Force only one fetch at a time. Abort any outstanding requests.
    if (this._searchResultsFetchController) {
      this._searchResultsFetchController.abort();
    }
    this._searchResultsFetchController = new AbortController();

    // Utility to insert the selected corpus into the left- and right-hand trace filters, as
    // required by the /json/search RPC.
    const insertCorpus = (paramSet: ParamSet) => {
      const copy = deepCopy(paramSet);
      copy[CORPUS_KEY] = [this._searchCriteria.corpus];
      return copy;
    }

    // Populate a SearchRequest object, which we'll use to generate the query string for the
    // /json/search RPC.
    const searchRequest: SearchRequest = {
      query: fromParamSet(insertCorpus(this._searchCriteria.leftHandTraceFilter)),
      rquery: fromParamSet(insertCorpus(this._searchCriteria.rightHandTraceFilter)),
      pos: this._searchCriteria.includePositiveDigests,
      neg: this._searchCriteria.includeNegativeDigests,
      unt: this._searchCriteria.includeUntriagedDigests,
      head: !this._searchCriteria.includeDigestsNotAtHead, // Inverted because head = at head only.
      include: this._searchCriteria.includeIgnoredDigests,
      frgbamin: this._searchCriteria.minRGBADelta,
      frgbamax: this._searchCriteria.maxRGBADelta,
      fref: this._searchCriteria.mustHaveReferenceImage,
      sort: this._searchCriteria.sortOrder === 'ascending' ? 'asc' : 'desc',
    };

    try {
      sendBeginTask(this);
      const searchResponse: SearchResponse =
        await fetch(
            '/json/search?' + fromObject(searchRequest as any),
            {method: 'GET', signal: this._searchResultsFetchController.signal})
          .then(jsonOrThrow);
      this._searchResponse = searchResponse;
      this._render();
      sendEndTask(this);
    } catch(e) {
      sendFetchError(this, e, 'fetching the available digest parameters');
    }
  }

  private _onSearchControlsChange(event: CustomEvent<SearchCriteria>) {
    this._searchCriteria = event.detail;
    this._stateChanged!();
    this._fetchSearchResults();
  }
}

define('search-page-sk', SearchPageSk);
