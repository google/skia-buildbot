import './index';
import { setUpElementUnderTest, eventSequencePromise, eventPromise, setQueryString } from '../../../infra-sk/modules/test_util';
import { searchResponse, statusResponse, paramSetResponse } from './demo_data';
import fetchMock from 'fetch-mock';
import { deepCopy } from 'common-sk/modules/object';
import { fromObject } from 'common-sk/modules/query';
import { SearchPageSk, SearchRequest } from './search-page-sk';
import { SearchPageSkPO } from './search-page-sk_po';
import { SearchResponse } from '../rpc_types';
import { testOnlySetSettings } from '../settings';

const expect = chai.expect;

describe('search-page-sk', () => {
  const newInstance = setUpElementUnderTest<SearchPageSk>('search-page-sk');

  let searchPageSk: SearchPageSk;
  let searchPageSkPO: SearchPageSkPO;

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

  const emptySearchResponse = deepCopy(searchResponse);
  emptySearchResponse.size = 0;
  emptySearchResponse.digests = [];

  // Instantiates the search page after setting up the mock search RPC with the given search query
  // and response.
  //
  // The search page hits the search RPC immediately after instantiation. This function ensures that
  // the RPC is correctly mocked before the search page is instantiated.
  const instantiate =
      async (
        initialSearchRequest: SearchRequest = defaultSearchRequest,
        initialSearchResponse: SearchResponse = searchResponse) => {
    fetchMock.get(
      '/json/v1/search?' + fromObject(initialSearchRequest as any), () => initialSearchResponse);
    const events = eventSequencePromise(['end-task', 'end-task', 'end-task']);
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

  beforeEach(() => {
    setQueryString('');
    fetchMock.get('/json/v1/trstatus', () => statusResponse);
    fetchMock.get('/json/v1/paramset', () => paramSetResponse);
  });

  afterEach(() => {
    expect(fetchMock.done()).to.be.true; // All mock RPCs called at least once.
    fetchMock.reset();
  });

  it('shows empty search results', async () => {
    await instantiate(defaultSearchRequest, emptySearchResponse);

    expect(await searchPageSkPO.getSummary()).to.equal('No results matched your search criteria.');
    expect(await searchPageSkPO.getDigests()).to.be.empty;
  });

  it('shows non-empty search results', async () => {
    await instantiate();

    expect(await searchPageSkPO.getSummary()).to.equal('Showing results 1 to 3 (out of 85).');
    expect(await searchPageSkPO.getDigests()).to.deep.equal([
      'Left: fbd3de3fff6b852ae0bb6751b9763d27',
      'Left: 2fa58aa430e9c815755624ca6cca4a72',
      'Left: ed4a8cf9ea9fbb57bf1f302537e07572'
    ]);
  });

  // TODO(lovisolo): Test this more thoroughly (exercise all search parameters, etc.).
  it('updates the search results when the search controls change', async () => {
    await instantiate();

    // We will pretend that the user unchecked "Include untriaged digests".
    const searchRequest = deepCopy(defaultSearchRequest);
    searchRequest.unt = false;
    fetchMock.get('/json/v1/search?' + fromObject(searchRequest as any), () => emptySearchResponse);

    const event = eventPromise('end-task');
    const searchControlsSkPO = await searchPageSkPO.getSearchControlsSkPO();
    await searchControlsSkPO.clickIncludeUntriagedDigestsCheckbox();
    await event;

    expect(await searchPageSkPO.getSummary()).to.equal('No results matched your search criteria.');
  });

  describe('changelist support', () => {
    const searchRequestWithCL = deepCopy(defaultSearchRequest);
    searchRequestWithCL.crs = 'Gerrit';
    searchRequestWithCL.issue = '123456';

    beforeEach(() => {
      setQueryString('?crs=Gerrit&issue=123456');
    });

    it('shows search results with changelist information', async () => {
      await instantiate(searchRequestWithCL);

      expect(await searchPageSkPO.getDigests()).to.deep.equal([
        'Left: fbd3de3fff6b852ae0bb6751b9763d27',
        'Left: 2fa58aa430e9c815755624ca6cca4a72',
        'Left: ed4a8cf9ea9fbb57bf1f302537e07572'
      ]);

      const diffDetailsHrefs = await searchPageSkPO.getDiffDetailsHrefs();
      expect(diffDetailsHrefs[0]).to.contain('changelist_id=123456&crs=Gerrit');
      expect(diffDetailsHrefs[1]).to.contain('changelist_id=123456&crs=Gerrit');
      expect(diffDetailsHrefs[2]).to.contain('changelist_id=123456&crs=Gerrit');
    });

    // TODO(lovisolo): Assert that the changelist controls are visible.
  });

  describe('optional URL query parameters', () => {
    // The test cases below assert that the search page's optional URL parameters (blame, crs,
    // issue) are reflected in the /json/v1/search RPC.
    //
    // No explicit asserts are necessary because if the search RPC is not called with the expected
    // SearchRequest then the fetchMock.done() call in the afterEach hook above will fail.

    it('reads the "blame" URL parameter', async () => {
      setQueryString('?blame=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb');

      const searchRequest = deepCopy(defaultSearchRequest);
      searchRequest.blame = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb';
      await instantiate(searchRequest);
    });

    it('reads the "crs" URL parameter', async () => {
      setQueryString('?crs=gerrit');

      const searchRequest = deepCopy(defaultSearchRequest);
      searchRequest.crs = 'gerrit';
      await instantiate(searchRequest);
    });

    it('reads the "issue" URL parameter', async () => {
      setQueryString('?issue=123456');

      const searchRequest = deepCopy(defaultSearchRequest);
      searchRequest.issue = '123456';
      await instantiate(searchRequest);
    });
  });
});
