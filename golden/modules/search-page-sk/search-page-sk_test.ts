import './index';
import { setUpElementUnderTest, eventSequencePromise, eventPromise, setQueryString } from '../../../infra-sk/modules/test_util';
import { searchResponse, statusResponse, paramSetResponse } from './demo_data';
import fetchMock from 'fetch-mock';
import { deepCopy } from 'common-sk/modules/object';
import { SearchPageSk } from './search-page-sk';
import { SearchPageSkPO } from './search-page-sk_po';
import { SearchResponse } from '../rpc_types';
import { testOnlySetSettings } from '../settings';

const expect = chai.expect;

describe('search-page-sk', () => {
  const newInstance = setUpElementUnderTest<SearchPageSk>('search-page-sk');

  let searchPageSk: SearchPageSk;
  let searchPageSkPO: SearchPageSkPO;

  const defaultSearchRpcQueryString =
    'fref=false&' +
    'frgbamax=255&' +
    'frgbamin=0&' +
    'head=true&' +
    'include=false&' +
    'neg=false&' +
    'pos=false&' +
    'query=source_type%3Dinfra&' +
    'rquery=source_type%3Dinfra&' +
    'sort=desc&' +
    'unt=true';

  const emptySearchResponse = deepCopy(searchResponse);
  emptySearchResponse.size = 0;
  emptySearchResponse.digests = [];

  const instantiate =
      async (
        searchRpcQueryString: string = defaultSearchRpcQueryString,
        initialSearchResponse: SearchResponse = searchResponse) => {
    fetchMock.get('/json/search?' + searchRpcQueryString, () => initialSearchResponse);
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
    fetchMock.get('/json/trstatus', () => statusResponse);
    fetchMock.get('/json/paramset', () => paramSetResponse);
  });

  afterEach(() => {
    expect(fetchMock.done()).to.be.true; // All mock RPCs called at least once.
    fetchMock.reset();
  });

  it('shows empty search results', async () => {
    await instantiate(defaultSearchRpcQueryString, emptySearchResponse);

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
    ])
  });

  // TODO(lovisolo): Test this more thoroughly (exercise all search parameters, etc.).
  it('updates the search results when the search controls change', async () => {
    await instantiate();

    // We will pretend that the user unchecked "Include untriaged digests".
    fetchMock.get(
      '/json/search?' + defaultSearchRpcQueryString.replace('unt=true', 'unt=false'),
      () => emptySearchResponse);

    const event = eventPromise('end-task');
    const searchControlsSkPO = await searchPageSkPO.getSearchControlsSkPO();
    await searchControlsSkPO.clickIncludeUntriagedDigestsCheckbox();
    await event;

    expect(await searchPageSkPO.getSummary()).to.equal('No results matched your search criteria.');
  });

  it('reads the "blame" URL parameter', async () => {
    setQueryString('?blame=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb');

    await instantiate(
      'blame=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa%3Abbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb&' +
      defaultSearchRpcQueryString);

    // Nothing to assert here. If the RPC wasn't called with the query string above, the call to
    // fetchMock.done() in the afterEach hook will fail.
  });
});
