import './index';
import '../gold-scaffold-sk';
import { $$ } from 'common-sk/modules/dom';
import { deepCopy } from 'common-sk/modules/object';
import { testOnlySetSettings } from '../settings';
import { SearchPageSk } from './search-page-sk';
import { searchResponse, statusResponse, paramSetResponse, fakeNow } from './demo_data';
import fetchMock from 'fetch-mock';
import { setImageEndpointsForDemos } from '../common';

testOnlySetSettings({
  title: 'Skia Infra',
  defaultCorpus: 'infra',
  baseRepoURL: 'https://skia.googlesource.com/buildbot.git',
});

// TODO(lovisolo): Replace any with GoldScaffoldSk when said component is ported to TypeScript.
// TODO(lovisolo): Consider folding this into testOnlySetSettings().
$$<any>('gold-scaffold-sk')._render(); // Pick up title from instance settings.

Date.now = () => fakeNow;

fetchMock.get('/json/trstatus', () => statusResponse);
fetchMock.get('/json/paramset', () => paramSetResponse);
fetchMock.get('glob:/json/search*', (url: string) => {
  const filteredSearchResponse = deepCopy(searchResponse);

  // Filter only by untriaged/positive/negative
  filteredSearchResponse.digests = filteredSearchResponse.digests!.filter(
    (digest) =>
      (digest!.status === 'untriaged' && url.includes('unt=true')) ||
      (digest!.status === 'positive' && url.includes('pos=true')) ||
      (digest!.status === 'negative' && url.includes('neg=true')));
  filteredSearchResponse.size = filteredSearchResponse.digests.length;

  return filteredSearchResponse
});

setImageEndpointsForDemos();

const searchPageSk = new SearchPageSk();
$$('gold-scaffold-sk')!.appendChild(searchPageSk);
