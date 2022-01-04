import './index';
import fetchMock from 'fetch-mock';
import { BloatyRPCResponse } from '../rpc_types';
import { IndexPageSk } from './index-page-sk';

const fakeRpcDelayMillis = 300;

const fakeRPCResponse: BloatyRPCResponse = {
  rows: [
    { name: 'ROOT', parent: '', size: 0 },
    { name: 'a', parent: 'ROOT', size: 50 },
    { name: 'a1', parent: 'a', size: 30 },
    { name: 'a2', parent: 'a', size: 20 },
    { name: 'b', parent: 'ROOT', size: 100 },
  ],
};

fetchMock.get(
  '/rpc/bloaty/v1',
  () => new Promise(
    (resolve) => setTimeout(
      () => resolve(JSON.stringify(fakeRPCResponse)),
      fakeRpcDelayMillis,
    ),
  ),
);

// Add the page under test only after all RPCs are mocked out.
document.body.appendChild(new IndexPageSk());
