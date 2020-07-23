import './index';
import '../gold-scaffold-sk';
import { ParamSet, fromParamSet } from 'common-sk/modules/query';
import { $$ } from 'common-sk/modules/dom';
import { delay } from '../demo_util';
import { testOnlySetSettings } from '../settings';
import { SearchPageSk } from './search-page-sk';
import { searchResponse, statusResponse, paramSetResponse } from './demo_data';
import fetchMock from 'fetch-mock';

// testOnlySetSettings({
//   title: 'Skia Public',
//   defaultCorpus: 'gm',
//   baseRepoURL: 'https://skia.googlesource.com/skia.git',
// });
// $$('gold-scaffold-sk')._render(); // pick up title from settings.

// // Set up RPC failure simulation.
// const getSimulateRpcFailure = () => sessionStorage.getItem('simulateRpcFailure') === 'true';
// const setSimulateRpcFailure = (val) => sessionStorage.setItem('simulateRpcFailure', val);
// $$('#simulate-rpc-failure').checked = getSimulateRpcFailure();
// $$('#simulate-rpc-failure').addEventListener('change', (e) => {
//   setSimulateRpcFailure(e.target.checked);
// });

// const fakeRpcDelayMillis = 300;

// function byBlame(response) {
//   if (getSimulateRpcFailure()) {
//     return 500; // Fake an internal server error.
//   }
//   return delay(response, fakeRpcDelayMillis);
// }

// Date.now = () => fakeNow;

// fetchMock.get('/json/byblame?query=source_type%3Dcanvaskit', () => byBlame(canvaskit));
// fetchMock.get('/json/byblame?query=source_type%3Dgm', () => byBlame(gm));
// fetchMock.get('/json/byblame?query=source_type%3Dsvg', () => byBlame(svg));
// fetchMock.get('/json/trstatus', () => {
//   if ($$('#simulate-rpc-failure').checked) {
//     return 500; // Fake an internal server error.
//   }
//   return delay(trstatus, fakeRpcDelayMillis);
// });

fetchMock.get('/json/trstatus', () => statusResponse);
fetchMock.get('/json/paramset', () => paramSetResponse);
fetchMock.get('glob:/json/search*', () => searchResponse);

// Create the component after we've had a chance to mock the JSON endpoints.
const searchPageSk = new SearchPageSk();
$$('gold-scaffold-sk')!.appendChild(searchPageSk);
