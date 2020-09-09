import './index';
import '../gold-scaffold-sk';
import { $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';
import {
  canvaskit,
  fakeNow,
  gm,
  svg,
  trstatus,
} from './demo_data';
import { delay } from '../demo_util';
import { testOnlySetSettings } from '../settings';

testOnlySetSettings({
  title: 'Skia Public',
  defaultCorpus: 'gm',
  baseRepoURL: 'https://skia.googlesource.com/skia.git',
});

// Set up RPC failure simulation.
const getSimulateRpcFailure = () => sessionStorage.getItem('simulateRpcFailure') === 'true';
const setSimulateRpcFailure = (val) => sessionStorage.setItem('simulateRpcFailure', val);
$$('#simulate-rpc-failure').checked = getSimulateRpcFailure();
$$('#simulate-rpc-failure').addEventListener('change', (e) => {
  setSimulateRpcFailure(e.target.checked);
});

const fakeRpcDelayMillis = 300;

function byBlame(response) {
  if (getSimulateRpcFailure()) {
    return 500; // Fake an internal server error.
  }
  return delay(response, fakeRpcDelayMillis);
}

Date.now = () => fakeNow;

fetchMock.get('/json/v1/byblame?query=source_type%3Dcanvaskit', () => byBlame(canvaskit));
fetchMock.get('/json/v1/byblame?query=source_type%3Dgm', () => byBlame(gm));
fetchMock.get('/json/v1/byblame?query=source_type%3Dsvg', () => byBlame(svg));
fetchMock.get('/json/v1/trstatus', () => {
  if ($$('#simulate-rpc-failure').checked) {
    return 500; // Fake an internal server error.
  }
  return delay(trstatus, fakeRpcDelayMillis);
});

// By adding these elements after all the fetches are mocked out, they should load ok.
const newScaf = document.createElement('gold-scaffold-sk');
newScaf.setAttribute('testing_offline', 'true');
const body = $$('body');
body.insertBefore(newScaf, body.childNodes[0]); // Make it the first element in body.
const page = document.createElement('byblame-page-sk');
newScaf.appendChild(page);
