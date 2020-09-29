import { Comment, Task } from '../rpc/status';
import { DisplayCommit } from './details-dialog-sk';

export const comment: Comment = {
  id: 'foo',
  repo: 'skia',
  timestamp: new Date(Date.now() - 1000 * 60 * 5).toISOString(),
  user: 'alison@google.com',
  message: 'this is a comment on a task',
  ignoreFailure: true,
  deleted: false,
  flaky: false,
  taskId: 'SOMETASKID',
  taskSpecName: 'Build-Some-Stuff',
  commit: 'abc123',
};

export const task: Task = {
  commits: ['abc123', 'parentofabc123'],
  id: '99999',
  name: 'Build-Some-Stuff',
  revision: 'abc123',
  status: 'FAILURE',
  swarmingTaskId: 'someswarmingtaskid',
};

export const commitsByHash: Map<string, DisplayCommit> = new Map([
  [
    'abc123',
    {
      shortAuthor: 'alice',
      shortHash: 'abc123',
      shortSubject: 'the most recent commit',
    },
  ],
  [
    'parentofabc123',
    {
      shortAuthor: 'bob',
      shortHash: 'parentofabc123',
      shortSubject: 'the commit before the most recent one',
    },
  ],
]);
