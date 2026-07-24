import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import fetchMock from 'fetch-mock';
import { $$ } from '../../../infra-sk/modules/dom';
import { DetailsDialogSk } from './details-dialog-sk';
import { comment, commit, commitsByHash, task } from './test_data';
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
  repoToProject: new Map([['skia', 'skia']]),
});

const element = document.querySelector('details-dialog-sk') as DetailsDialogSk;

element.repo = 'skia';

document.addEventListener('click', async (e) => {
  const targetId = (<HTMLElement>e.target).id;
  if (['taskButton', 'taskDriverButton', 'commitButton', 'taskSpecButton'].includes(targetId)) {
    fetchMock.restore();
  }
  switch (targetId) {
    case 'taskButton':
      fetchMock.get('path:/json/task-summary/999990', {
        errorMessage:
          'Something went wrong while executing the task in EMCC release run.\nLine 42: compile error: undefined reference to function',
        analysis: 'Compile failure in CanvasKit target.',
      });
      fetchMock.get('path:/json/td/999990', 404);
      element.displayTask(task, [comment], commitsByHash);
      await fetchMock.flush(true);
      return;
    case 'taskDriverButton':
      fetchMock.get('path:/json/td/999990', taskDriverData);
      fetchMock.get('path:/json/task-summary/999990', {
        errorMessage:
          'Something went wrong while executing the task in EMCC release run.\nLine 42: compile error: undefined reference to function',
        analysis: 'Compile failure in CanvasKit target.',
      });
      element.displayTask(task, [comment], commitsByHash);
      await fetchMock.flush(true);
      return;
    case 'commitButton':
      element.displayCommit(commit, [comment]);
      return;
    case 'taskSpecButton':
      element.displayTaskSpec('', 'Test-Android-Clang-Nexus7-GPU-Tegra3-arm-Debug-All-Android', [
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
