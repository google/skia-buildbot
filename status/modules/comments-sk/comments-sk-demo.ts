import { $$ } from 'common-sk/modules/dom';
import { SetupMocks } from '../rpc-mock';
import { CommentsSk } from './comments-sk';
import './index';
import { taskspecComments } from './test_data';

Date.now = () => 1600883976659;
SetupMocks().expectAddComment({ timestamp: new Date().toISOString() }).expectDeleteComment({});
const element = document.createElement('comments-sk') as CommentsSk;
element.commentData = {
  repo: 'skia',
  taskId: '',
  taskSpec: 'Build-The-Thing',
  commit: '',
  comments: taskspecComments,
};
element.showFlaky = true;
element.showIgnoreFailure = true;
element.allowDelete = true;
element.allowEmpty = true;
element.allowAdd = true;
element.editRights = true;

element.addEventListener('data-update', (e) => {
  document.querySelector('#events')!.textContent = JSON.stringify(e, null, '  ');
});
$$('#container')!.appendChild(element);
