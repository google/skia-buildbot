import { $$ } from 'common-sk/modules/dom';
import { CommentsSk } from './comments-sk';
import './index';
import { taskspecComments } from './test_data';

document.querySelector('comments-sk')!.addEventListener('some-event-name', (e) => {
  document.querySelector('#events')!.textContent = JSON.stringify(e, null, '  ');
});

Date.now = () => 1600883976659;
(<CommentsSk>$$('comments-sk')).comments = taskspecComments;
(<CommentsSk>$$('comments-sk')).showFlaky = true;
(<CommentsSk>$$('comments-sk')).showIgnoreFailure = true;
(<CommentsSk>$$('comments-sk')).allowDelete = true;
(<CommentsSk>$$('comments-sk')).allowEmpty = true;
(<CommentsSk>$$('comments-sk')).allowAdd = true;
