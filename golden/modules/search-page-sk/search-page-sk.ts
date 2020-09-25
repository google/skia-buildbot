/**
 * @module modules/search-page-sk
 * @description <h2><code>search-page-sk</code></h2>
 *
 */
import { html } from 'lit-html';
import { define } from 'elements-sk/define';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { deepCopy } from 'common-sk/modules/object';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { ParamSet, fromParamSet, fromObject } from 'common-sk/modules/query';
import dialogPolyfill from 'dialog-polyfill';
import { HintableObject } from 'common-sk/modules/hintable';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { ChangelistControlsSkChangeEventDetail } from '../changelist-controls-sk/changelist-controls-sk';
import { SearchCriteria, SearchCriteriaToHintableObject, SearchCriteriaFromHintableObject } from '../search-controls-sk/search-controls-sk';
import { sendBeginTask, sendEndTask, sendFetchError } from '../common';
import { defaultCorpus } from '../settings';
import { SearchResponse, StatusResponse, ParamSetResponse, SearchResult, ChangeListSummaryResponse, TriageRequestData, Label } from '../rpc_types';

import 'elements-sk/checkbox-sk';
import 'elements-sk/styles/buttons';
import '../bulk-triage-sk';
import '../search-controls-sk';
import '../changelist-controls-sk';
import '../digest-details-sk';

// Used to include/exclude the corpus field from the various ParamSets being passed around.
const CORPUS_KEY = 'source_type';

/**
 * Counterpart to SearchRespose (declared in rpc_types.ts).
 *
 * Contains the query string arguments to the /json/v1/search RPC. Intended to be used with
 * common-sk's fromObject() function.
 *
 * This type cannot be generated from Go because there is no counterpart Go struct.
 *
 * TODO(lovisolo): Consider reworking the /json/v1/search RPC to take arguments via POST, so that
 *                 we're able to unmarshal the JSON arguments into a SearchRequest Go struct. That
 *                 struct can then be converted into TypeScript via go2ts and used here, instead of
 *                 the ad-hoc SearchRequest interface defined below.
 * TODO(lovisolo): Consider generating the SearchCriteria struct from the above Go struct so we can
 *                 use the same type across the whole stack.
 */
export interface SearchRequest {
  // Fields populated via the page's query string.
  blame?: string;
  crs?: string;
  issue?: string;

  // Fields populated via the changelist-controls-sk.
  master?: boolean; // Show all results if true, or exclude results from the master branch if false.
  patchsets?: number;

  // Fields populated via the search-controls-sk.
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
    <!-- TODO(lovisolo): Add "Help" button. -->

    <div class="top-controls">
      <search-controls-sk .corpora=${el._corpora}
                          .searchCriteria=${el._searchCriteria}
                          .paramSet=${el._paramSet}
                          @search-controls-sk-change=${el._onSearchControlsChange}>
      </search-controls-sk>
      <button class="bulk-triage" @click=${() => el._bulkTriageDialog?.showModal()}>
        Bulk Triage
      </button>
    </div>

    <!-- This is only visible when the summary property is not null. -->
    <changelist-controls-sk .ps_order=${el._patchset}
                            .include_master=${el._includeDigestsFromPrimary}
                            .summary=${el._changeListSummaryResponse}
                            @cl-control-change=${el._onChangelistControlsChange}>
    </changelist-controls-sk>

    <p class=summary>${SearchPageSk._summary(el)}</p>

    ${el._searchResponse?.digests?.map((result) => SearchPageSk._resultTemplate(el, result!))}

    <dialog class="bulk-triage">
      <bulk-triage-sk .currentPageDigests=${el._getCurrentPageDigestsTriageRequestData()}
                      .allDigests=${el._searchResponse?.bulk_triage_data || {}}
                      .crs=${el._crs || ''}
                      .changeListID=${el._changelistId || ''}
                      @bulk_triage_invoked=${() => el._bulkTriageDialog?.close()}
                      @bulk_triage_finished=${() => el._fetchSearchResults()}
                      @bulk_triage_cancelled=${() => el._bulkTriageDialog?.close()}>
      </bulk-triage-sk>
    </dialog>`;

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

  // TODO(lovisolo): Add keyboard shortcuts (J, K, W, A, S, D, ?).
  private static _resultTemplate = (el: SearchPageSk, result: SearchResult) => html`
    <digest-details-sk .commits=${el._searchResponse?.commits}
                       .details=${result}
                       .changeListID=${el._changelistId}
                       .crs=${el._crs}}>
    </digest-details-sk>
  `;

  // Reflected to/from the URL and modified by the search-controls-sk.
  private _searchCriteria: SearchCriteria = {
    corpus: defaultCorpus(),
    leftHandTraceFilter: {},
    rightHandTraceFilter: {},
    includePositiveDigests: false,
    includeNegativeDigests: false,
    includeUntriagedDigests: true,
    includeDigestsNotAtHead: false,
    includeIgnoredDigests: false,
    minRGBADelta: 0,
    maxRGBADelta: 255,
    mustHaveReferenceImage: false,
    sortOrder: 'descending'
  };

  // Fields reflected to/from the URL and modified by the changelist-controls-sk.
  private _includeDigestsFromPrimary: boolean | null = null;
  private _patchset: number | null = null;

  // Other fields reflected from the URL.
  private _blame: string | null = null;
  private _crs: string | null = null;
  private _changelistId: string | null = null;

  // stateReflector update function.
  private _stateChanged: (() => void) | null = null;

  // Fields populated from JSON RPCs.
  private _corpora: string[] = [];
  private _paramSet: ParamSet = {};
  private _changeListSummaryResponse: ChangeListSummaryResponse | null = null;
  private _searchResponse: SearchResponse | null = null;

  private _searchResultsFetchController: AbortController | null = null;

  private _bulkTriageDialog: HTMLDialogElement | null = null;

  constructor() {
    super(SearchPageSk._template);

    this._stateChanged = stateReflector(
      /* getState */ () => {
        const state = SearchCriteriaToHintableObject(this._searchCriteria) as HintableObject;
        state.blame = this._blame || '';
        state.crs = this._crs || '';
        state.issue = this._changelistId || '';
        state.master = this._includeDigestsFromPrimary || '';
        state.patchsets = this._patchset || '';
        return state;
      },
      /* setState */ (newState) => {
        if (!this._connected) {
          return;
        }
        this._searchCriteria = SearchCriteriaFromHintableObject(newState);
        this._blame = (newState.blame as string) || null;
        this._crs = (newState.crs as string) || null;
        this._changelistId = (newState.issue as string) || null;
        this._includeDigestsFromPrimary = (newState.master as boolean) || null;
        this._patchset = (newState.patchsets as number) || null;
        this._render();
        this._fetchChangeListSummary(); // Called here because the RPC needs the CRS and CL number.
        this._fetchSearchResults();
      },
    );
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();

    this._bulkTriageDialog = this.querySelector('dialog.bulk-triage');
    dialogPolyfill.registerDialog(this._bulkTriageDialog!);

    // It suffices to fetch the corpora and paramset once. We assume they don't change during the
    // lifecycle of the page. Worst case, the user will have to reload to get any new parameters.
    this._fetchCorpora();
    this._fetchParamSet();
  }

  private async _fetchCorpora() {
    try {
      sendBeginTask(this);
      const statusResponse: StatusResponse =
        await fetch('/json/v1/trstatus', {method: 'GET'}).then(jsonOrThrow);
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
            '/json/v1/paramset' + (changeListId ? '?changelist_id='+  changeListId : ''),
            {method: 'GET'})
          .then(jsonOrThrow);

      // TODO(lovisolo): Type ParamSetResponse is generated by go2ts as
      //                 { [key: string]: string[] | null }, but the real ParamSet type used here is
      //                 { [key: string]: string[] }. Instead of blindly typecasing, perform an
      //                 explicit check that no values are null, then convert to ParamSet.
      //                 Alternatively, add support for overriding a type definition in go2ts.
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

  private async _fetchChangeListSummary() {
    // We can skip this RPC if no CL information has been provided via URL parameters.
    if (!this._crs || !this._changelistId) return;

    // It suffices to fetch the changelist summary only once because it's not possible to change the
    // CL or CRS via the UI.
    if (this._changeListSummaryResponse) return;

    try {
      sendBeginTask(this);
      this._changeListSummaryResponse =
        await fetch(`/json/v1/changelist/${this._crs}/${this._changelistId}`, {method: 'GET'})
          .then(jsonOrThrow);
      this._render();
      sendEndTask(this);
    } catch(e) {
      sendFetchError(this, e, 'fetching the changelist summary');
    }
  }

  private async _fetchSearchResults() {
    // Force only one fetch at a time. Abort any outstanding requests.
    if (this._searchResultsFetchController) {
      this._searchResultsFetchController.abort();
    }
    this._searchResultsFetchController = new AbortController();

    // Utility function to insert the selected corpus into the left- and right-hand trace filters,
    // as required by the /json/v1/search RPC.
    const insertCorpus = (paramSet: ParamSet) => {
      const copy = deepCopy(paramSet);
      copy[CORPUS_KEY] = [this._searchCriteria.corpus];
      return copy;
    }

    // Populate a SearchRequest object, which we'll use to generate the query string for the
    // /json/v1/search RPC.
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

    // Populate optional query parameters.
    if (this._blame) searchRequest.blame = this._blame;
    if (this._crs) searchRequest.crs = this._crs;
    if (this._changelistId) searchRequest.issue = this._changelistId;
    if (this._includeDigestsFromPrimary) searchRequest.master = this._includeDigestsFromPrimary;
    if (this._patchset) searchRequest.patchsets = this._patchset;

    try {
      sendBeginTask(this);
      const searchResponse: SearchResponse =
        await fetch(
            '/json/v1/search?' + fromObject(searchRequest as any),
            {method: 'GET', signal: this._searchResultsFetchController.signal})
          .then(jsonOrThrow);
      this._searchResponse = searchResponse;
      this._render();
      sendEndTask(this);
    } catch(e) {
      sendFetchError(this, e, 'fetching the available digest parameters');
    }
  }

  private _getCurrentPageDigestsTriageRequestData() {
    const triageRequestData: TriageRequestData = {};

    if (!this._searchResponse) return triageRequestData;

    for (const result of this._searchResponse.digests!) {
      let byTest = triageRequestData[result!.test];
      if (!byTest) {
        byTest = {};
        triageRequestData[result!.test] = byTest;
      }
      let valueToSet: Label | '' = '';
      if (result!.closestRef === 'pos') {
        valueToSet = 'positive';
      } else if (result!.closestRef === 'neg') {
        valueToSet = 'negative';
      }
      // Note: We cast this potentially empty string as a Label due to the legacy behaviors
      // documented here:
      // https://github.com/google/skia-buildbot/blob/6dd58fac8d1eac7bbf4e737110605dcdf1b20a56/golden/modules/bulk-triage-sk/bulk-triage-sk.ts#L134
      // TODO(lovisolo): Clean this up after the legacy search-page-sk is removed.
      byTest[result!.digest] = valueToSet as Label;
    }

    return triageRequestData;
  }

  private _onSearchControlsChange(event: CustomEvent<SearchCriteria>) {
    this._searchCriteria = event.detail;
    this._stateChanged!();
    this._fetchSearchResults();
  }

  private _onChangelistControlsChange(event: CustomEvent<ChangelistControlsSkChangeEventDetail>) {
    this._includeDigestsFromPrimary = event.detail.include_master;
    this._patchset = event.detail.ps_order;
    this._stateChanged!();
    this._fetchSearchResults();
  }
}

define('search-page-sk', SearchPageSk);
