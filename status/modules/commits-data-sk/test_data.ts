import { Branch, IncrementalUpdate, GetIncrementalCommitsRequest, GetIncrementalCommitsResponse, LongCommit, Comment } from '../rpc/status';
export const branch0: Branch = { name: 'main', head: 'abc123' };
export const branch1: Branch = { name: 'bar', head: '456789' };
export const commentTask: Comment =
{
  id: 'foo',
  repo: 'skia',
  timestamp: 'timey',
  user: 'alison@google.com',
  message: 'this is a comment',
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
  message: 'this is a comment',
  ignoreFailure: true,
  deleted: false,
  flaky: false,
  taskId: '',
  taskSpecName: '',
  commit: '456789'
};
export const commentTaskSpec: Comment =
{
  id: 'foo',
  repo: 'skia',
  timestamp: 'timey',
  user: 'alison@google.com',
  message: 'this is a comment',
  ignoreFailure: true,
  deleted: false,
  flaky: false,
  taskId: '',
  taskSpecName: 'Build-Some-Stuff',
  commit: ''
};
export const incrementalResponse0: GetIncrementalCommitsResponse = {
  metadata: { pod: 'podd', startOver: true },
  update: {
    branchHeads: [branch0, branch1],
    swarmingUrl: 'swarmyswarm',
    taskSchedulerUrl: 'tasky',
    comments: [commentCommit, commentTask, commentTaskSpec],
    commits: [
      { hash: 'abc123', author: 'bob@example.com', parents: ['parentofabc123'], subject: 'current HEAD', body: 'the most recent commit', timestamp: '34613488' },
      { hash: 'parentofabc123', author: 'alison@example.com', parents: ['grandparentofabc123'], subject: '2nd from HEAD', body: 'the commit that comes before the most recent commit', timestamp: '34613288' }],
    tasks: [{commits: ['abc123'], id: '99999', name: 'Build-Some-Stuff', revision: 'whatsarevisioninthiscontext', status: 'SUCCESS', swarmingTaskId: 'idforswarming'}],
  }
};
