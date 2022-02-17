import './index';
import fetchMock from 'fetch-mock';

import { IndexPageSk } from './index-page-sk';
import { fakeMostRecentBinariesRPCResponse, fakeNow } from './demo_data';
import { CodesizeScaffoldSk } from '../codesize-scaffold-sk/codesize-scaffold-sk';

Date.now = () => fakeNow;

const fakeRpcDelayMillis = 300;

fetchMock.get(
  '/rpc/most_recent_binaries/v1',
  () => new Promise(
    (resolve) => setTimeout(
      () => resolve(JSON.stringify(fakeMostRecentBinariesRPCResponse)),
      fakeRpcDelayMillis,
    ),
  ),
);

// Add the page under test only after all RPCs are mocked out.
const scaffold = new CodesizeScaffoldSk();
document.body.appendChild(scaffold);
scaffold.appendChild(new IndexPageSk());
