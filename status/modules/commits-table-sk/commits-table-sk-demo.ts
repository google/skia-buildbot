import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import '../../../elements-sk/modules/error-toast-sk';
import { $$ } from '../../../infra-sk/modules/dom';
import { mockIncrementalResponse, SetupMocks } from '../rpc-mock';
import { SetTestSettings } from '../settings';
import { sameTimestamp } from './test_data';

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
  repoToProject: new Map([
    ['skia', 'skia'],
    ['infra', 'skiabuildbot'],
    ['skcms', 'skcms'],
  ]),
});

const table = document.createElement('commits-table-sk');
($$('#container') as HTMLElement).appendChild(table);
