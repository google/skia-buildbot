import './index';
import { incrementalResponse0, SetupMocks } from '../rpc-mock';
import { $$ } from 'common-sk/modules/dom';
import { SetTestSettings } from '../settings';

SetupMocks().expectGetIncrementalCommits(incrementalResponse0);
SetTestSettings({
  swarmingUrl: 'example.com/swarming',
  taskSchedulerUrl: 'example.com/ts',
  defaultRepo: 'skia',
  repos: new Map([['skia', 'https://skia.googlesource.com/skia/+show/']]),
});
const data = document.createElement('status-sk');
($$('#container') as HTMLElement).appendChild(data);
(document.querySelector('#AllFilter') as HTMLElement).click();

document.querySelector('status-sk')!.addEventListener('some-event-name', (e) => {
  document.querySelector('#events')!.textContent = JSON.stringify(e, null, '  ');
});
