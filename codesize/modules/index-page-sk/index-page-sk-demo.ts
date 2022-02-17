import './index';
import fetchMock from 'fetch-mock';
import { BinaryRPCRequest, BinaryRPCResponse } from '../rpc_types';
import { IndexPageSk } from './index-page-sk';
import { CodesizeScaffoldSk } from '../codesize-scaffold-sk/codesize-scaffold-sk';

const fakeRpcDelayMillis = 300;

const fakeRPCResponse: BinaryRPCResponse = {
  metadata: {
    version: 1,
    timestamp: '2022-02-15T23:57:57Z',
    swarming_task_id: '59189d06df0e9611',
    swarming_server: 'https://chromium-swarm.appspot.com',
    task_id: 'rG2vO5FkQOODEHgHf7W8',
    task_name: 'CodeSize-dm-Debian10-Clang-x86_64-Debug',
    compile_task_name: 'Build-Debian10-Clang-x86_64-Debug',
    binary_name: 'dm',
    bloaty_cipd_version: 'version:1',
    bloaty_args: ['build/dm', '-d', 'compileunits,symbols', '-n', '0', '--tsv'],
    patch_issue: '',
    patch_server: '',
    patch_set: '',
    repo: 'https://skia.googlesource.com/skia.git',
    revision: '34e3b35eb460e8668bb063adeefdc1fed857d075',
    commit_timestamp: '2022-02-15T23:42:43Z',
    author: 'Alice (alice@google.com)',
    subject: '[codesize] Define more CodeSize tasks for testing purposes (both for CQ and waterfall).',
  },
  rows: [
    { name: 'ROOT', parent: '', size: 0 },
    { name: 'a', parent: 'ROOT', size: 50 },
    { name: 'a1', parent: 'a', size: 30 },
    { name: 'a2', parent: 'a', size: 20 },
    { name: 'b', parent: 'ROOT', size: 100 },
  ],
};

fetchMock.post(
  (url, opts) => {
    const request = JSON.parse(opts.body?.toString() || '') as BinaryRPCRequest;
    return url === '/rpc/binary/v1'
      && request.commit === fakeRPCResponse.metadata.revision
      && request.patch_issue === fakeRPCResponse.metadata.patch_issue
      && request.patch_set === fakeRPCResponse.metadata.patch_set
      && request.binary_name === fakeRPCResponse.metadata.binary_name
      && request.compile_task_name === fakeRPCResponse.metadata.compile_task_name;
  },
  () => new Promise(
    (resolve) => setTimeout(
      () => resolve(JSON.stringify(fakeRPCResponse)),
      fakeRpcDelayMillis,
    ),
  ),
);

const queryString = `?commit=${fakeRPCResponse.metadata.revision}&`
  + `patch_issue=${fakeRPCResponse.metadata.patch_issue}&`
  + `patch_set=${fakeRPCResponse.metadata.patch_set}&`
  + `binary_name=${fakeRPCResponse.metadata.binary_name}&`
  + `compile_task_name=${fakeRPCResponse.metadata.compile_task_name}`;
window.history.pushState(null, '', queryString);

// Add the page under test only after all RPCs are mocked out.
const scaffold = new CodesizeScaffoldSk();
document.body.appendChild(scaffold);
scaffold.appendChild(new IndexPageSk());
