import './index';
import fetchMock from 'fetch-mock';
import { BinarySizeDiffRPCRequest, BinarySizeDiffRPCResponse } from '../rpc_types';
import { BinaryDiffPageSk } from './binary-diff-page-sk';
import { CodesizeScaffoldSk } from '../codesize-scaffold-sk/codesize-scaffold-sk';

const fakeRpcDelayMillis = 300;

const fakeRPCResponse: BinarySizeDiffRPCResponse = {
  metadata: {
    version: 1,
    timestamp: '2022-02-15T23:57:57Z',
    swarming_task_id: '59189d06df0e9611',
    swarming_server: 'https://chromium-swarm.appspot.com',
    task_id: 'rG2vO5FkQOODEHgHf7W8',
    task_name: 'CodeSize-dm-Debian10-Clang-x86_64-Debug',
    compile_task_name: 'Build-Debian10-Clang-x86_64-Debug',
    compile_task_name_no_patch: 'Build-Debian10-Clang-x86_64-Debug-NoPatch',
    binary_name: 'dm',
    bloaty_cipd_version: 'version:1',
    bloaty_args: ['build/dm', '-d', 'compileunits,symbols', '-n', '0', '--tsv'],
    bloaty_diff_args: ['build/dm', '--', 'build_nopatch/dm'],
    patch_issue: '12345',
    patch_server: 'https://skia-review.googlesource.com',
    patch_set: '6',
    repo: 'https://skia.googlesource.com/skia.git',
    revision: '34e3b35eb460e8668bb063adeefdc1fed857d075',
    commit_timestamp: '2022-02-15T23:42:43Z',
    author: 'Alice (alice@google.com)',
    subject: '[codesize] Define more CodeSize tasks for testing purposes (both for CQ and waterfall).',
  },
  // Example taken from
  // gs://skia-codesize/2022/07/27/04/tryjob/556358/56/lnwGGlkpXd2obFWx9xrA/Build-Debian10-Clang-x86_64-OptimizeForSize/dm.diff.txt.
  raw_diff: `     VM SIZE                     FILE SIZE
 --------------               --------------
  +0.1% +9.75Ki .rodata       +9.75Ki  +0.1%
  [ = ]       0 .debug_info       +45  +0.0%
  [ = ]       0 .debug_str        +45  +0.0%
  [ = ]       0 .debug_line       +28  +0.0%
  [ = ]       0 .debug_ranges     +16  +0.0%
  +0.0%     +16 .text             +16  +0.0%
  [ = ]       0 [Unmapped]        +10  +200%
  -0.0%      -8 .eh_frame          -8  -0.0%
  +0.0% +9.76Ki TOTAL         +9.90Ki  +0.0%
`,
};

fetchMock.post(
  (url, opts) => {
    const request = JSON.parse(opts.body?.toString() || '') as BinarySizeDiffRPCRequest;
    return url === '/rpc/binary_size_diff/v1'
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
scaffold.appendChild(new BinaryDiffPageSk());
