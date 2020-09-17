import './index';
import { setUpElementUnderTest, eventSequencePromise, eventPromise, setQueryString, expectQueryStringToEqual } from '../../../infra-sk/modules/test_util';
import { searchResponse, statusResponse, paramSetResponse, changeListSummaryResponse } from './demo_data';
import fetchMock from 'fetch-mock';
import { deepCopy } from 'common-sk/modules/object';
import { fromObject } from 'common-sk/modules/query';
import { SearchPageSk, SearchRequest, defaultSearchCriteria } from './search-page-sk';
import { SearchPageSkPO } from './search-page-sk_po';
import { SearchResponse } from '../rpc_types';
import { testOnlySetSettings } from '../settings';
import { SearchCriteria } from '../search-controls-sk/search-controls-sk';

const expect = chai.expect;

describe('search-page-sk', () => {
  const newInstance = setUpElementUnderTest<SearchPageSk>('search-page-sk');

  let searchPageSk: SearchPageSk;
  let searchPageSkPO: SearchPageSkPO;

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
    queryString: string;
    expectedSearchRequest: SearchRequest;
    searchResponse: SearchResponse;
    mockAndWaitForChangelistSummaryRPC: boolean;
  };

  // Instantiates the search page, sets up the necessary mock RPCs and waits for it to load.
  const instantiate = async (opts: Partial<InstantiationOptions> = {}) => {
    const defaults: InstantiationOptions = {
      queryString: '',
      expectedSearchRequest: defaultSearchRequest,
      searchResponse: searchResponse,
      mockAndWaitForChangelistSummaryRPC: false,
    };

    // Override defaults with the given options, if any.
    opts = {...defaults, ...opts};

    setQueryString(opts.queryString!);

    fetchMock.get('/json/v1/trstatus', () => statusResponse);
    fetchMock.get('/json/v1/paramset', () => paramSetResponse);
    fetchMock.get(
      '/json/v1/search?' + fromObject(opts.expectedSearchRequest as any),
      () => opts.searchResponse);

    // We always wait for at least the three above RPCs.
    const eventsToWaitFor = ['end-task', 'end-task', 'end-task'];

    // This mocked RPC corresponds to the queryStringWithCL and searchRequestWithCL constants
    // defined above.
    if (opts.mockAndWaitForChangelistSummaryRPC) {
      fetchMock.get('/json/v1/changelist/gerrit/123456', () => changeListSummaryResponse);
      eventsToWaitFor.push('end-task');
    }

    const events = eventSequencePromise(eventsToWaitFor);
    searchPageSk = newInstance();
    searchPageSkPO = new SearchPageSkPO(searchPageSk);
    await events;
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

  describe('search results', () => {
    it('shows empty search results', async () => {
      await instantiate({searchResponse: emptySearchResponse});

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
      await instantiate({
        queryString: queryStringWithCL,
        expectedSearchRequest: searchRequestWithCL,
        mockAndWaitForChangelistSummaryRPC: true,
      });

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

  describe('search controls', () => {

    // This function takes a search-controls-sk field (in the form of a partial, single-field
    // SearchCriteria object) and adds tests to ensure that the field is correctly linked to its
    // corresponding URL query parameter and its corresponding field in the SearchRequest object.
    const fieldCanBeReadInFromURLAndSetViaUI = (
      queryString: string,
      partialSearchCriteria: Partial<SearchCriteria>,
      partialSearchRequest: Partial<SearchRequest>,
    ) => {
      let searchCriteria: SearchCriteria;
      let searchRequest: SearchRequest;

      beforeEach(() => {
        // We wrap this code inside a beforeEach() hook to ensure that it is always executed after
        // the top-level before() hook, where we call testOnlySetSettings(). This is necessary
        // because defaultSearchCriteria() reads the default corpus.

        // Set any missing fields to their default values.
        searchCriteria = {...defaultSearchCriteria(), ...partialSearchCriteria};
        searchRequest = {...defaultSearchRequest, ...partialSearchRequest};

        // We want the expected objects to be different from their default values, otherwise the
        // tests will be pointless.
        expect(searchCriteria).to.not.deep.equal(defaultSearchCriteria());
        expect(searchRequest).to.not.deep.equal(defaultSearchRequest);
      });

      it('can be read from the URL', async () => {
        // Set the query string to the given value and initialize the page. This should cause the
        // field under test to be read from the URL and reflected in the search-controls-sk.
        //
        // The field under test should also be reflected in the initial search RPC. If said RPC is
        // not called with the expected SearchRequest, the top-level afterEach() hook will fail.
        await instantiate({queryString: queryString, expectedSearchRequest: searchRequest});

        // Get the actual SearchCriteria displayed in the UI.
        const searchControlsSkPO = await searchPageSkPO.getSearchControlsSkPO();
        const actualSearchCriteria = await searchControlsSkPO.getSearchCriteria();

        // The UI should show the expected SearchCriteria.
        expect(actualSearchCriteria).to.deep.equal(searchCriteria);
      });

      it('can be set via the UI', async () => {
        // Instantiate the search page with an empy query string.
        await instantiate();
        expectQueryStringToEqual('');

        // Mock the search RPC we expect to take when we change the field under test via the UI.
        // The top-level afterEach() hook will fail if this RPC is not called.
        fetchMock.get('/json/v1/search?' + fromObject(searchRequest as any), () => searchResponse);

        const searchControlsSkPO = await searchPageSkPO.getSearchControlsSkPO();

        // Set the field under test via the UI and wait for the RPC to complete.
        const event = eventPromise('end-task');
        await searchControlsSkPO.setSearchCriteria(searchCriteria);
        await event;

        // The URL should have been updated with the new search criteria.
        expectQueryStringToEqual(queryString);
      });
    }

    describe('corpus', () => {
      fieldCanBeReadInFromURLAndSetViaUI(
        '?corpus=my-corpus',
        {corpus: 'my-corpus'},
        {
          query: 'source_type=my-corpus',
          rquery: 'source_type=my-corpus',
        });
    });

    describe('left-hand trace filter', () => {
      fieldCanBeReadInFromURLAndSetViaUI(
        '?left_filter=name%3Dam_email-chooser-sk',
        {leftHandTraceFilter: {'name': ['am_email-chooser-sk']}},
        {query: 'name=am_email-chooser-sk&source_type=infra'});
    });

    describe('right-hand trace filter', () => {
      fieldCanBeReadInFromURLAndSetViaUI(
        '?right_filter=name%3Dam_email-chooser-sk',
        {rightHandTraceFilter: {'name': ['am_email-chooser-sk']}},
        {rquery: 'name=am_email-chooser-sk&source_type=infra'});
    });

    describe('include positive digests', () => {
      fieldCanBeReadInFromURLAndSetViaUI(
        '?positive=true',
        {includePositiveDigests: true},
        {pos: true});
    });

    describe('include negative digests', () => {
      fieldCanBeReadInFromURLAndSetViaUI(
        '?negative=true',
        {includeNegativeDigests: true},
        {neg: true});
    });

    describe('include untriaged digests', () => {
      // This field is true by default, so we set it to false.
      fieldCanBeReadInFromURLAndSetViaUI(
        '?untriaged=false',
        {includeUntriagedDigests: false},
        {unt: false});
    });

    describe('include digests not at head', () => {
      fieldCanBeReadInFromURLAndSetViaUI(
        '?not_at_head=true',
        {includeDigestsNotAtHead: true},
        {head: false}); // SearchRequest field "head" means "at head only".
    });

    describe('include ignored digests', () => {
      fieldCanBeReadInFromURLAndSetViaUI(
        '?include_ignored=true',
        {includeIgnoredDigests: true},
        {include: true});
    });

    describe('min RGBA delta', () => {
      fieldCanBeReadInFromURLAndSetViaUI(
        '?min_rgba=10',
        {minRGBADelta: 10},
        {frgbamin: 10});
    });

    describe('max RGBA delta', () => {
      fieldCanBeReadInFromURLAndSetViaUI(
        '?max_rgba=200',
        {maxRGBADelta: 200},
        {frgbamax: 200});
    });

    describe('max RGBA delta', () => {
      fieldCanBeReadInFromURLAndSetViaUI(
        '?max_rgba=200',
        {maxRGBADelta: 200},
        {frgbamax: 200});
    });

    describe('must have reference image', () => {
      fieldCanBeReadInFromURLAndSetViaUI(
        '?reference_image_required=true',
        {mustHaveReferenceImage: true},
        {fref: true});
    });

    describe('sort order', () => {
      fieldCanBeReadInFromURLAndSetViaUI(
        '?sort=ascending',
        {sortOrder: 'ascending'},
        {sort: 'asc'});
    });
  });

  describe('changelist controls', () => {
    it('is hidden if no CL is provided in the query string', async () => {
      await instantiate();

      const changelistControlsSkPO = await searchPageSkPO.getChangelistControlsSkPO();
      expect(await changelistControlsSkPO.isVisible()).to.be.false;
    });

    it('is visible if CL and CRS are provided in the query string', async () => {
      await instantiate({
        queryString: queryStringWithCL,
        expectedSearchRequest: searchRequestWithCL,
        mockAndWaitForChangelistSummaryRPC: true,
      });

      const changelistControlsSkPO = await searchPageSkPO.getChangelistControlsSkPO();
      expect(await changelistControlsSkPO.isVisible()).to.be.true;
    });

    // TODO(lovisolo): Test this more thoroughly (exercise all search parameters, etc.).

    it('updates the search results when the changelist controls change', async () => {
      await instantiate({
        queryString: queryStringWithCL,
        expectedSearchRequest: searchRequestWithCL,
        mockAndWaitForChangelistSummaryRPC: true,
      });

      // We will pretend that the user clicked on the "Show all results" radio.
      const searchRequest = deepCopy(defaultSearchRequest);
      searchRequest.crs = 'gerrit';
      searchRequest.issue = '123456';
      searchRequest.master = true;
      searchRequest.patchsets = 2;
      fetchMock.get(
        '/json/v1/search?' + fromObject(searchRequest as any), () => emptySearchResponse);

      const event = eventPromise('end-task');
      const changelistControlsSkPO = await searchPageSkPO.getChangelistControlsSkPO();
      await changelistControlsSkPO.clickShowAllResultsRadio();
      await event;

      // TODO(lovisolo): Fake a single-digest search response, then assert that it's displayed.
    });
  });

  describe('search RPC reflects any optional URL query parameters', () => {
    // The test cases below assert that the search page's optional URL parameters (blame, crs,
    // issue) are reflected in the /json/v1/search RPC.
    //
    // No explicit asserts are necessary because if the search RPC is not called with the expected
    // SearchRequest then the fetchMock.done() call in the top-level afterEach() hook will fail.

    it('reflects the "blame" URL parameter', async () => {
      const searchRequest = deepCopy(defaultSearchRequest);
      searchRequest.blame = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb';

      await instantiate({
        queryString: '?blame=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
        expectedSearchRequest: searchRequest,
      });
    });

    it('reflects the "crs" URL parameter', async () => {
      const searchRequest = deepCopy(defaultSearchRequest);
      searchRequest.crs = 'gerrit';

      await instantiate({
        queryString: '?crs=gerrit',
        expectedSearchRequest: searchRequest,
      });
    });

    it('reflects the "issue" URL parameter', async () => {
      const searchRequest = deepCopy(defaultSearchRequest);
      searchRequest.issue = '123456';

      await instantiate({
        queryString: '?issue=123456',
        expectedSearchRequest: searchRequest,
      });
    });

    it('reflects the "master" URL parameter', async () => {
      const searchRequest = deepCopy(defaultSearchRequest);
      searchRequest.master = true;

      await instantiate({
        queryString: '?master=true',
        expectedSearchRequest: searchRequest,
      });
    });

    it('reflects the "patchsets" URL parameter', async () => {
      const searchRequest = deepCopy(defaultSearchRequest);
      searchRequest.patchsets = 1;

      await instantiate({
        queryString: '?patchsets=1',
        expectedSearchRequest: searchRequest,
      });
    });
  });
});
