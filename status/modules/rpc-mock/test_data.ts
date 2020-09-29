/**
 * Deterministic data for tests, crafted to cover various use cases, whereas mock_data is for
 * visualizing a complete status table via nondeterministically generated tasks/commits.
 */
import { Branch, GetIncrementalCommitsResponse, Comment, Task, LongCommit } from '../rpc/status';

function copy<T>(obj: T): T {
  return JSON.parse(JSON.stringify(obj));
}

export const branch0: Branch = { name: 'main', head: 'abc123' };
export const branch1: Branch = { name: 'bar', head: '456789' };
export const commentTask: Comment = {
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
  commit: 'abc123',
};
export const commentCommit: Comment = {
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
  commit: 'parentofabc123',
};
export const commentTaskSpec: Comment = {
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
  commit: '',
};

// Single commit task.
const task0: Task = {
  commits: ['abc123'],
  id: '99999',
  name: 'Build-Some-Stuff',
  revision: 'abc123',
  status: 'SUCCESS',
  swarmingTaskId: 'swarmy',
};
const task1: Task = {
  commits: ['parentofabc123'],
  id: '11111',
  name: 'Test-Some-Stuff',
  revision: 'parentofabc123',
  status: 'FAILURE',
  swarmingTaskId: 'swarmy',
};
const task2: Task = {
  commits: ['acommitthatisnotlisted'],
  id: '77777',
  name: 'Upload-Some-Stuff',
  revision: 'acommitthatisnotlisted',
  status: 'SUCCESS',
  swarmingTaskId: 'swarmy',
};
const multicommitTask: Task = {
  commits: ['abc123', 'parentofabc123'],
  id: '99999',
  name: 'Build-Some-Stuff',
  revision: 'abc123',
  status: 'FAILURE',
  swarmingTaskId: 'swarmy',
};
const commit0: LongCommit = {
  hash: 'abc123',
  author: 'alice@example.com',
  parents: ['parentofabc123'],
  subject: 'current HEAD',
  body: 'the most recent commit',
  timestamp: '34613488',
};
const commit1: LongCommit = {
  hash: 'parentofabc123',
  author: 'bob@example.com',
  parents: ['grandparentofabc123'],
  subject: '2nd from HEAD',
  body: 'the commit that comes before the most recent commit',
  timestamp: '34613288',
};
const branchCommit0: LongCommit = {
  hash: '456789',
  author: 'bob@example.com',
  parents: ['10111213'],
  subject: 'branch commit',
  body: 'commit on differnet branch',
  timestamp: '34613388',
};
export const incrementalResponse0: GetIncrementalCommitsResponse = {
  metadata: { pod: 'podd', startOver: true },
  update: {
    branchHeads: [branch0, branch1],
    comments: [commentCommit, commentTask, commentTaskSpec],
    commits: [
      commit0,
      commit1,
      {
        hash: 'relandbad',
        author: 'alice@example.com',
        parents: ['revertbad'],
        subject: 'is a reland',
        body: 'This is a reland of bad',
        timestamp: '34611288',
      },
      {
        // To test handling of leading digits in the selector.
        hash: '1revertbad',
        author: 'bob@example.com',
        parents: ['bad'],
        subject: 'is a revert',
        body: 'This reverts commit bad',
        timestamp: '34608288',
      },
      {
        hash: 'bad',
        author: 'alice@example.com',
        parents: ['acommitthatisnotlisted'],
        subject: 'get reverted',
        body: 'dereference some null pointers',
        timestamp: '34605288',
      },
    ],
    tasks: [task0, task1, task2],
  },
};

export const responseSingleCommitTask: GetIncrementalCommitsResponse = {
  metadata: { pod: 'podd', startOver: true },
  update: {
    branchHeads: [],
    comments: [],
    commits: [copy(commit0)],
    tasks: [copy(task0)],
  },
};

export const responseMultiCommitTask = (() => {
  const r = copy(responseSingleCommitTask);
  r.update!.tasks = [copy(multicommitTask)];
  r.update!.commits!.push(copy(commit1));
  return r;
})();

// contains a single task covering 2 commits, that have a different-branch commit between them.
export const responseNoncontiguousCommitsTask = (() => {
  const r = copy(responseSingleCommitTask);
  r.update!.tasks = [copy(multicommitTask)];
  // Insert a 'hole' between the commits the task covers.
  r.update!.commits = copy([commit0, branchCommit0, commit1]);
  return r;
})();

export const responseTasksToFilter = (() => {
  const r = copy(incrementalResponse0);
  r.update!.tasks = [];
  // Add some other tasks to make it interesting.
  const tasks = [
    { name: 'Always-Green-Spec', status: 'SUCCESS', commits: ['abc123'] },
    { name: 'Always-Green-Spec', status: 'SUCCESS', commits: ['parentofabc123'] },
    { name: 'Always-Red-Spec', status: 'FAILURE', commits: ['abc123'] },
    { name: 'Always-Red-Spec', status: 'FAILURE', commits: ['parentofabc123'] },
    { name: 'Interesting-Spec', status: 'FAILURE', commits: ['abc123'] },
    { name: 'Interesting-Spec', status: 'SUCCESS', commits: ['parentofabc123'] },
    { name: 'Only-Failed-On-Commented-Commit-Spec', status: 'SUCCESS', commits: ['abc123'] },
    {
      name: 'Only-Failed-On-Commented-Commit-Spec',
      status: 'FAILURE',
      commits: ['parentofabc123'],
    },
  ];
  let i = 0;
  for (let task of tasks) {
    r.update!.tasks.push(Object.assign(copy(task0), task, { id: `id${i++}` }));
  }
  // Add a comment to 'Always-Red-Spec'.
  r.update!.comments!.push(
    Object.assign(copy(commentTaskSpec), { ignoreFailure: false, taskSpecName: 'Always-Red-Spec' })
  );
  return r;
})();
