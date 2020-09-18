import './index';
import { setUpElementUnderTest, eventSequencePromise, eventPromise, setQueryString, expectQueryStringToEqual } from '../../../infra-sk/modules/test_util';
import { searchResponse, statusResponse, paramSetResponse, changeListSummaryResponse } from './demo_data';
import fetchMock from 'fetch-mock';
import { deepCopy } from 'common-sk/modules/object';
import { fromObject } from 'common-sk/modules/query';
import { SearchPageSk, SearchRequest } from './search-page-sk';
import { SearchPageSkPO } from './search-page-sk_po';
import { SearchResponse } from '../rpc_types';
import { testOnlySetSettings } from '../settings';
import { SearchCriteria } from '../search-controls-sk/search-controls-sk';
import { SearchControlsSkPO } from '../search-controls-sk/search-controls-sk_po';
import { ChangelistControlsSkPO } from '../changelist-controls-sk/changelist-controls-sk_po';

const expect = chai.expect;

describe('search-page-sk', () => {
  const newInstance = setUpElementUnderTest<SearchPageSk>('search-page-sk');

  let searchPageSk: SearchPageSk;
  let searchPageSkPO: SearchPageSkPO;
  let searchControlsSkPO: SearchControlsSkPO;
  let changelistControlsSkPO: ChangelistControlsSkPO;

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
    sortOrder: 'descending'
  };

  // Default request to the /json/v1/search RPC when the page is loaded with an empty query string.
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
  }

  // Query string that will produce the searchRequestWithCL defined below upon page load.
  const queryStringWithCL = '?crs=gerrit&issue=123456';

  // Request to the /json/v1/search RPC when URL parameters crs=gerrit and issue=123456 are present.
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
  };

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
    opts = {...defaults, ...opts};

    fetchMock.get('/json/v1/trstatus', () => statusResponse);
    fetchMock.get('/json/v1/paramset', () => paramSetResponse);
    fetchMock.get(
      '/json/v1/search?' + fromObject(opts.expectedInitialSearchRequest as any),
      () => opts.initialSearchResponse);

    // We always wait for at least the three above RPCs.
    const eventsToWaitFor = ['end-task', 'end-task', 'end-task'];

    // This mocked RPC corresponds to the queryStringWithCL and searchRequestWithCL constants
    // defined above.
    if (opts.mockAndWaitForChangelistSummaryRPC) {
      fetchMock.get('/json/v1/changelist/gerrit/123456', () => changeListSummaryResponse);
      eventsToWaitFor.push('end-task');
    }

    // The search page will derive its initial search RPC from the query parameters in the URL.
    setQueryString(opts.initialQueryString!);

    // Instantiate search page and wait for all of the above mocked RPCs to complete.
    const events = eventSequencePromise(eventsToWaitFor);
    searchPageSk = newInstance();
    await events;

    searchPageSkPO = new SearchPageSkPO(searchPageSk);
    searchControlsSkPO = await searchPageSkPO.getSearchControlsSkPO();
    changelistControlsSkPO = await searchPageSkPO.getChangelistControlsSkPO();
  }

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
    uiValueGetterFn: () => Promise<T>,
    uiValueSetterFn: () => Promise<void>,
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
        expectedInitialSearchRequest: expectedSearchRequest
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
        '/json/v1/search?' + fromObject(expectedSearchRequest as any), () => searchResponse);

      // Set the search field under test via the UI and wait for the above RPC to complete.
      const event = eventPromise('end-task');
      await uiValueSetterFn();
      await event;

      // The search field under test should now be reflected in the URL.
      expectQueryStringToEqual(queryStringWithSearchField);
    });
  }

  describe('search-controls-sk', () => {
    const itIsBoundToURLAndRPC = (
      queryString: string,
      searchCriteria: Partial<SearchCriteria>,
      serachRequest: Partial<SearchRequest>
    ) => {
      const expectedSearchCriteria: SearchCriteria = {...defaultSearchCriteria, ...searchCriteria};
      const expectedSearchRequest: SearchRequest = {...defaultSearchRequest, ...serachRequest};

      searchFieldIsBoundToURLAndRPC<SearchCriteria>(
        /* initializationOpts= */ {},
        queryString,
        () => searchControlsSkPO.getSearchCriteria(),
        () => searchControlsSkPO.setSearchCriteria(expectedSearchCriteria!),
        expectedSearchCriteria!,
        expectedSearchRequest!);
    }

    describe('field "corpus"', () => {
      itIsBoundToURLAndRPC(
        '?corpus=my-corpus',
        {corpus: 'my-corpus'},
        {query: 'source_type=my-corpus', rquery: 'source_type=my-corpus'});
    });

    describe('field "left-hand trace filter"', () => {
      itIsBoundToURLAndRPC(
        '?left_filter=name%3Dam_email-chooser-sk',
        {leftHandTraceFilter: {'name': ['am_email-chooser-sk']}},
        {query: 'name=am_email-chooser-sk&source_type=infra'});
    });

    describe('field "right-hand trace filter"', () => {
      itIsBoundToURLAndRPC(
        '?right_filter=name%3Dam_email-chooser-sk',
        {rightHandTraceFilter: {'name': ['am_email-chooser-sk']}},
        {rquery: 'name=am_email-chooser-sk&source_type=infra'});
    });

    describe('field "include positive digests"', () => {
      itIsBoundToURLAndRPC(
        '?positive=true',
        {includePositiveDigests: true},
        {pos: true});
    });

    describe('field "include negative digests"', () => {
      itIsBoundToURLAndRPC(
        '?negative=true',
        {includeNegativeDigests: true},
        {neg: true});
    });

    describe('field "include untriaged digests"', () => {
      // This field is true by default, so we set it to false.
      itIsBoundToURLAndRPC(
        '?untriaged=false',
        {includeUntriagedDigests: false},
        {unt: false});
    });

    describe('field "include digests not at head"', () => {
      itIsBoundToURLAndRPC(
        '?not_at_head=true',
        {includeDigestsNotAtHead: true},
        {head: false}); // SearchRequest field "head" means "at head only".
    });

    describe('field "include ignored digests"', () => {
      itIsBoundToURLAndRPC(
        '?include_ignored=true',
        {includeIgnoredDigests: true},
        {include: true});
    });

    describe('field "min RGBA delta"', () => {
      itIsBoundToURLAndRPC(
        '?min_rgba=10',
        {minRGBADelta: 10},
        {frgbamin: 10});
    });

    describe('field "max RGBA delta"', () => {
      itIsBoundToURLAndRPC(
        '?max_rgba=200',
        {maxRGBADelta: 200},
        {frgbamax: 200});
    });

    describe('field "max RGBA delta"', () => {
      itIsBoundToURLAndRPC(
        '?max_rgba=200',
        {maxRGBADelta: 200},
        {frgbamax: 200});
    });

    describe('field "must have reference image"', () => {
      itIsBoundToURLAndRPC(
        '?reference_image_required=true',
        {mustHaveReferenceImage: true},
        {fref: true});
    });

    describe('field "sort order"', () => {
      itIsBoundToURLAndRPC(
        '?sort=ascending',
        {sortOrder: 'ascending'},
        {sort: 'asc'});
    });
  });

  describe('changelist-controls-sk', () => {
    it('is hidden if no CL is provided in the query string', async () => {
      // When instantiated without URL parameters "crs" and "issue", the search page does not make
      // an RPC to /json/v1/changelist, therefore there is no changelist summary for the
      // changelist-controls-sk component to display.
      await instantiate();
      expect(await changelistControlsSkPO.isVisible()).to.be.false;
    });

    it(
        'is visible if a CL is provided in the query string and /json/v1/changelist returns a ' +
        'non-empty response',
        async () => {
      // We instantiate the serach page with URL parameters "crs" and "issue", which causes it to
      // make an RPC to /json/v1/changelist. The returned changelist summary is passed to the
      // changelist-controls-sk component, which then makes itself visible.
      await instantiate(instantiationOptionsWithCL);
      expect(await changelistControlsSkPO.isVisible()).to.be.true;
    });

    describe('field "patchset"', () => {
      searchFieldIsBoundToURLAndRPC<string>(
        instantiationOptionsWithCL,
        queryStringWithCL + '&patchsets=1',
        () => changelistControlsSkPO.getPatchSet(),
        () => changelistControlsSkPO.setPatchSet('PS 1'),
        /* expectedUiValue= */ 'PS 1',
        {...searchRequestWithCL, patchsets: 1});
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
          expectedInitialSearchRequest:  {...searchRequestWithCL, master: true, patchsets: 2},
          initialQueryString: queryStringWithCL + '&master=true&patchsets=2'
        },
        queryStringWithCL + '&patchsets=2',
        () => changelistControlsSkPO.isExcludeResultsFromPrimaryRadioChecked(),
        () => changelistControlsSkPO.clickExcludeResultsFromPrimaryRadio(),
        /* expectedUiValue= */ true,
        {...searchRequestWithCL, patchsets: 2});
    });

    describe('radio "show all results"', () => {
      searchFieldIsBoundToURLAndRPC<boolean>(
        instantiationOptionsWithCL,
        queryStringWithCL + '&master=true&patchsets=2',
        () => changelistControlsSkPO.isShowAllResultsRadioChecked(),
        () => changelistControlsSkPO.clickShowAllResultsRadio(),
        /* expectedUiValue= */ true,
        {...searchRequestWithCL, master: true, patchsets: 2});
    });
  });

  describe('search results', () => {
    it('shows empty search results', async () => {
      await instantiate({initialSearchResponse: emptySearchResponse});

      expect(await searchPageSkPO.getSummary())
        .to.equal('No results matched your search criteria.');
      expect(await searchPageSkPO.getDigests()).to.be.empty;
    });

    it('shows search results', async () => {
      await instantiate();

      expect(await searchPageSkPO.getSummary()).to.equal('Showing results 1 to 3 (out of 85).');
      expect(await searchPageSkPO.getDigests()).to.deep.equal([
        'Left: fbd3de3fff6b852ae0bb6751b9763d27',
        'Left: 2fa58aa430e9c815755624ca6cca4a72',
        'Left: ed4a8cf9ea9fbb57bf1f302537e07572'
      ]);
    });

    it('shows search results with changelist information', async () => {
      await instantiate(instantiationOptionsWithCL);

      expect(await searchPageSkPO.getDigests()).to.deep.equal([
        'Left: fbd3de3fff6b852ae0bb6751b9763d27',
        'Left: 2fa58aa430e9c815755624ca6cca4a72',
        'Left: ed4a8cf9ea9fbb57bf1f302537e07572'
      ]);

      const diffDetailsHrefs = await searchPageSkPO.getDiffDetailsHrefs();
      expect(diffDetailsHrefs[0]).to.contain('changelist_id=123456&crs=gerrit');
      expect(diffDetailsHrefs[1]).to.contain('changelist_id=123456&crs=gerrit');
      expect(diffDetailsHrefs[2]).to.contain('changelist_id=123456&crs=gerrit');
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
          blame: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'
        },
      });
    });
  });
});
