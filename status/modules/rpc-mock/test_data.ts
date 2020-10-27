/**
 * Deterministic data for tests, crafted to cover various use cases, whereas mock_data is for
 * visualizing a complete status table via nondeterministically generated tasks/commits.
 */
import {
  Branch,
  GetIncrementalCommitsResponse,
  Comment,
  Task,
  LongCommit,
  GetAutorollerStatusesResponse,
} from '../rpc/status';

function copy<T>(obj: T): T {
  return JSON.parse(JSON.stringify(obj));
}

Date.now = () => 1600883976659;
const timestampBeforeNow = (seconds: number = 0) => {
  return new Date(Date.now() - 1000 * seconds).toISOString();
};

export const branch0: Branch = { name: 'main', head: 'abc123' };
export const branch1: Branch = { name: 'bar', head: '456789' };
export const commentTask: Comment = {
  id: 'foo',
  repo: 'skia',
  timestamp: timestampBeforeNow(300),
  user: 'alison@google.com',
  message: 'this is a comment on a task',
  ignoreFailure: true,
  deleted: false,
  flaky: false,
  taskId: '99999',
  taskSpecName: 'Build-Some-Stuff',
  commit: 'abc123',
};
export const commentCommit: Comment = {
  id: 'foo',
  repo: 'skia',
  timestamp: timestampBeforeNow(300),
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
  timestamp: timestampBeforeNow(300),
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
const task3: Task = {
  commits: ['childofabc123'],
  id: '33333',
  name: 'Build-Some-Stuff',
  revision: 'childofabc123',
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
  timestamp: timestampBeforeNow(300),
};
const commit1: LongCommit = {
  hash: 'parentofabc123',
  author: 'bob@example.com',
  parents: ['grandparentofabc123'],
  subject: '2nd from HEAD',
  body: 'the commit that comes before the most recent commit',
  timestamp: timestampBeforeNow(600),
};
const commit2: LongCommit = {
  hash: 'childofabc123',
  author: 'alice@example.com',
  parents: ['abc123'],
  subject: 'newest commit',
  body: 'newest commit sent via incremental update',
  timestamp: timestampBeforeNow(100),
};
const branchCommit0: LongCommit = {
  hash: '456789',
  author: 'bob@example.com',
  parents: ['10111213'],
  subject: 'branch commit',
  body: 'commit on differnet branch',
  timestamp: timestampBeforeNow(450),
};
export const incrementalResponse0: GetIncrementalCommitsResponse = {
  metadata: { pod: 'podd', startOver: true },
  update: {
    branchHeads: [branch0, branch1],
    comments: [commentCommit, commentTask, commentTaskSpec],
    commits: [
      commit0,
      { ...commit1, parents: ['relandbad'] },
      {
        hash: 'relandbad',
        author: 'alice@example.com',
        parents: ['1revertbad'],
        subject: 'is a reland',
        body: 'This is a reland of bad',
        timestamp: timestampBeforeNow(700),
      },
      {
        // To test handling of leading digits in the selector.
        hash: '1revertbad',
        author: 'bob@example.com',
        parents: ['bad'],
        subject: 'is a revert',
        body: 'This reverts commit bad',
        timestamp: timestampBeforeNow(800),
      },
      {
        hash: 'bad',
        author: 'alice@example.com',
        parents: ['acommitthatisnotlisted'],
        subject: 'get reverted',
        body: 'dereference some null pointers',
        timestamp: timestampBeforeNow(900),
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

export const incrementalResponse1 = (() => {
  const r = copy(incrementalResponse0);
  r.metadata!.startOver = false;
  r.update!.tasks = [
    // New task for a the new commit.
    task3,
    // Existing task, with a revised status.
    { ...task1, status: 'SUCCESS' },
  ];
  r.update!.comments = [];
  r.update!.commits = [commit2];

  return r;
})();

export const resetResponse0 = (() => {
  const r = copy(incrementalResponse1);
  r.metadata!.startOver = true;
  return r;
})();

export const getAutorollerStatusesResponse: GetAutorollerStatusesResponse = {
  rollers: [
    {
      name: 'ANGLE',
      mode: 'running',
      numBehind: 0,
      numFailed: 0,
      currentRollRev: 'abc123',
      lastRollRev: 'def456',
      url: 'https://autoroll.skia.org/r/angle-skia-autoroll',
    },
    {
      name: 'Android',
      mode: 'running',
      numBehind: 0,
      numFailed: 1,
      currentRollRev: 'abc123',
      lastRollRev: 'def456',
      url: 'https://autoroll.skia.org/r/android-skia-autoroll',
    },
    {
      name: 'Chrome',
      mode: 'running',
      numBehind: 0,
      numFailed: 1,
      currentRollRev: 'abc123',
      lastRollRev: 'def456',
      url: 'https://autoroll.skia.org/r/chrome-skia-autoroll',
    },
    {
      name: 'Flutter',
      mode: 'running',
      numBehind: 3,
      numFailed: 6,
      currentRollRev: 'abc123',
      lastRollRev: 'def456',
      url: 'https://autoroll.skia.org/r/flutter-skia-autoroll',
    },
    {
      name: 'Google3',
      mode: 'running',
      numBehind: 0,
      numFailed: 0,
      currentRollRev: 'abc123',
      lastRollRev: 'def456',
      url: 'https://autoroll.skia.org/r/g3-skia-autoroll',
    },
    {
      name: 'SwiftSh',
      mode: 'running',
      numBehind: 0,
      numFailed: 0,
      currentRollRev: 'abc123',
      lastRollRev: 'def456',
      url: 'https://autoroll.skia.org/r/swiftsh-skia-autoroll',
    },
    {
      name: 'skcms',
      mode: 'running',
      numBehind: 0,
      numFailed: 0,
      currentRollRev: 'abc123',
      lastRollRev: 'def456',
      url: 'https://autoroll.skia.org/r/skcms-skia-autoroll',
    },
  ],
};
