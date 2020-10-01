import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import '../commits-data-sk';
import { $$ } from 'common-sk/modules/dom';
import { mockIncrementalResponse, SetupMocks } from '../rpc-mock';
import { SetTestSettings } from '../settings';

SetupMocks().expectGetIncrementalCommits(mockIncrementalResponse);
SetTestSettings({
  swarmingUrl: 'example.com/swarming',
  taskSchedulerUrl: 'example.com/ts',
  defaultRepo: 'skia',
  repos: new Map([['skia', 'https://skia.googlesource.com/skia/+show/']]),
});

const data = document.createElement('commits-data-sk');
($$('#container') as HTMLElement).appendChild(data);
const table = document.createElement('commits-table-sk');
($$('#container') as HTMLElement).appendChild(table);
