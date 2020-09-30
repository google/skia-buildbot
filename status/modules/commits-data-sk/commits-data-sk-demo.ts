import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';
import { SetupMocks } from '../rpc-mock';
import { SetTestSettings } from '../settings';

SetupMocks();
SetTestSettings({
  swarmingUrl: 'example.com/swarming',
  taskSchedulerUrl: 'example.com/ts',
  defaultRepo: 'skia',
  repos: new Map([['skia', 'https://skia.googlesource.com/skia/+show/']]),
});
const ele = document.createElement('commits-data-sk');
($$('#container') as HTMLElement).appendChild(ele);
