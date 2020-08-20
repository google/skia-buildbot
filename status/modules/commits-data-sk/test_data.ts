import { Branch, IncrementalUpdate, IncrementalCommitsRequest, IncrementalCommitsResponse, LongCommit, Comment } from '../rpc/statusFe';
export const branch0: Branch = { name: 'main', head: 'abc123' };
export const branch1: Branch = { name: 'bar', head: '456789' };
export const commentTask: Comment =
{
  id: 'foo',
  repo: 'skia',
  revision: 'abc123',
  timestamp: 'timey',
  user: 'alison@google.com',
  message: 'this is a comment',
  ignorefailure: true,
  deleted: false,
  flaky: false,
  taskid: 'SOMETASKID',
  taskspecname: 'Build-Some-Stuff',
  commithash: 'abc123'
};
export const commentCommit: Comment =
{
  id: 'foo',
  repo: 'skia',
  revision: 'abc123',
  timestamp: 'timey',
  user: 'alison@google.com',
  message: 'this is a comment',
  ignorefailure: true,
  deleted: false,
  flaky: false,
  taskid: '',
  taskspecname: '',
  commithash: '456789'
};
export const commentTaskSpec: Comment =
{
  id: 'foo',
  repo: 'skia',
  revision: 'abc123',
  timestamp: 'timey',
  user: 'alison@google.com',
  message: 'this is a comment',
  ignorefailure: true,
  deleted: false,
  flaky: false,
  taskid: '',
  taskspecname: 'Build-Some-Stuff',
  commithash: ''
};
const task0 = { commits: ['abc123'], id: '1', name: 'Build-Android-Stuff-Metal', revision: 'whatsarevisioninthiscontext', status: 'SUCCESS', swarmingtaskid: 'idforswarming' };
const task1 = Object.assign(JSON.parse(JSON.stringify(task0)), {id: '2', name: 'Test-Android-Pixel12-Stuff'});
const task2 = Object.assign(JSON.parse(JSON.stringify(task0)), {id: '3', name: 'Housekeeper-PerCommit-Small'});
const task3 = Object.assign(JSON.parse(JSON.stringify(task0)), {id: '4', name: 'Housekeeper-PerCommit-Medium'});
const task4 = Object.assign(JSON.parse(JSON.stringify(task0)), {id: '5', name: 'Housekeeper-PerCommit-Large'});
const task5 = Object.assign(JSON.parse(JSON.stringify(task0)), {id: '6', name: 'Build-iOS-Mac15.5-Metal'});
const task6 = Object.assign(JSON.parse(JSON.stringify(task0)), {id: '7', name: 'Build-iOS-Mac15-Metal'});
const task7 = Object.assign(JSON.parse(JSON.stringify(task0)), {id: '8', name: 'Test-iOS-Mac15.5-Metal'});
export const incrementalResponse0: IncrementalCommitsResponse = {
  metadata: { pod: 'podd' },
  update: {
    branchheads: [branch0, branch1],
    swarmingurl: 'swarmyswarm',
    startover: true,
    taskschedulerurl: 'tasky',
    timestamp: 'timey',
    comments: [commentCommit, commentTask, commentTaskSpec],
    commits: [
      { hash: 'abc123', author: 'bob@example.com', parents: ['parentofabc123'], subject: 'current HEAD', body: 'the most recent commit', timestamp: '34613488' },
      { hash: 'parentofabc123', author: 'alison@example.com', parents: ['grandparentofabc123'], subject: '2nd from HEAD', body: 'the commit that comes before the most recent commit', timestamp: '34613288' }],
    tasks: [task0, task1, task2, task3, task4, task5, task6, task7],
  }
};
