import './index';
import '../gold-scaffold-sk';
import { $$ } from 'common-sk/modules/dom';
import fetchMock from 'fetch-mock';
import {
  canvaskit,
  fakeNow,
  gm,
  svg,
  trstatus,
} from './demo_data';
import { delay } from '../demo_util';
import { testOnlySetSettings } from '../settings';
import { ByBlameResponse } from '../rpc_types';
import { GoldScaffoldSk } from '../gold-scaffold-sk/gold-scaffold-sk';

testOnlySetSettings({
  title: 'Skia Public',
  defaultCorpus: 'gm',
  baseRepoURL: 'https://skia.googlesource.com/skia.git',
});

// Set up RPC failure simulation.
const getSimulateRpcFailure = (): boolean => sessionStorage.getItem('simulateRpcFailure') === 'true';
const setSimulateRpcFailure = (val: boolean) => sessionStorage.setItem('simulateRpcFailure', val.toString());
$$<HTMLInputElement>('#simulate-rpc-failure')!.checked = getSimulateRpcFailure();
$$<HTMLInputElement>('#simulate-rpc-failure')!.addEventListener('change', (e: Event) => {
  setSimulateRpcFailure((e.target as HTMLInputElement).checked);
});

const fakeRpcDelayMillis = 300;

function byBlame(response: ByBlameResponse) {
  if (getSimulateRpcFailure()) {
    return 500; // Fake an internal server error.
  }
  return delay(response, fakeRpcDelayMillis);
}

Date.now = () => fakeNow;

fetchMock.get('/json/v1/byblame?query=source_type%3Dcanvaskit', () => byBlame(canvaskit));
fetchMock.get('/json/v1/byblame?query=source_type%3Dgm', () => byBlame(gm));
fetchMock.get('/json/v1/byblame?query=source_type%3Dsvg', () => byBlame(svg));
fetchMock.get('/json/v2/trstatus', () => {
  if ($$<HTMLInputElement>('#simulate-rpc-failure')!.checked) {
    return 500; // Fake an internal server error.
  }
  return delay(trstatus, fakeRpcDelayMillis);
});

// By adding these elements after all the fetches are mocked out, they should load ok.
const newScaf = new GoldScaffoldSk();
newScaf.testingOffline = true;
const body = $$('body')!;
body.insertBefore(newScaf, body.childNodes[0]); // Make it the first element in body.
const page = document.createElement('byblame-page-sk');
newScaf.appendChild(page);
