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
import { fromObject, fromParamSet, ParamSet } from 'common-sk/modules/query';
import dialogPolyfill from 'dialog-polyfill';
import { HintableObject } from 'common-sk/modules/hintable';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { ChangelistControlsSkChangeEventDetail } from '../changelist-controls-sk/changelist-controls-sk';
import {
  SearchCriteria,
  SearchCriteriaFromHintableObject,
  SearchCriteriaToHintableObject,
} from '../search-controls-sk/search-controls-sk';
import { sendBeginTask, sendEndTask, sendFetchError } from '../common';
import { defaultCorpus } from '../settings';
import {
  ChangelistSummaryResponse,
  Label,
  ParamSetResponse,
  SearchResponse,
  SearchResult,
  StatusResponse,
  TriageRequestData,
} from '../rpc_types';

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
  head: boolean; // At head only.
  include: boolean; // Include ignored.
  frgbamin: number;
  frgbamax: number;
  fref: boolean;
  sort: 'asc' | 'desc';
}

export class SearchPageSk extends ElementSk {
  private static template = (el: SearchPageSk) => html`
    <div class="top-controls">
      <search-controls-sk .corpora=${el.corpora}
                          .searchCriteria=${el.searchCriteria}
                          .paramSet=${el.paramSet}
                          @search-controls-sk-change=${el.onSearchControlsChange}>
      </search-controls-sk>
      <div class="buttons">
        <button class="bulk-triage" @click=${() => el.bulkTriageDialog?.showModal()}>
          Bulk Triage
        </button>
        <button class="help" @click=${() => el.helpDialog?.showModal()}>
          Help
        </button>
      </div>
    </div>

    <!-- This is only visible when the summary property is not null. -->
    <changelist-controls-sk .ps_order=${el.patchset}
                            .include_master=${el.includeDigestsFromPrimary}
                            .summary=${el.changeListSummaryResponse}
                            @cl-control-change=${el.onChangelistControlsChange}>
    </changelist-controls-sk>

    <p class=summary>${SearchPageSk.summary(el)}</p>

    <div class="results">
      ${el.searchResponse?.digests?.map(
    (result: SearchResult, idx: number) => SearchPageSk.resultTemplate(
      el, result!, /* selected= */ idx === el.selectedSearchResultIdx,
    ),
  )}
    </div>

    <dialog class="bulk-triage">
      <bulk-triage-sk .currentPageDigests=${el.getCurrentPageDigestsTriageRequestData()}
                      .allDigests=${el.searchResponse?.bulk_triage_data || {}}
                      .crs=${el.crs || ''}
                      .changeListID=${el.changelistId || ''}
                      .useOldAPI=${el.useOldAPI}
                      @bulk_triage_invoked=${() => el.bulkTriageDialog?.close()}
                      @bulk_triage_finished=${() => el.fetchSearchResults()}
                      @bulk_triage_cancelled=${() => el.bulkTriageDialog?.close()}>
      </bulk-triage-sk>
    </dialog>

    <dialog class="help">
      <h2>Keyboard shortcuts</h2>
      <dl>
        <dt>J</dt> <dd>Next digest</dd>
        <dt>K</dt> <dd>Previous digest</dd>
        <dt>W</dt> <dd>Zoom into current digest</dd>
        <dt>A</dt> <dd>Mark as positive</dd>
        <dt>S</dt> <dd>Mark as negative</dd>
        <dt>D</dt> <dd>Mark as untriaged</dd>
        <dt>?</dt> <dd>Show help dialog</dd>
      </dl>
      <div class="buttons">
        <button class="cancel action" @click=${() => el.helpDialog?.close()}>Close</button>
      </div>
    </dialog>`;

  private static summary = (el: SearchPageSk) => {
    if (!el.searchResponse) {
      return ''; // No results have been loaded yet. It's OK not to show anything at this point.
    }

    if (el.searchResponse.size === 0) {
      return 'No results matched your search criteria.';
    }

    const first = el.searchResponse.offset + 1;
    const last = el.searchResponse.offset + el.searchResponse.digests!.length;
    const total = el.searchResponse.size;
    return `Showing results ${first} to ${last} (out of ${total}).`;
  }

  // Note: The "selected" class is added/removed via DOM manipulations outside of lit-html for
  // performance reasons when navigating search results via the "J" and "K" keyboard shortcuts.
  // This is because re-rendering the search page can be very slow when displaying a large number of
  // search results.
  private static resultTemplate =
    (el: SearchPageSk, result: SearchResult, selected: boolean) => html`
      <digest-details-sk .commits=${el.searchResponse?.commits}
                         .details=${result}
                         .changeListID=${el.changelistId}
                         .crs=${el.crs}
                         .useOldAPI=${el.useOldAPI}
                         @triage=${(e: CustomEvent<Label>) => el.onTriage(result, e.detail)}
                         class="${selected ? 'selected' : ''}">
      </digest-details-sk>
    `;

  // Reflected to/from the URL and modified by the search-controls-sk.
  private searchCriteria: SearchCriteria = {
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
    sortOrder: 'descending',
  };

  // Fields reflected to/from the URL and modified by the changelist-controls-sk.
  private includeDigestsFromPrimary: boolean | null = null;

  private patchset: number | null = null;

  // Other fields reflected from the URL.
  private blame: string | null = null;

  private crs: string | null = null;

  private changelistId: string | null = null;

  private useOldAPI: boolean = false;

  // stateReflector update function.
  private readonly stateChanged: (()=> void) | null;

  // Fields populated from JSON RPCs.
  private corpora: string[] = [];

  private paramSet: ParamSet = {};

  private changeListSummaryResponse: ChangelistSummaryResponse | null = null;

  private searchResponse: SearchResponse | null = null;

  private searchResultsFetchController: AbortController | null = null;

  private bulkTriageDialog: HTMLDialogElement | null = null;

  private helpDialog: HTMLDialogElement | null = null;

  private keyDownEventHandlerFn: ((event: KeyboardEvent)=> void) | null = null;

  // Search result currently selected (e.g. via the J and K keyboard shortcuts). A negative value
  // represents an empty selection.
  private selectedSearchResultIdx: number = -1;

  constructor() {
    super(SearchPageSk.template);

    this.stateChanged = stateReflector(
      /* getState */ () => {
        const state = SearchCriteriaToHintableObject(this.searchCriteria) as HintableObject;
        state.blame = this.blame || '';
        state.crs = this.crs || '';
        state.issue = this.changelistId || '';
        state.use_old_api = this.useOldAPI || '';
        state.master = this.includeDigestsFromPrimary || '';
        state.patchsets = this.patchset || '';
        return state;
      },
      /* setState */ (newState) => {
        if (!this._connected) {
          return;
        }
        this.searchCriteria = SearchCriteriaFromHintableObject(newState);
        this.blame = (newState.blame as string) || null;
        this.crs = (newState.crs as string) || null;
        this.changelistId = (newState.issue as string) || null;
        this.useOldAPI = (newState.use_old_api === 'true') || false;
        this.includeDigestsFromPrimary = (newState.master as boolean) || null;
        this.patchset = (newState.patchsets as number) || null;

        // These RPCs are only called once during the page's lifetime.
        this.fetchCorporaOnce();
        this.fetchParamSetOnce();
        this.maybeFetchChangelistSummaryOnce(); // Only called if the CL/CRS URL params are set.

        // Called every time the state changes.
        this.fetchSearchResults();

        this._render();
      },
    );
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();

    this.keyDownEventHandlerFn = (event: KeyboardEvent) => this.onKeyDown(event);
    document.addEventListener('keydown', this.keyDownEventHandlerFn);

    this.bulkTriageDialog = this.querySelector('dialog.bulk-triage');
    dialogPolyfill.registerDialog(this.bulkTriageDialog!);

    this.helpDialog = this.querySelector('dialog.help');
    dialogPolyfill.registerDialog(this.helpDialog!);
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    document.removeEventListener('keydown', this.keyDownEventHandlerFn!);
  }

  private async fetchCorporaOnce() {
    // Only fetch once. We assume this doesn't change during the page's lifetime.
    if (this.corpora.length > 0) return;

    try {
      sendBeginTask(this);
      const statusResponse: StatusResponse = await fetch('/json/v2/trstatus', { method: 'GET' }).then(jsonOrThrow);
      this.corpora = statusResponse.corpStatus.map((corpus) => corpus.name);
      this._render();
      sendEndTask(this);
    } catch (e) {
      sendFetchError(this, e, 'fetching the available corpora');
    }
  }

  private async fetchParamSetOnce(changeListId?: number) {
    // Only fetch once. We assume this doesn't change during the page's lifetime.
    if (Object.keys(this.paramSet).length > 0) return;

    try {
      sendBeginTask(this);
      const url = this.useOldAPI ? '/json/v1/paramset' : '/json/v2/paramset';
      const paramSetResponse: ParamSetResponse = await fetch(
        url + (changeListId ? `?changelist_id=${changeListId}` : ''),
        { method: 'GET' },
      )
        .then(jsonOrThrow);

      // TODO(lovisolo): Type ParamSetResponse is generated by go2ts as
      //                 { [key: string]: string[] | null }, but the real ParamSet type used here is
      //                 { [key: string]: string[] }. Instead of blindly typecasing, perform an
      //                 explicit check that no values are null, then convert to ParamSet.
      //                 Alternatively, add support for overriding a type definition in go2ts.
      this.paramSet = paramSetResponse as ParamSet;

      // Remove the corpus to prevent it from showing up in the search controls left- and right-hand
      // trace filter selectors.
      delete this.paramSet[CORPUS_KEY];

      this._render();
      sendEndTask(this);
    } catch (e) {
      sendFetchError(this, e, 'fetching the available digest parameters');
    }
  }

  private async maybeFetchChangelistSummaryOnce() {
    // We can skip this RPC if no CL information has been provided via URL parameters.
    if (!this.crs || !this.changelistId) return;

    // Only fetch once. This is OK because the changelist cannot be changed via the UI.
    if (this.changeListSummaryResponse) return;

    try {
      sendBeginTask(this);
      const base = this.useOldAPI ? '/json/v1/changelist' : '/json/v2/changelist';
      this.changeListSummaryResponse = await fetch(`${base}/${this.crs}/${this.changelistId}`, { method: 'GET' })
        .then(jsonOrThrow);
      this._render();
      sendEndTask(this);
    } catch (e) {
      sendFetchError(this, e, 'fetching the changelist summary');
    }
  }

  private makeSearchRequest(): SearchRequest {
    // Utility function to insert the selected corpus into the left- and right-hand trace filters,
    // as required by the /json/v1/search RPC.
    const insertCorpus = (paramSet: ParamSet) => {
      const copy = deepCopy(paramSet);
      copy[CORPUS_KEY] = [this.searchCriteria.corpus];
      return copy;
    };

    // Populate a SearchRequest object, which we'll use to generate the query string for the
    // /json/v1/search RPC.
    const searchRequest: SearchRequest = {
      query: fromParamSet(insertCorpus(this.searchCriteria.leftHandTraceFilter)),
      rquery: fromParamSet(insertCorpus(this.searchCriteria.rightHandTraceFilter)),
      pos: this.searchCriteria.includePositiveDigests,
      neg: this.searchCriteria.includeNegativeDigests,
      unt: this.searchCriteria.includeUntriagedDigests,
      head: !this.searchCriteria.includeDigestsNotAtHead, // Inverted because head = at head only.
      include: this.searchCriteria.includeIgnoredDigests,
      frgbamin: this.searchCriteria.minRGBADelta,
      frgbamax: this.searchCriteria.maxRGBADelta,
      fref: this.searchCriteria.mustHaveReferenceImage,
      sort: this.searchCriteria.sortOrder === 'ascending' ? 'asc' : 'desc',
    };

    // Populate optional query parameters.
    if (this.blame) searchRequest.blame = this.blame;
    if (this.crs) searchRequest.crs = this.crs;
    if (this.changelistId) searchRequest.issue = this.changelistId;
    if (this.includeDigestsFromPrimary) searchRequest.master = this.includeDigestsFromPrimary;
    if (this.patchset) searchRequest.patchsets = this.patchset;

    return searchRequest;
  }

  private async fetchSearchResults() {
    // Force only one fetch at a time. Abort any outstanding requests.
    if (this.searchResultsFetchController) {
      this.searchResultsFetchController.abort();
    }
    this.searchResultsFetchController = new AbortController();

    const searchRequest = this.makeSearchRequest();

    try {
      sendBeginTask(this);
      if (this.useOldAPI) {
        this.searchResponse = await fetch(
          `/json/v1/search?${fromObject(searchRequest as any)}`,
          { method: 'GET', signal: this.searchResultsFetchController.signal },
        )
          .then(jsonOrThrow);
      } else {
        this.searchResponse = await fetch(
          `/json/v2/search?${fromObject(searchRequest as any)}`,
          { method: 'GET', signal: this.searchResultsFetchController.signal },
        )
          .then(jsonOrThrow);
      }

      // Reset UI and render.
      this.clearSelectedSearchResult();
      this._render();
      sendEndTask(this);
    } catch (e) {
      sendFetchError(this, e, 'fetching the available digest parameters');
    }
  }

  private getCurrentPageDigestsTriageRequestData() {
    const triageRequestData: TriageRequestData = {};

    if (!this.searchResponse) return triageRequestData;

    for (const result of this.searchResponse.digests!) {
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

  private onSearchControlsChange(event: CustomEvent<SearchCriteria>) {
    this.searchCriteria = event.detail;
    this.stateChanged!();
    this.fetchSearchResults();
  }

  private onChangelistControlsChange(event: CustomEvent<ChangelistControlsSkChangeEventDetail>) {
    this.includeDigestsFromPrimary = event.detail.include_master;
    this.patchset = event.detail.ps_order;
    this.stateChanged!();
    this.fetchSearchResults();
    this._render();
  }

  private onTriage(result: SearchResult, label: Label) {
    // When the user triages a digest, we patch the corresponding cached SearchResult with the new
    // label. This prevents the digest-details-sk component from reverting to the original label
    // when the search-page-sk is re-rendered with the same cached SearchResults.
    result.status = label;
  }

  private onKeyDown(event: KeyboardEvent) {
    // Ignore all keyboard shortcuts if there are any open modals.
    if (document.querySelectorAll('dialog[open]').length > 0) return;

    switch (event.key) {
      // Next.
      case 'j':
        this.selectSearchResult(this.selectedSearchResultIdx + 1);
        break;

      // Previous.
      case 'k':
        this.selectSearchResult(this.selectedSearchResultIdx - 1);
        break;

      // Zoom in.
      case 'w':
        this.openZoomDialogForSelectedSearchResult();
        break;

      // Mark as positive.
      case 'a':
        this.triageSelectedSearchResult('positive');
        break;

      // Mark as negative.
      case 's':
        this.triageSelectedSearchResult('negative');
        break;

      // Mark as untriaged.
      case 'd':
        this.triageSelectedSearchResult('untriaged');
        break;

      // Show help dialog.
      case '?':
        this.helpDialog?.showModal();
        break;

      default:
        return; // Do not stop propagation if we haven't captured the event.
    }

    event.stopPropagation(); // Stop propagation if we captured the event.
  }

  /**
   * Selects the search result with the given index, i.e. it draws a box around its corresponding
   * digest-details-sk element to indicate focus and scrolls it into view.
   */
  private selectSearchResult(index: number) {
    const searchResults = this.searchResponse?.digests || [];
    if (index < 0 || index >= searchResults.length) return;

    // We update the selected search result by hand to avoid re-rendering the entire page, which can
    // be very slow if there are many search results.
    this.querySelector<HTMLElement>('digest-details-sk.selected')?.classList.remove('selected');
    this.querySelector<HTMLElement>(`digest-details-sk:nth-child(${index + 1})`)
      ?.classList.add('selected');

    // We also keep track of the selected result so we can correctly add the "selected" CSS class
    // in the lit-html template in case we re-render the page with the cached search results.
    this.selectedSearchResultIdx = index;

    this.getSelectedDigestDetailsSk()!.scrollIntoView();
  }

  /** Clears the selected search result without re-rendering the entire page. */
  private clearSelectedSearchResult() {
    this.selectedSearchResultIdx = -1;
    this.querySelector<HTMLElement>('digest-details-sk.selected')?.classList.remove('selected');
  }

  /**
   * Applies the given label to the currently selected search result.
   */
  private triageSelectedSearchResult(label: Label) {
    // TODO(lovisolo): Clean this up once DigestDetailsSk is ported to TypeScript.

    const digestDetailsSk = this.getSelectedDigestDetailsSk();
    if (!digestDetailsSk) return;

    digestDetailsSk.querySelector<HTMLButtonElement>(`triage-sk button.${label}`)!.click();
  }

  /**
   * Opens the zoom dialog of the details-digest-sk element corresponding to the currently selected
   * search result.
   */
  private openZoomDialogForSelectedSearchResult() {
    // TODO(lovisolo): Clean this up once DigestDetailsSk is ported to TypeScript.

    const digestDetailsSk = this.getSelectedDigestDetailsSk();
    if (!digestDetailsSk) return;

    digestDetailsSk.querySelector<HTMLButtonElement>('button.zoom_btn')!.click();
  }

  /**
   * Returns the digest-details-sk element corresponding to the currently selected search result.
   */
  private getSelectedDigestDetailsSk() {
    // TODO(lovisolo): Clean this up once DigestDetailsSk is ported to TypeScript.

    if (this.selectedSearchResultIdx < 0) return null;
    return this.querySelector<HTMLElement>(
      `digest-details-sk:nth-child(${this.selectedSearchResultIdx + 1})`,
    );
  }
}

define('search-page-sk', SearchPageSk);
