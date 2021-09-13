import './index';
import '../gold-scaffold-sk';
import { $$ } from 'common-sk/modules/dom';
import fetchMock from 'fetch-mock';
import { delay } from '../demo_util';
import { triageLogsV2 } from './demo_data';
import { testOnlySetSettings } from '../settings';
import { exampleStatusData } from '../last-commit-sk/demo_data';
import { TriageLogResponse2 } from '../rpc_types';
import { GoldScaffoldSk } from '../gold-scaffold-sk/gold-scaffold-sk';

const fakeRpcDelayMillis = 300;

testOnlySetSettings({
  title: 'Skia Public',
  baseRepoURL: 'https://skia.googlesource.com/skia.git',
});

// TODO(lovisolo): Consider extracting some of the mock fetch logic below into demo_utils.ts.
fetchMock.get('glob:/json/v2/triagelog*', () => {
  if ($$<HTMLInputElement>('#simulate-rpc-failure')!.checked) {
    return 500; // Fake an internal server error.
  }
  const response: TriageLogResponse2 = {
    entries: triageLogsV2,
    offset: 0,
    size: 20,
    total: triageLogsV2.length,
  };
  // Fake a 300ms delay.
  return delay(response, fakeRpcDelayMillis);
});

fetchMock.get('/json/v2/trstatus', JSON.stringify(exampleStatusData));

// By adding these elements after all the fetches are mocked out, they should load ok.
const newScaf = new GoldScaffoldSk();
newScaf.testingOffline = true;
// Make it the first element in body.
document.body.insertBefore(newScaf, document.body.childNodes[0]);
const page = document.createElement('triagelog-page-sk');
newScaf.appendChild(page);
