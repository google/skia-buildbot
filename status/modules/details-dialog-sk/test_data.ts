import { Commit, TaskSpecDetails } from '../commits-table-sk/commits-table-sk';
import { Comment, Task } from '../rpc/status';

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
  id: '999990',
  name: 'Build-Some-Stuff',
  revision: 'abc123',
  status: 'FAILURE',
  swarmingTaskId: '1234560',
};

export const commitsByHash: Map<string, Commit> = new Map([
  [
    'abc123',
    {
      shortAuthor: 'alice',
      shortHash: 'abc123',
      shortSubject: 'the most recent commit',
      hash: 'parentofabc123',
      author: 'bob@example.com',
      parents: ['grandparentofabc123'],
      subject: '2nd from HEAD',
      body: 'the commit that comes before the most recent commit',
      timestamp: '2020-09-30T16:58:16+00:00',
      isRevert: false,
      isReland: false,
      ignoreFailure: false,
      patchStorage: 'gerrit',
      issue: '320722',
    },
  ],
  [
    'parentofabc123',
    {
      shortAuthor: 'bob',
      shortHash: 'parentofabc123',
      shortSubject: 'the commit before the most recent one',
      hash: 'parentofabc123',
      author: 'bob@example.com',
      parents: ['grandparentofabc123'],
      subject: '2nd from HEAD',
      body: 'the commit that comes before the most recent commit',
      timestamp: '2020-09-30T16:58:16+00:00',
      isRevert: false,
      isReland: false,
      ignoreFailure: false,
      patchStorage: 'gerrit',
      issue: '320722',
    },
  ],
]);

export const commit: Commit = {
  hash: 'parentofabc123',
  author: 'bob@example.com',
  parents: ['grandparentofabc123'],
  subject: '2nd from HEAD',
  body: 'the commit that comes before the most recent commit',
  timestamp: '2020-09-30T16:58:16+00:00',
  shortAuthor: 'bob',
  shortHash: 'parentofabc123',
  shortSubject: 'the commit before the most recent one',
  isRevert: false,
  isReland: false,
  ignoreFailure: false,
  patchStorage: 'gerrit',
  issue: '320722',
};
