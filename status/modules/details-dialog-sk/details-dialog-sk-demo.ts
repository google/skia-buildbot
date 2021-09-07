import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';
import fetchMock from 'fetch-mock';
import { DetailsDialogSk } from './details-dialog-sk';
import {
  comment, commit, commitsByHash, task,
} from './test_data';
import { SetTestSettings } from '../settings';
import { taskDriverData } from '../../../infra-sk/modules/task-driver-sk/test_data';

document.querySelector('details-dialog-sk')!.addEventListener('some-event-name', (e) => {
  document.querySelector('#events')!.textContent = JSON.stringify(e, null, '  ');
});
SetTestSettings({
  swarmingUrl: 'example.com/swarming',
  treeStatusBaseUrl: 'example.com/treestatus',
  logsUrlTemplate:
    'https://ci.chromium.org/raw/build/logs.chromium.org/skia/{{TaskID}}/+/annotations',
  taskSchedulerUrl: 'example.com/ts',
  defaultRepo: 'skia',
  repos: new Map([['skia', 'https://skia.googlesource.com/skia/+show/']]),
});

const element = document.querySelector('details-dialog-sk') as DetailsDialogSk;

element.repo = 'skia';

document.addEventListener('click', async (e) => {
  switch ((<HTMLElement>e.target).id) {
    case 'taskButton':
      element.displayTask(task, [comment], commitsByHash);
      return;
    case 'taskDriverButton':
      fetchMock.getOnce('path:/json/td/999990', taskDriverData);
      element.displayTask(task, [comment], commitsByHash);
      await fetchMock.flush(true);
      return;
    case 'commitButton':
      element.displayCommit(commit, [comment]);
      return;
    case 'taskSpecButton':
      element.displayTaskSpec('Test-Android-Clang-Nexus7-GPU-Tegra3-arm-Debug-All-Android', [
        comment,
      ]);
      return;
    default:
      break;
  }
  // Wasn't a button we care about. Close the dialog.
  if (!element.contains(<Node>e.target)) {
    (<DetailsDialogSk>$$('details-dialog-sk')).close();
  }
});
