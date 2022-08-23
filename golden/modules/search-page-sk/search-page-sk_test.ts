import './index';
import fetchMock from 'fetch-mock';
import { expect } from 'chai';
import { deepCopy } from 'common-sk/modules/object';
import { fromObject } from 'common-sk/modules/query';
import {
  searchResponse, statusResponse, paramSetResponse, changeListSummaryResponse, groupingsResponse,
} from './demo_data';
import {
  setUpElementUnderTest, eventSequencePromise, eventPromise, setQueryString, expectQueryStringToEqual, noEventPromise,
} from '../../../infra-sk/modules/test_util';
import { SearchPageSk, SearchRequest, DEFAULT_SEARCH_RESULTS_LIMIT } from './search-page-sk';
import { SearchPageSkPO } from './search-page-sk_po';
import {
  Label, SearchResponse, TriageRequestV3, TriageResponse,
} from '../rpc_types';
import { testOnlySetSettings } from '../settings';
import { SearchCriteria } from '../search-controls-sk/search-controls-sk';
import { SearchControlsSkPO } from '../search-controls-sk/search-controls-sk_po';
import { ChangelistControlsSkPO } from '../changelist-controls-sk/changelist-controls-sk_po';
import { BulkTriageSkPO } from '../bulk-triage-sk/bulk-triage-sk_po';
import { PaginationSkPO } from '../pagination-sk/pagination-sk_po';

describe('search-page-sk', () => {
  const newInstance = setUpElementUnderTest<SearchPageSk>('search-page-sk');

  let searchPageSk: SearchPageSk;
  let searchPageSkPO: SearchPageSkPO;
  let searchControlsSkPO: SearchControlsSkPO;
  let changelistControlsSkPO: ChangelistControlsSkPO;
  let bulkTriageSkPO: BulkTriageSkPO;
  let topPaginationSkPO: PaginationSkPO;
  let bottomPaginationSkPO: PaginationSkPO;

  // SearchCriteria shown by the search-controls-sk component when the search page loads without any
  // URL parameters.
  const defaultSearchCriteria: SearchCriteria = {
    corpus: 'infra',
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

  // Default request to the /json/v2/search RPC when the page is loaded with an empty query string.
  const defaultSearchRequest: SearchRequest = {
    fref: false,
    frgbamax: 255,
    frgbamin: 0,
    head: true,
    include: false,
    neg: false,
    pos: false,
    query: 'source_type=infra',
    rquery: 'source_type=infra',
    sort: 'desc',
    unt: true,
    offset: 0,
    limit: DEFAULT_SEARCH_RESULTS_LIMIT,
  };

  // Query string that will produce the searchRequestWithCL defined below upon page load.
  const queryStringWithCL = '?crs=gerrit&issue=123456';

  // Request to the /json/v2/search RPC when URL parameters crs=gerrit and issue=123456 are present.
  const searchRequestWithCL: SearchRequest = deepCopy(defaultSearchRequest);
  searchRequestWithCL.crs = 'gerrit';
  searchRequestWithCL.issue = '123456';

  // Search response when the query matches 0 digests.
  const emptySearchResponse: SearchResponse = deepCopy(searchResponse);
  emptySearchResponse.size = 0;
  emptySearchResponse.digests = [];

  // Options for the instantiate() function below.
  interface InstantiationOptions {
    initialQueryString: string;
    expectedInitialSearchRequest: SearchRequest;
    initialSearchResponse: SearchResponse;
    mockAndWaitForChangelistSummaryRPC: boolean;
  }

  // Instantiation options for tests where the URL params crs=gerrit and issue=123456 are present.
  const instantiationOptionsWithCL: Partial<InstantiationOptions> = {
    initialQueryString: queryStringWithCL,
    expectedInitialSearchRequest: searchRequestWithCL,
    mockAndWaitForChangelistSummaryRPC: true,
  };

  // Instantiates the search page, sets up the necessary mock RPCs and waits for it to load.
  const instantiate = async (opts: Partial<InstantiationOptions> = {}) => {
    const defaults: InstantiationOptions = {
      initialQueryString: '',
      expectedInitialSearchRequest: defaultSearchRequest,
      initialSearchResponse: searchResponse,
      mockAndWaitForChangelistSummaryRPC: false,
    };

    // Override defaults with the given options, if any.
    opts = { ...defaults, ...opts };

    fetchMock.getOnce('/json/v2/trstatus', () => statusResponse);
    fetchMock.getOnce('/json/v2/paramset', () => paramSetResponse);
    fetchMock.getOnce('/json/v1/groupings', () => groupingsResponse);
    fetchMock.get(
      `/json/v2/search?${fromObject(opts.expectedInitialSearchRequest as any)}`,
      () => opts.initialSearchResponse,
    );

    // We always wait for at least the four above RPCs.
    const eventsToWaitFor = ['end-task', 'end-task', 'end-task', 'end-task'];

    // This mocked RPC corresponds to the queryStringWithCL and searchRequestWithCL constants
    // defined above.
    if (opts.mockAndWaitForChangelistSummaryRPC) {
      fetchMock.getOnce('/json/v2/changelist/gerrit/123456', () => changeListSummaryResponse);
      eventsToWaitFor.push('end-task');
    }

    // The search page will derive its initial search RPC from the query parameters in the URL.
    setQueryString(opts.initialQueryString!);

    // Instantiate search page and wait for all of the above mocked RPCs to complete.
    const events = eventSequencePromise(eventsToWaitFor);
    searchPageSk = newInstance();
    await events;

    searchPageSkPO = new SearchPageSkPO(searchPageSk);
    searchControlsSkPO = searchPageSkPO.searchControlsSkPO;
    changelistControlsSkPO = searchPageSkPO.changelistControlsSkPO;
    bulkTriageSkPO = searchPageSkPO.bulkTriageSkPO;
    topPaginationSkPO = searchPageSkPO.topPaginationSkPO;
    bottomPaginationSkPO = searchPageSkPO.bottomPaginationSkPO;
  };

  before(() => {
    testOnlySetSettings({
      title: 'Skia Infra',
      defaultCorpus: 'infra',
      baseRepoURL: 'https://skia.googlesource.com/buildbot.git',
    });
  });

  afterEach(() => {
    expect(fetchMock.done()).to.be.true; // All mock RPCs called at least once.
    fetchMock.reset();
  });

  // This function adds tests to ensure that a search field in the UI is correctly bound to its
  // corresponding query parameter in the URL and to its corresponding field in the SearchRequest
  // object.
  const searchFieldIsBoundToURLAndRPC = <T>(
    instantiationOpts: Partial<InstantiationOptions>,
    queryStringWithSearchField: string,
    uiValueGetterFn: ()=> Promise<T>,
    uiValueSetterFn: ()=> Promise<void>,
    expectedUiValue: T,
    expectedSearchRequest: SearchRequest,
  ) => {
    it('is read from the URL and included in the initial search RPC', async () => {
      // We initialize the search page using a query string that contains the search field under
      // test, so that said field is included in the initial search RPC.
      //
      // If the search RPC is not called with the expected SearchRequest, the top-level
      // afterEach() hook will fail.
      await instantiate({
        ...instantiationOpts,
        initialQueryString: queryStringWithSearchField,
        expectedInitialSearchRequest: expectedSearchRequest,
      });

      // The search field in the UI should reflect the value from the URL.
      expect(await uiValueGetterFn()).to.deep.equal(expectedUiValue);
    });

    it('is reflected in the URL and included in the search RPC when set via the UI', async () => {
      // We initialize the search page using the default query string.
      await instantiate(instantiationOpts);

      // We will trigger a search RPC when we set the value of the field under test via the UI.
      // If the RPC is not called with the expected SearchRequest, the top-level afterEach() hook
      // will fail.
      fetchMock.get(
        `/json/v2/search?${fromObject(expectedSearchRequest as any)}`, () => searchResponse,
      );

      // Set the search field under test via the UI and wait for the above RPC to complete.
      const event = eventPromise('end-task');
      await uiValueSetterFn();
      await event;

      // The search field under test should now be reflected in the URL.
      expectQueryStringToEqual(queryStringWithSearchField);
    });
  };

  describe('loading indicator', () => {
    it('is visible only while the search results are loading', async () => {
      await instantiate();
      expect(await searchPageSkPO.getSummary()).to.not.equal('Loading...');

      // We will trigger a search RPC using the pagination-sk element's "next" button. The exact
      // element does not matter as long as a search RPC is triggered.
      fetchMock.get(
        `/json/v2/search?${fromObject({ ...defaultSearchRequest, offset: 50 })}`,
        () => searchResponse, // This test does not care about the search response. Any is fine.
      );

      const beginTaskEvent = eventPromise('begin-task');
      const endTaskEvent = eventPromise('end-task');

      // Trigger a search RPC using the pagination-sk element's "next" button.
      topPaginationSkPO.clickNextBtn();
      await beginTaskEvent;
      expect(await searchPageSkPO.getSummary()).to.equal('Loading...');

      // The loading indicator should go away once the results are loaded.
      await endTaskEvent;
      expect(await searchPageSkPO.getSummary()).to.not.equal('Loading...');
    });
  });

  describe('search-controls-sk', () => {
    const itIsBoundToURLAndRPC = (
      queryString: string,
      searchCriteria: Partial<SearchCriteria>,
      serachRequest: Partial<SearchRequest>,
    ) => {
      const expectedSearchCriteria: SearchCriteria = { ...defaultSearchCriteria, ...searchCriteria };
      const expectedSearchRequest: SearchRequest = { ...defaultSearchRequest, ...serachRequest };

      searchFieldIsBoundToURLAndRPC<SearchCriteria>(
        /* initializationOpts= */ {},
        queryString,
        () => searchControlsSkPO.getSearchCriteria(),
        () => searchControlsSkPO.setSearchCriteria(expectedSearchCriteria!),
        expectedSearchCriteria!,
        expectedSearchRequest!,
      );
    };

    describe('field "corpus"', () => {
      itIsBoundToURLAndRPC(
        '?corpus=my-corpus',
        { corpus: 'my-corpus' },
        { query: 'source_type=my-corpus', rquery: 'source_type=my-corpus' },
      );
    });

    describe('field "left-hand trace filter"', () => {
      itIsBoundToURLAndRPC(
        '?left_filter=name%3Dam_email-chooser-sk',
        { leftHandTraceFilter: { name: ['am_email-chooser-sk'] } },
        { query: 'name=am_email-chooser-sk&source_type=infra' },
      );
    });

    describe('field "right-hand trace filter"', () => {
      itIsBoundToURLAndRPC(
        '?right_filter=name%3Dam_email-chooser-sk',
        { rightHandTraceFilter: { name: ['am_email-chooser-sk'] } },
        { rquery: 'name=am_email-chooser-sk&source_type=infra' },
      );
    });

    describe('field "include positive digests"', () => {
      itIsBoundToURLAndRPC(
        '?positive=true',
        { includePositiveDigests: true },
        { pos: true },
      );
    });

    describe('field "include negative digests"', () => {
      itIsBoundToURLAndRPC(
        '?negative=true',
        { includeNegativeDigests: true },
        { neg: true },
      );
    });

    describe('field "include untriaged digests"', () => {
      // This field is true by default, so we set it to false.
      itIsBoundToURLAndRPC(
        '?untriaged=false',
        { includeUntriagedDigests: false },
        { unt: false },
      );
    });

    describe('field "include digests not at head"', () => {
      itIsBoundToURLAndRPC(
        '?not_at_head=true',
        { includeDigestsNotAtHead: true },
        { head: false },
      ); // SearchRequest field "head" means "at head only".
    });

    describe('field "include ignored digests"', () => {
      itIsBoundToURLAndRPC(
        '?include_ignored=true',
        { includeIgnoredDigests: true },
        { include: true },
      );
    });

    describe('field "min RGBA delta"', () => {
      itIsBoundToURLAndRPC(
        '?min_rgba=10',
        { minRGBADelta: 10 },
        { frgbamin: 10 },
      );
    });

    describe('field "max RGBA delta"', () => {
      itIsBoundToURLAndRPC(
        '?max_rgba=200',
        { maxRGBADelta: 200 },
        { frgbamax: 200 },
      );
    });

    describe('field "max RGBA delta"', () => {
      itIsBoundToURLAndRPC(
        '?max_rgba=200',
        { maxRGBADelta: 200 },
        { frgbamax: 200 },
      );
    });

    describe('field "must have reference image"', () => {
      itIsBoundToURLAndRPC(
        '?reference_image_required=true',
        { mustHaveReferenceImage: true },
        { fref: true },
      );
    });

    describe('field "sort order"', () => {
      itIsBoundToURLAndRPC(
        '?sort=ascending',
        { sortOrder: 'ascending' },
        { sort: 'asc' },
      );
    });
  });

  describe('changelist-controls-sk', () => {
    it('is hidden if no CL is provided in the query string', async () => {
      // When instantiated without URL parameters "crs" and "issue", the search page does not make
      // an RPC to /json/v2/changelist, therefore there is no changelist summary for the
      // changelist-controls-sk component to display.
      await instantiate();
      expect(await changelistControlsSkPO.isVisible()).to.be.false;
    });

    it(
      'is visible if a CL is provided in the query string and /json/v2/changelist returns a '
        + 'non-empty response',
      async () => {
      // We instantiate the serach page with URL parameters "crs" and "issue", which causes it to
      // make an RPC to /json/v2/changelist. The returned changelist summary is passed to the
      // changelist-controls-sk component, which then makes itself visible.
        await instantiate(instantiationOptionsWithCL);
        expect(await changelistControlsSkPO.isVisible()).to.be.true;
      },
    );

    describe('field "patchset"', () => {
      searchFieldIsBoundToURLAndRPC<string>(
        instantiationOptionsWithCL,
        `${queryStringWithCL}&patchsets=1`,
        () => changelistControlsSkPO.getPatchset(),
        () => changelistControlsSkPO.setPatchset('PS 1'),
        /* expectedUiValue= */ 'PS 1',
        { ...searchRequestWithCL, patchsets: 1 },
      );
    });

    describe('radio "exclude results from primary branch"', () => {
      // When this radio is clicked, the "master" parameter is removed from the URL if present, so
      // we need to test this backwards by starting with "master=true" in the URL (which means the
      // initial search RPC will include "master=true" in the SearchRequest as well) and then
      // asserting that "master" is removed from both the URL and the SearchRequest when the radio
      // is clicked.
      searchFieldIsBoundToURLAndRPC<boolean>(
        {
          ...instantiationOptionsWithCL,
          expectedInitialSearchRequest: { ...searchRequestWithCL, master: true, patchsets: 2 },
          initialQueryString: `${queryStringWithCL}&master=true&patchsets=2`,
        },
        `${queryStringWithCL}&patchsets=2`,
        () => changelistControlsSkPO.isExcludeResultsFromPrimaryRadioChecked(),
        () => changelistControlsSkPO.clickExcludeResultsFromPrimaryRadio(),
        /* expectedUiValue= */ true,
        { ...searchRequestWithCL, patchsets: 2 },
      );
    });

    describe('radio "show all results"', () => {
      searchFieldIsBoundToURLAndRPC<boolean>(
        instantiationOptionsWithCL,
        `${queryStringWithCL}&master=true&patchsets=2`,
        () => changelistControlsSkPO.isShowAllResultsRadioChecked(),
        () => changelistControlsSkPO.clickShowAllResultsRadio(),
        /* expectedUiValue= */ true,
        { ...searchRequestWithCL, master: true, patchsets: 2 },
      );
    });
  });

  const testPaginationSk = (getPaginationSkPO: ()=> PaginationSkPO) => {
    // Returns the current page displayed by both the top and bottom pagination-sk elements as a
    // single pipe-separated string (e.g. "4|4"). This allows us to test that both elements show
    // the same page number.
    const getCurrentPageFromBothPaginationSkElements = async (): Promise<string> => {
      const top = await topPaginationSkPO.getCurrentPage();
      const bottom = await bottomPaginationSkPO.getCurrentPage();
      return `${top}|${bottom}`;
    };

    describe('button "next" with no explicit "limit" URL parameter', () => {
      searchFieldIsBoundToURLAndRPC<string>(
        {
          initialQueryString: '',
          expectedInitialSearchRequest: { ...defaultSearchRequest },
        },
        '?offset=50',
        getCurrentPageFromBothPaginationSkElements,
        () => getPaginationSkPO().clickNextBtn(),
        /* expectedUiValue= */ '2|2',
        { ...defaultSearchRequest, offset: 50 },
      );
    });

    describe('button "next"', () => {
      searchFieldIsBoundToURLAndRPC<string>(
        {
          initialQueryString: '?limit=3',
          expectedInitialSearchRequest: { ...defaultSearchRequest, limit: 3 },
        },
        '?limit=3&offset=3',
        getCurrentPageFromBothPaginationSkElements,
        () => getPaginationSkPO().clickNextBtn(),
        /* expectedUiValue= */ '2|2',
        { ...defaultSearchRequest, limit: 3, offset: 3 },
      );
    });

    describe('button "skip"', () => {
      searchFieldIsBoundToURLAndRPC<string>(
        {
          initialQueryString: '?limit=3',
          expectedInitialSearchRequest: { ...defaultSearchRequest, limit: 3 },
        },
        '?limit=3&offset=15',
        getCurrentPageFromBothPaginationSkElements,
        () => getPaginationSkPO().clickSkipBtn(),
        /* expectedUiValue= */ '6|6',
        { ...defaultSearchRequest, limit: 3, offset: 15 },
      );
    });

    describe('button "prev"', () => {
      searchFieldIsBoundToURLAndRPC<string>(
        {
          initialQueryString: '?limit=3&offset=12',
          expectedInitialSearchRequest: { ...defaultSearchRequest, limit: 3, offset: 12 },
        },
        '?limit=3&offset=9',
        getCurrentPageFromBothPaginationSkElements,
        () => getPaginationSkPO().clickPrevBtn(),
        /* expectedUiValue= */ '4|4',
        { ...defaultSearchRequest, limit: 3, offset: 9 },
      );
    });
  };

  describe('top pagination-sk', () => {
    testPaginationSk(() => topPaginationSkPO);
  });

  describe('bottom pagination-sk', () => {
    testPaginationSk(() => bottomPaginationSkPO);
  });

  describe('search results', () => {
    it('shows empty search results', async () => {
      await instantiate({ initialSearchResponse: emptySearchResponse });

      expect(await searchPageSkPO.getSummary())
        .to.equal('No results matched your search criteria.');
      expect(await searchPageSkPO.getDigests()).to.be.empty;
      expect(await topPaginationSkPO.isEmpty()).to.be.true;
      expect(await bottomPaginationSkPO.isEmpty()).to.be.true;
    });

    it('shows search results', async () => {
      await instantiate();

      expect(await searchPageSkPO.getSummary()).to.equal('Showing results 1 to 3 (out of 85).');
      expect(await searchPageSkPO.getDigests()).to.deep.equal([
        'Left: fbd3de3fff6b852ae0bb6751b9763d27',
        'Left: 2fa58aa430e9c815755624ca6cca4a72',
        'Left: ed4a8cf9ea9fbb57bf1f302537e07572',
      ]);
      expect(await topPaginationSkPO.getCurrentPage()).to.equal(1);
      expect(await bottomPaginationSkPO.getCurrentPage()).to.equal(1);
    });

    it('shows search results with changelist information', async () => {
      await instantiate(instantiationOptionsWithCL);

      expect(await searchPageSkPO.getDigests()).to.deep.equal([
        'Left: fbd3de3fff6b852ae0bb6751b9763d27',
        'Left: 2fa58aa430e9c815755624ca6cca4a72',
        'Left: ed4a8cf9ea9fbb57bf1f302537e07572',
      ]);
      expect(await topPaginationSkPO.getCurrentPage()).to.equal(1);
      expect(await bottomPaginationSkPO.getCurrentPage()).to.equal(1);

      const diffDetailsHrefs = await searchPageSkPO.getDiffDetailsHrefs();
      expect(diffDetailsHrefs[0]).to.contain('changelist_id=123456&crs=gerrit');
      expect(diffDetailsHrefs[1]).to.contain('changelist_id=123456&crs=gerrit');
      expect(diffDetailsHrefs[2]).to.contain('changelist_id=123456&crs=gerrit');
    });

    describe('triaging a single digest', () => {
      // These test cases exercise the wiring code that passes the groupings returned by the
      // /json/v1/groupings RPC to the digest-details-sk elements.

      const triageRequest: TriageRequestV3 = {
        deltas: [
          {
            grouping: {
              source_type: 'infra',
              name: 'gold_search-controls-sk_right-hand-trace-filter-editor',
            },
            digest: 'fbd3de3fff6b852ae0bb6751b9763d27',
            label_before: 'positive',
            label_after: 'negative',
          },
        ],
      };
      const triageResponse: TriageResponse = { status: 'ok' };

      it('can triage at head', async () => {
        await instantiate();
        fetchMock.post(
          { url: '/json/v3/triage', body: triageRequest },
          { status: 200, body: triageResponse },
        );

        const digest = await searchPageSkPO.digestDetailsSkPOs.item(0);
        await digest.triageSkPO.clickButton('negative');
      });

      it('can triage at a CL', async () => {
        await instantiate(instantiationOptionsWithCL);
        const triageRequestForCL: TriageRequestV3 = {
          ...triageRequest,
          changelist_id: '123456',
          crs: 'gerrit',
        };
        fetchMock.post(
          { url: '/json/v3/triage', body: triageRequestForCL },
          { status: 200, body: triageResponse },
        );

        const digest = await searchPageSkPO.digestDetailsSkPOs.item(0);
        await digest.triageSkPO.clickButton('negative');
      });
    });
  });

  // TODO(lovisolo): Add some sort of indication in the UI when searching by blame.
  describe('"blame" URL parameter', () => {
    it('is reflected in the initial search RPC', async () => {
      // No explicit assertions are necessary because if the search RPC is not called with the
      // expected SearchRequest then the fetchMock.done() call in the top-level afterEach() hook
      // will fail.
      await instantiate({
        initialQueryString:
          '?blame=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
        expectedInitialSearchRequest: {
          ...defaultSearchRequest,
          blame: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
        },
      });
    });
  });

  describe('help dialog', () => {
    it('is closed by default', async () => {
      await instantiate();
      expect(await searchPageSkPO.isHelpDialogOpen()).to.be.false;
    });

    it('opens when clicking the "Help" button', async () => {
      await instantiate();
      await searchPageSkPO.clickHelpBtn();
      expect(await searchPageSkPO.isHelpDialogOpen()).to.be.true;
    });

    it('closes when the "Close" button is clicked', async () => {
      await instantiate();
      await searchPageSkPO.clickHelpBtn();
      await searchPageSkPO.clickHelpDialogCancelBtn();
      expect(await searchPageSkPO.isHelpDialogOpen()).to.be.false;
    });
  });

  describe('bulk triage dialog', () => {
    describe('opening and closing', () => {
      it('is closed by default', async () => {
        await instantiate();
        expect(await searchPageSkPO.isBulkTriageDialogOpen()).to.be.false;
      });

      it('opens when clicking the "Bulk Triage" button', async () => {
        await instantiate();
        await searchPageSkPO.clickBulkTriageBtn();
        expect(await searchPageSkPO.isBulkTriageDialogOpen()).to.be.true;
      });

      it('closes when the "Cancel" button is clicked', async () => {
        await instantiate();
        await searchPageSkPO.clickBulkTriageBtn();
        await bulkTriageSkPO.clickCancelBtn();
        expect(await searchPageSkPO.isBulkTriageDialogOpen()).to.be.false;
      });

      it('closes when the "Triage ..." button is clicked', async () => {
        fetchMock.post('/json/v3/triage', 200); // We ignore the TriageRequest in this test.

        await instantiate();
        await searchPageSkPO.clickBulkTriageBtn();
        await bulkTriageSkPO.clickTriageBtn();
        expect(await searchPageSkPO.isBulkTriageDialogOpen()).to.be.false;
      });
    });

    describe('affected CL', () => {
      it('does not show an affected CL if none is provided', async () => {
        await instantiate();
        await searchPageSkPO.clickBulkTriageBtn();
        expect(await bulkTriageSkPO.isAffectedChangelistIdVisible()).to.be.false;
      });

      it('shows the affected CL if one is provided', async () => {
        await instantiate(instantiationOptionsWithCL);
        await searchPageSkPO.clickBulkTriageBtn();
        expect(await bulkTriageSkPO.isAffectedChangelistIdVisible()).to.be.true;
        expect(await bulkTriageSkPO.getAffectedChangelistId()).to.equal(
          'This affects Changelist 123456.',
        );
      });
    });

    describe('RPCs', () => {
      describe('search results from current page only', () => {
        const expectedTriageRequest: TriageRequestV3 = {
          deltas: [
            {
              grouping: {
                source_type: 'infra',
                name: 'gold_search-controls-sk_right-hand-trace-filter-editor',
              },
              digest: 'fbd3de3fff6b852ae0bb6751b9763d27',
              label_before: 'positive',
              label_after: 'positive',
            },
            {
              grouping: {
                source_type: 'infra',
                name: 'perf_alert-config-sk',
              },
              digest: '2fa58aa430e9c815755624ca6cca4a72',
              label_before: 'negative',
              label_after: 'positive',
            },
            {
              grouping: {
                source_type: 'infra',
                name: 'perf_alert-config-sk',
              },
              digest: 'ed4a8cf9ea9fbb57bf1f302537e07572',
              label_before: 'untriaged',
              label_after: 'positive',
            },
          ],
        };

        it('can bulk-triage without a CL', async () => {
          fetchMock.post('/json/v3/triage', 200, { body: expectedTriageRequest });

          await instantiate();
          await searchPageSkPO.clickBulkTriageBtn();
          await bulkTriageSkPO.clickPositiveBtn();
          await bulkTriageSkPO.clickTriageBtn();
        });

        it('can bulk-triage with a CL', async () => {
          fetchMock.post('/json/v3/triage', 200, {
            body: {
              ...expectedTriageRequest,
              changelist_id: '123456',
              crs: 'gerrit',
            },
          });

          await instantiate(instantiationOptionsWithCL);
          await searchPageSkPO.clickBulkTriageBtn();
          await bulkTriageSkPO.clickPositiveBtn();
          await bulkTriageSkPO.clickTriageBtn();
        });
      });

      describe('all search results', () => {
        const expectedTriageRequest: TriageRequestV3 = {
          deltas: [
            {
              grouping: {
                source_type: 'infra',
                name: 'gold_details-page-sk',
              },
              digest: '29f31f703510c2091840b5cf2b032f56',
              label_before: 'positive',
              label_after: 'positive',
            },
            {
              grouping: {
                source_type: 'infra',
                name: 'gold_details-page-sk',
              },
              digest: '7c0a393e57f14b5372ec1590b79bed0f',
              label_before: 'positive',
              label_after: 'positive',
            },
            {
              grouping: {
                source_type: 'infra',
                name: 'gold_details-page-sk',
              },
              digest: '971fe90fa07ebc2c7d0c1a109a0f697c',
              label_before: 'positive',
              label_after: 'positive',
            },
            {
              grouping: {
                source_type: 'infra',
                name: 'gold_details-page-sk',
              },
              digest: 'e49c92a2cff48531810cc5e863fad0ee',
              label_before: 'positive',
              label_after: 'positive',
            },
            {
              grouping: {
                source_type: 'infra',
                name: 'gold_search-controls-sk_right-hand-trace-filter-editor',
              },
              digest: '5d8c80eda80e015d633a4125ab0232dc',
              label_before: 'positive',
              label_after: 'positive',
            },
            {
              grouping: {
                source_type: 'infra',
                name: 'gold_search-controls-sk_right-hand-trace-filter-editor',
              },
              digest: 'd20f37006e436fe17f50ecf49ff2bdb5',
              label_before: 'positive',
              label_after: 'positive',
            },
            {
              grouping: {
                source_type: 'infra',
                name: 'gold_search-controls-sk_right-hand-trace-filter-editor',
              },
              digest: 'fbd3de3fff6b852ae0bb6751b9763d27',
              label_before: 'positive',
              label_after: 'positive',
            },
            {
              grouping: {
                source_type: 'infra',
                name: 'perf_alert-config-sk',
              },
              digest: '2fa58aa430e9c815755624ca6cca4a72',
              label_before: 'negative',
              label_after: 'positive',
            },
            {
              grouping: {
                source_type: 'infra',
                name: 'perf_alert-config-sk',
              },
              digest: 'ed4a8cf9ea9fbb57bf1f302537e07572',
              label_before: 'untriaged',
              label_after: 'positive',
            },
          ],
        };

        it('can bulk-triage without a CL', async () => {
          fetchMock.post('/json/v3/triage', 200, { body: expectedTriageRequest });

          await instantiate();
          await searchPageSkPO.clickBulkTriageBtn();
          await bulkTriageSkPO.clickTriageAllCheckbox();
          await bulkTriageSkPO.clickPositiveBtn();
          await bulkTriageSkPO.clickTriageBtn();
        });

        it('can bulk-triage with a CL', async () => {
          fetchMock.post('/json/v3/triage', 200, {
            body: {
              ...expectedTriageRequest,
              changelist_id: '123456',
              crs: 'gerrit',
            },
          });

          await instantiate(instantiationOptionsWithCL);
          await searchPageSkPO.clickBulkTriageBtn();
          await bulkTriageSkPO.clickTriageAllCheckbox();
          await bulkTriageSkPO.clickPositiveBtn();
          await bulkTriageSkPO.clickTriageBtn();
        });
      });
    });
  });

  describe('keyboard shortcuts', () => {
    // TODO(lovisolo): Clean this up after digest-details-sk is ported to TypeScript and we have
    //                 a DigestDetailsSkPO.
    const firstDigest = 'Left: fbd3de3fff6b852ae0bb6751b9763d27';
    const secondDigest = 'Left: 2fa58aa430e9c815755624ca6cca4a72';
    const thirdDigest = 'Left: ed4a8cf9ea9fbb57bf1f302537e07572';

    const expectLabelsForFirstSecondAndThirdDigestsToBe = async (firstLabel: Label, secondLabel: Label, thirdLabel: Label) => {
      expect(await searchPageSkPO.getLabelForDigest(firstDigest)).to.equal(firstLabel);
      expect(await searchPageSkPO.getLabelForDigest(secondDigest)).to.equal(secondLabel);
      expect(await searchPageSkPO.getLabelForDigest(thirdDigest)).to.equal(thirdLabel);
    };

    describe('navigation', () => {
      it('initially has an empty selection', async () => {
        await instantiate();
        expect(await searchPageSkPO.getSelectedDigest()).to.be.null;
      });

      it('can navigate between digests with keys "J" and "K"', async () => {
        await instantiate();

        expect(await searchPageSkPO.getSelectedDigest()).to.be.null;

        // Forward.
        await searchPageSkPO.typeKey('j');
        expect(await searchPageSkPO.getSelectedDigest()).to.equal(firstDigest);

        // Forward.
        await searchPageSkPO.typeKey('j');
        expect(await searchPageSkPO.getSelectedDigest()).to.equal(secondDigest);

        // Forward.
        await searchPageSkPO.typeKey('j');
        expect(await searchPageSkPO.getSelectedDigest()).to.equal(thirdDigest);

        // Forward. Nothing happens because we're at the last search result.
        await searchPageSkPO.typeKey('j');
        expect(await searchPageSkPO.getSelectedDigest()).to.equal(thirdDigest);

        // Back.
        await searchPageSkPO.typeKey('k');
        expect(await searchPageSkPO.getSelectedDigest()).to.equal(secondDigest);

        // Back.
        await searchPageSkPO.typeKey('k');
        expect(await searchPageSkPO.getSelectedDigest()).to.equal(firstDigest);

        // Back. Nothing happens because we're at the first search result.
        await searchPageSkPO.typeKey('k');
        expect(await searchPageSkPO.getSelectedDigest()).to.equal(firstDigest);
      });

      it('resets the selection when the search results change', async () => {
        await instantiate();

        // Select the first search result.
        await searchPageSkPO.typeKey('j');

        // Refresh the results by changing a search parameter.
        fetchMock.get('glob:/json/v2/search?*', searchResponse);
        const event = eventPromise('end-task');
        await searchControlsSkPO.clickIncludePositiveDigestsCheckbox();
        await event;

        // Search results should be non-empty, but selection should be empty.
        expect(await searchPageSkPO.getDigests()).to.not.be.empty;
        expect(await searchPageSkPO.getSelectedDigest()).to.be.null;
      });
    });

    describe('triaging', () => {
      it('cannot triage with "A", "S" and "D" keys when the selection is empty', async () => {
        await instantiate();

        // Check initial labels.
        await expectLabelsForFirstSecondAndThirdDigestsToBe('positive', 'negative', 'untriaged');

        // Triaging as positive should have no effect.
        await searchPageSkPO.typeKey('a');
        await expectLabelsForFirstSecondAndThirdDigestsToBe('positive', 'negative', 'untriaged');

        // Triaging as negative should have no effect.
        await searchPageSkPO.typeKey('s');
        await expectLabelsForFirstSecondAndThirdDigestsToBe('positive', 'negative', 'untriaged');

        // Triaging as untriaged should have no effect.
        await searchPageSkPO.typeKey('d');
        await expectLabelsForFirstSecondAndThirdDigestsToBe('positive', 'negative', 'untriaged');
      });

      it('can triage the selected digest with keys "A", "S" and "D"', async () => {
        // We ignore the TriageRequest in this test.
        const triageResponse: TriageResponse = { status: 'ok' };
        fetchMock.post('/json/v3/triage', { status: 200, body: triageResponse });

        await instantiate();

        // Check initial labels.
        await expectLabelsForFirstSecondAndThirdDigestsToBe('positive', 'negative', 'untriaged');

        // Select the second search result.
        await searchPageSkPO.typeKey('j');
        await searchPageSkPO.typeKey('j');

        // We will also test that, when the user triages a digest, the new label remains in place
        // even after the search-page-sk component is re-rendered with the same (now stale)
        // cached SearchResults from an earlier RPC to /json/v2/search. The SearchResults are now
        // stale because they reflect the RPC response prior to the user's triage action.
        //
        // This behavior is important to test because it exercises logic in search-page-sk that
        // patches the cached SearchResults with a new label when the user triages a digest via the
        // digest-details-sk component, or via the "A", "S" or "D" keyboard shortcuts.
        //
        // Currently there are no situations that would cause the search page to be re-rendered with
        // cached SearchResults. It used to be the case that navigating between search results with
        // the "J" and "K" keyboard shortcuts would trigger a re-render in order to redraw the box
        // around the selected search result. However, this turned out to be slow for pages with
        // many search results. So this is now done in an ad-hoc way by manually updating the
        // affected DOM nodes, which is much faster than calling lit-html's render() function.
        //
        // We still want to test this behavior in case we decide to revert the above optimization
        // (e.g. if we add pagination, therefore limiting the number of results displayed at once),
        // or if we decide to implement new features that might require re-rendering the page with
        // cached SearchResults. We exercise this behavior by forcing a page re-render via the
        // _render() method.
        //
        // We test this behavior here via keyboard shortcuts for convenience.

        // Triage as positive.
        let event = eventPromise('end-task');
        await searchPageSkPO.typeKey('a');
        await event;

        // It should be positive, and the label should stick after the page is re-rendered.
        await expectLabelsForFirstSecondAndThirdDigestsToBe('positive', 'positive', 'untriaged');
        (searchPageSk as any)._render(); // We cast to "any" because _render is not public.
        await expectLabelsForFirstSecondAndThirdDigestsToBe('positive', 'positive', 'untriaged');

        // Triage as negative.
        event = eventPromise('end-task');
        await searchPageSkPO.typeKey('s');
        await event;

        // It should be negative, and the label should stick after the page is re-rendered.
        await expectLabelsForFirstSecondAndThirdDigestsToBe('positive', 'negative', 'untriaged');
        (searchPageSk as any)._render(); // We cast to "any" because _render is not public.
        await expectLabelsForFirstSecondAndThirdDigestsToBe('positive', 'negative', 'untriaged');

        // Triage as untriaged.
        event = eventPromise('end-task');
        await searchPageSkPO.typeKey('d');
        await event;

        // It should be untriaged, and the label should stick after the page is re-rendered.
        await expectLabelsForFirstSecondAndThirdDigestsToBe('positive', 'untriaged', 'untriaged');
        (searchPageSk as any)._render(); // We cast to "any" because _render is not public.
        await expectLabelsForFirstSecondAndThirdDigestsToBe('positive', 'untriaged', 'untriaged');
      });
    });

    describe('zoom', () => {
      it('cannot zoom with the "W" key when the selection is empty', async () => {
        await instantiate();

        // Check that there is no open zoom dialog.
        expect(await searchPageSkPO.getDigestWithOpenZoomDialog()).to.be.null;

        // The keyboard shortcut should have no effect as no digest is selected.
        await searchPageSkPO.typeKey('w');
        expect(await searchPageSkPO.getDigestWithOpenZoomDialog()).to.be.null;
      });

      it('can zoom into the selected digest with the "W" key', async () => {
        await instantiate();

        // Select the second search result.
        await searchPageSkPO.typeKey('j');
        await searchPageSkPO.typeKey('j');

        // The zoom dialog for the second search result should open.
        await searchPageSkPO.typeKey('w');
        expect(await searchPageSkPO.getDigestWithOpenZoomDialog()).to.equal(secondDigest);
      });
    });

    it('shows the help dialog when pressing the "?" key', async () => {
      await instantiate();
      await searchPageSkPO.typeKey('?');
      expect(await searchPageSkPO.isHelpDialogOpen()).to.be.true;
    });

    describe('shortcuts are disabled when a dialog is open', () => {
      beforeEach(async () => {
        await instantiate();

        // Select the second search result. The expectKeyboardShortcutsToBeDisabled() helper below
        // relies on this.
        await searchPageSkPO.typeKey('j');
        await searchPageSkPO.typeKey('j');
      });

      const expectKeyboardShortcutsToBeDisabled = async () => {
        // Navigation shortcuts should have no effect.
        expect(await searchPageSkPO.getSelectedDigest()).to.equal(secondDigest);
        await searchPageSkPO.typeKey('j');
        expect(await searchPageSkPO.getSelectedDigest()).to.equal(secondDigest);
        await searchPageSkPO.typeKey('k');
        expect(await searchPageSkPO.getSelectedDigest()).to.equal(secondDigest);

        // Check initial triage labels.
        await expectLabelsForFirstSecondAndThirdDigestsToBe('positive', 'negative', 'untriaged');

        // Shortcut for triaging as positive should have no effect.
        let noEvent = noEventPromise('begin-task');
        await searchPageSkPO.typeKey('a');
        await noEvent;
        await expectLabelsForFirstSecondAndThirdDigestsToBe('positive', 'negative', 'untriaged');

        // Shortcut for triaging as negative should have no effect.
        noEvent = noEventPromise('begin-task');
        await searchPageSkPO.typeKey('s');
        await noEvent;
        await expectLabelsForFirstSecondAndThirdDigestsToBe('positive', 'negative', 'untriaged');

        // Shortcut for triaging as untriagaed should have no effect.
        noEvent = noEventPromise('begin-task');
        await searchPageSkPO.typeKey('d');
        await noEvent;
        await expectLabelsForFirstSecondAndThirdDigestsToBe('positive', 'negative', 'untriaged');

        // Shortcut for the help dialog should have no effect, but we can only test this if the
        // help dialog is not already open, otherwise the shortcut has no effect.
        if (!(await searchPageSkPO.isHelpDialogOpen())) {
          await searchPageSkPO.typeKey('?');
          expect(await searchPageSkPO.isHelpDialogOpen()).to.be.false;
        }
      };

      it('disables keyboard shortcuts when the help dialog is open', async () => {
        await searchPageSkPO.clickHelpBtn(); // Open help dialog.

        expect(await searchPageSkPO.isHelpDialogOpen()).to.be.true;
        await expectKeyboardShortcutsToBeDisabled();
      });

      it('disables keyboard shortcuts when the bulk triage dialog is open', async () => {
        await searchPageSkPO.clickBulkTriageBtn(); // Open bulk triage dialog.

        expect(await searchPageSkPO.isBulkTriageDialogOpen()).to.be.true;
        await expectKeyboardShortcutsToBeDisabled();
      });

      it('disables keyboard shortcuts when the left-hand trace filter dialog is open', async () => {
        const leftHandTraceFilterSkPO = await searchControlsSkPO.traceFilterSkPO;
        await leftHandTraceFilterSkPO.clickEditBtn(); // Open left-hand trace filter dialog.

        expect(await leftHandTraceFilterSkPO.isQueryDialogSkOpen()).to.be.true;
        await expectKeyboardShortcutsToBeDisabled();
      });

      it('disables keyboard shortcuts when the more filters dialog is open', async () => {
        const filterDialogSkPO = await searchControlsSkPO.filterDialogSkPO;
        await searchControlsSkPO.clickMoreFiltersBtn(); // Open more filters dialog.

        expect(await filterDialogSkPO.isDialogOpen()).to.be.true;
        await expectKeyboardShortcutsToBeDisabled();
      });

      it(
        'disables keyboard shortcuts when the right-hand trace filter dialog is open',
        async () => {
          const filterDialogSkPO = await searchControlsSkPO.filterDialogSkPO;
          await searchControlsSkPO.clickMoreFiltersBtn(); // Open more filters dialog.

          const rightHandTraceFilterSkPO = await filterDialogSkPO.traceFilterSkPO;
          await rightHandTraceFilterSkPO.clickEditBtn(); // Open right-hand trace filter dialog.

          expect(await filterDialogSkPO.isDialogOpen()).to.be.true;
          expect(await rightHandTraceFilterSkPO.isQueryDialogSkOpen()).to.be.true;
          await expectKeyboardShortcutsToBeDisabled();
        },
      );

      it('disables keyboard shortcuts when the zoom dialog is open', async () => {
        await searchPageSkPO.typeKey('w'); // Open zoom dialog.

        expect(await searchPageSkPO.getDigestWithOpenZoomDialog()).to.not.be.null;
        await expectKeyboardShortcutsToBeDisabled();
      });
    });
  });
});
