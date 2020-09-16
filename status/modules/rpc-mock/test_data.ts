import { Branch, GetIncrementalCommitsResponse, Comment, Task, LongCommit } from '../rpc/status';

function copy<T>(obj: T): T {
  return Object.assign(JSON.parse(JSON.stringify(obj)));
}

export const branch0: Branch = { name: 'main', head: 'abc123' };
export const branch1: Branch = { name: 'bar', head: '456789' };
export const commentTask: Comment =
{
  id: 'foo',
  repo: 'skia',
  timestamp: 'timey',
  user: 'alison@google.com',
  message: 'this is a comment on a task',
  ignoreFailure: true,
  deleted: false,
  flaky: false,
  taskId: 'SOMETASKID',
  taskSpecName: 'Build-Some-Stuff',
  commit: 'abc123'
};
export const commentCommit: Comment =
{
  id: 'foo',
  repo: 'skia',
  timestamp: 'timey',
  user: 'alison@google.com',
  message: 'this is a comment on a commit',
  ignoreFailure: true,
  deleted: false,
  flaky: false,
  taskId: '',
  taskSpecName: '',
  commit: 'parentofabc123'
};
export const commentTaskSpec: Comment =
{
  id: 'foo',
  repo: 'skia',
  timestamp: 'timey',
  user: 'alison@google.com',
  message: 'this is a comment on a task spec',
  ignoreFailure: true,
  deleted: false,
  flaky: false,
  taskId: '',
  taskSpecName: 'Build-Some-Stuff',
  commit: ''
};

const task0: Task = { commits: ['abc123'], id: '99999', name: 'Build-Some-Stuff', revision: 'abc123', status: 'SUCCESS', swarmingTaskId: 'swarmy' };
const commit0: LongCommit = { hash: 'abc123', author: 'alice@example.com', parents: ['parentofabc123'], subject: 'current HEAD', body: 'the most recent commit', timestamp: '34613488' };
export const incrementalResponse0: GetIncrementalCommitsResponse = {
  metadata: { pod: 'podd', startOver: true },
  update: {
    branchHeads: [branch0, branch1],
    swarmingUrl: 'swarmyswarm',
    taskSchedulerUrl: 'taskytask',
    comments: [commentCommit, commentTask, commentTaskSpec],
    commits: [
      commit0,
      { hash: 'parentofabc123', author: 'bob@example.com', parents: ['grandparentofabc123'], subject: '2nd from HEAD', body: 'the commit that comes before the most recent commit', timestamp: '34613288' },
      { hash: 'relandbad', author: 'alice@example.com', parents: ['revertbad'], subject: 'is a reland', body: 'This is a reland of bad', timestamp: '34611288' },
      { hash: 'revertbad', author: 'bob@example.com', parents: ['bad'], subject: 'is a revert', body: 'This reverts commit bad', timestamp: '34608288' },
      { hash: 'bad', author: 'alice@example.com', parents: ['acommitthatisnotlisted'], subject: 'get reverted', body: 'dereference some null pointers', timestamp: '34605288' }],
    tasks: [
      task0,
      { commits: ['parentofabc123'], id: '11111', name: 'Test-Some-Stuff', revision: 'parentofabc123', status: 'FAILURE', swarmingTaskId: 'swarmy' },
      { commits: ['acommitthatisnotlisted'], id: '77777', name: 'Upload-Some-Stuff', revision: 'acommitthatisnotlisted', status: 'SUCCESS', swarmingTaskId: 'swarmy' },
    ],
  }
};

export const responseSingleCommitTask: GetIncrementalCommitsResponse = {
  metadata: { pod: 'podd', startOver: true },
  update: {
    branchHeads: [],
    swarmingUrl: 'swarmingurl',
    taskSchedulerUrl: 'tsurl',
    comments: [],
    commits: [commit0],
    tasks: [task0],
  }
};
