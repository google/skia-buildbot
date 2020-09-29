import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { DetailsDialogSk } from './details-dialog-sk';
import { comment, commitsByHash, task } from './test_data';
import { $$ } from 'common-sk/modules/dom';

document.querySelector('details-dialog-sk')!.addEventListener('some-event-name', (e) => {
  document.querySelector('#events')!.textContent = JSON.stringify(e, null, '  ');
});
document.querySelector('button')!.addEventListener('click', (e) => {
  (<DetailsDialogSk>document.querySelector('details-dialog-sk')).displayTask(
    task,
    [comment],
    commitsByHash
  );
});
document.addEventListener('click', (e) => {
  switch ((<HTMLElement>e.target).id) {
    case 'taskButton':
      (<DetailsDialogSk>document.querySelector('details-dialog-sk')).displayTask(
        task,
        [comment],
        commitsByHash
      );
      return;
    case 'commitButton':
      (<DetailsDialogSk>document.querySelector('details-dialog-sk')).displayTask(
        task,
        [comment],
        commitsByHash
      );
      return;
    case 'taskSpecButton':
      (<DetailsDialogSk>document.querySelector('details-dialog-sk')).displayTask(
        task,
        [comment],
        commitsByHash
      );
      return;
    default:
      break;
  }
  // Wasn't a button we care about. Close the dialog.
  if (!$$('details-dialog-sk')!.contains(<Node>e.target)) {
    console.log('clicked outside the dialog');
    (<DetailsDialogSk>$$('details-dialog-sk')).close();
  }
});
