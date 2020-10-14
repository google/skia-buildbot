import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import 'elements-sk/error-toast-sk';
import { $$ } from 'common-sk/modules/dom';
import { mockIncrementalResponse, SetupMocks } from '../rpc-mock';
import { SetTestSettings } from '../settings';

declare global {
  interface Window {
    Login: any;
  }
}
window.Login = Promise.resolve({
  Email: 'user@google.com',
  LoginURL: 'https://accounts.google.com/',
});

SetupMocks().expectGetIncrementalCommits(mockIncrementalResponse);
SetTestSettings({
  swarmingUrl: 'example.com/swarming',
  taskSchedulerUrl: 'example.com/ts',
  defaultRepo: 'skia',
  repos: new Map([
    ['skia', 'https://skia.googlesource.com/skia/+show/'],
    ['infra', 'https://skia.googlesource.com/buildbot/+show/'],
    ['skcms', 'https://skia.googlesource.com/skcms/+show/'],
  ]),
});

const table = document.createElement('commits-table-sk');
($$('#container') as HTMLElement).appendChild(table);
