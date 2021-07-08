import './index';
import '../gold-scaffold-sk';
import { $$ } from 'common-sk/modules/dom';
import { deepCopy } from 'common-sk/modules/object';
import { toObject } from 'common-sk/modules/query';
import fetchMock from 'fetch-mock';
import { delay } from '../demo_util';
import { triageLogs, triageLogsV2 } from './demo_data';
import { testOnlySetSettings } from '../settings';
import { exampleStatusData } from '../last-commit-sk/demo_data';
import {
  TriageLogEntry, TriageLogEntry2, TriageLogResponse, TriageLogResponse2,
} from '../rpc_types';
import { GoldScaffoldSk } from '../gold-scaffold-sk/gold-scaffold-sk';

const fakeRpcDelayMillis = 300;

testOnlySetSettings({
  title: 'Skia Public',
  baseRepoURL: 'https://skia.googlesource.com/skia.git',
});

// The mock /json/v1/triagelog/undo RPC will populate this set.
const undoneIds = new Set();

// Mock /json/v1/triagelog RPC implementation.
function getTriageLogs(details: boolean, offset: number, size: number): TriageLogResponse {
  // Filter out undone entries.
  const allTriageLogs = triageLogs.filter((entry) => !undoneIds.has(entry.id));

  // Log entries to be returned.
  const entries: TriageLogEntry[] = [];
  for (let i = offset; i < allTriageLogs.length && entries.length < size; i++) {
    let entry = allTriageLogs[i];
    if (!details) {
      entry = deepCopy(entry);
      entry.details = null;
    }
    entries.push(entry);
  }

  return {
    entries: entries,
    offset: offset,
    size: size,
    total: allTriageLogs.length,
  };
}

// TODO(lovisolo): Consider extracting some of the mock fetch logic below into demo_utils.ts.

fetchMock.post('glob:/json/v1/triagelog/undo?id=*', (url) => {
  if ($$<HTMLInputElement>('#simulate-rpc-failure')!.checked) {
    return 500; // Fake an internal server error.
  }

  // Parse query string.
  const queryString = url.substring(url.indexOf('?') + 1);
  const { id } = toObject(queryString, /* type hints */ { id: '' });
  console.log(`Mock JSON endpoint: URL=${url}, id=${id}`);

  // Undo entry.
  undoneIds.add(id);

  // Return results of the first search page with a 300ms delay.
  return delay(getTriageLogs(false, 0, 20), fakeRpcDelayMillis);
});

fetchMock.get('glob:/json/v1/triagelog*', (url) => {
  if ($$<HTMLInputElement>('#simulate-rpc-failure')!.checked) {
    return 500; // Fake an internal server error.
  }

  // Parse query string.
  const queryString = url.substring(url.indexOf('?') + 1);
  const { details, offset, size } = toObject(queryString, { details: false, offset: 0, size: 0 });

  // Fake a 300ms delay.
  return delay(
    getTriageLogs(details as boolean, offset as number, size as number),
    fakeRpcDelayMillis,
  );
});
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

fetchMock.get('/json/v1/trstatus', JSON.stringify(exampleStatusData));

// By adding these elements after all the fetches are mocked out, they should load ok.
const newScaf = new GoldScaffoldSk();
newScaf.testingOffline = true;
// Make it the first element in body.
document.body.insertBefore(newScaf, document.body.childNodes[0]);
const page = document.createElement('triagelog-page-sk');
newScaf.appendChild(page);
