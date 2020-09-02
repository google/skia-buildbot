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

const newScaf = document.createElement('gold-scaffold-sk');
newScaf.setAttribute('testing_offline', 'true');
const body = $$('body');
body?.insertBefore(newScaf, body.childNodes[0]); // Make it the first element in body.
const page = new SearchPageSk();
newScaf.appendChild(page);
