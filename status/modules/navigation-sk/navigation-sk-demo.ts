import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { SetTestSettings } from '../settings';
import { NavigationSk } from './navigation-sk';

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
const el = document.createElement('navigation-sk') as NavigationSk;
document.querySelector('#container')!.appendChild(el);

el.addEventListener('repo-changed', (e) => {
  document.querySelector('#events')!.textContent = JSON.stringify(e, null, '  ');
});
