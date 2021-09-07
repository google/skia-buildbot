import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import 'elements-sk/error-toast-sk';
import { $$ } from 'common-sk/modules/dom';
import { mockIncrementalResponse, SetupMocks } from '../rpc-mock';
import { SetTestSettings } from '../settings';
import { sameTimestamp } from './test_data';

declare global {
  interface Window {
    Login: any;
  }
}
window.Login = Promise.resolve({
  Email: 'user@google.com',
  LoginURL: 'https://accounts.google.com/',
});
const resp = mockIncrementalResponse;
resp.update!.commits = sameTimestamp;
SetupMocks().expectGetIncrementalCommits(resp);
SetTestSettings({
  swarmingUrl: 'example.com/swarming',
  taskSchedulerUrl: 'example.com/ts',
  treeStatusBaseUrl: 'example.com/treestatus',
  logsUrlTemplate: 'https://ci.chromium.org/raw/build/logs.chromium.org/skia/TASKID/+/annotations',
  defaultRepo: 'skia',
  repos: new Map([
    ['skia', 'https://skia.googlesource.com/skia/+show/'],
    ['infra', 'https://skia.googlesource.com/buildbot/+show/'],
    ['skcms', 'https://skia.googlesource.com/skcms/+show/'],
  ]),
});

const table = document.createElement('commits-table-sk');
($$('#container') as HTMLElement).appendChild(table);
