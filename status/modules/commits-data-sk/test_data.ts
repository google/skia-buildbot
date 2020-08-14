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
export const response: IncrementalCommitsResponse = {
  metadata: { pod: 'poddy' },
  update: {
    branchheads: [branch0, branch1],
    swarmingurl: 'swarmyswarm',
    startover: true,
    taskschedulerurl: 'tasky',
    timestamp: 'timey',
    comments: [
    commits: [],
    tasks: [],
  }
};
