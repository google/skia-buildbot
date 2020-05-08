import './index';
import '../gold-scaffold-sk';
import { $$ } from 'common-sk/modules/dom';
import { deepCopy } from 'common-sk/modules/object';
import { toObject } from 'common-sk/modules/query';
import { fetchMock } from 'fetch-mock';
import { delay } from '../demo_util';
import { triageLogs } from './demo_data';
import { testOnlySetSettings } from '../settings';

const fakeRpcDelayMillis = 300;

testOnlySetSettings({
  title: 'Skia Public',
});
$$('gold-scaffold-sk')._render(); // pick up title from settings.

// The mock /json/triagelog/undo RPC will populate this set.
const undoneIds = new Set();

// Mock /json/triagelog RPC implementation.
function getTriageLogs(details, offset, size) {
  // Filter out undone entries.
  const allTriageLogs = triageLogs.filter((entry) => !undoneIds.has(entry.id));

  // Log entries to be returned.
  const data = [];
  for (let i = offset; i < allTriageLogs.length && data.length < size; i++) {
    let entry = allTriageLogs[i];
    if (!details) {
      entry = deepCopy(entry);
      entry.details = undefined;
    }
    data.push(entry);
  }

  return {
    data: data,
    status: 200,
    pagination: {
      offset: offset,
      size: size,
      total: allTriageLogs.length,
    },
  };
}

// TODO(lovisolo): Consider extracting some of the mock fetch logic below into demo_utils.js.

fetchMock.post('glob:/json/triagelog/undo?id=*', (url) => {
  if ($$('#simulate-rpc-failure').checked) {
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

fetchMock.get('glob:/json/triagelog*', (url) => {
  if ($$('#simulate-rpc-failure').checked) {
    return 500; // Fake an internal server error.
  }

  // Parse query string.
  const queryString = url.substring(url.indexOf('?') + 1);
  const { details, offset, size } = toObject(queryString, /* type hints */ { details: false, offset: 0, size: 0 });
  console.log(`Mock JSON endpoint: URL=${url}, details=${details}, offset=${offset}, size=${size}`);

  // Fake a 300ms delay.
  return delay(getTriageLogs(details, offset, size), fakeRpcDelayMillis);
});

// Create the component after we've had a chance to mock the JSON endpoints.
$$('gold-scaffold-sk').appendChild(document.createElement('triagelog-page-sk'));
