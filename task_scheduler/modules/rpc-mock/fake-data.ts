import { Job, Task } from '../rpc';
import { TaskStatus, JobStatus, RepoState, SkipTaskRule } from '../rpc/rpc';

// This is an arbitrary date which happens to be after all of the other
// timestamps listed in this file.
export const fakeNow = new Date('2019-11-10T13:56:44.572122663Z').getTime();

export const job1ID = 'aYwjrLWysQRUW2lGFQvR';
export const job2ID = 'bYwjrLWysQRUW2lGFQvX';

export const repoState: RepoState = {
  patch: {
    issue: '193176',
    patchRepo: 'https://skia.googlesource.com/skia.git',
    patchset: '2',
    server: 'https://skia-review.googlesource.com',
  },
  repo: 'https://skia.googlesource.com/skia.git',
  revision: '9883def4f8661f8eec4ccbae2e34d7fcb14bf65d',
};

export const task0: Task = {
  attempt: 0,
  commits: [],
  createdAt: '2019-02-19T13:21:33.565771Z',
  dbModifiedAt: '2019-02-19T13:22:14.946038Z',
  finishedAt: '2019-02-19T13:22:14.669965Z',
  id: 'db0BuaY14LAtgqirQr0h',
  isolatedOutput: 'f43fcadbbffe79a92f5da6792ed992581aa620bd',
  jobs: [job1ID, job2ID],
  maxAttempts: 2,
  parentTaskIds: [],
  properties: {},
  retryOf: '',
  startedAt: '2019-02-19T13:21:33.920761Z',
  status: TaskStatus.TASK_STATUS_SUCCESS,
  swarmingBotId: 'skia-gce-215',
  swarmingTaskId: '431ebc8bdc182810',
  taskKey: {
    repoState: repoState,
    name: 'Housekeeper-PerCommit-BundleRecipes',
    forcedJobId: '',
  },
  stats: {
    totalOverheadS: '30.0',
    downloadOverheadS: '15.0',
    uploadOverheadS: '15.0',
  },
  taskExecutor: 'chromium-swarm.appspot.com',
};

export const task1: Task = {
  attempt: 0,
  commits: [],
  createdAt: '2019-02-19T13:22:33.565771Z',
  dbModifiedAt: '2019-02-19T13:23:14.946038Z',
  finishedAt: '2019-02-19T13:23:14.669965Z',
  id: 'J1iwABakhHfGNzGc0n0G',
  isolatedOutput: 'f43fcadbbffe79a92f5da6792ed992581aa620bd',
  jobs: [job1ID, job2ID],
  maxAttempts: 2,
  parentTaskIds: [task0.id],
  properties: {},
  retryOf: '',
  startedAt: '2019-02-19T13:22:33.920761Z',
  status: TaskStatus.TASK_STATUS_SUCCESS,
  swarmingBotId: 'skia-gce-215',
  swarmingTaskId: '431ebd696ac4fe10',
  taskKey: {
    repoState: repoState,
    name: 'Build-Debian9-EMCC-wasm-Release-CanvasKit',
    forcedJobId: '',
  },
  stats: {
    totalOverheadS: '15.0',
    downloadOverheadS: '5.0',
    uploadOverheadS: '10.0',
  },
  taskExecutor: 'chromium-swarm.appspot.com',
};

export const task2: Task = {
  attempt: 0,
  commits: [],
  createdAt: '2019-02-19T13:25:33.565771Z',
  dbModifiedAt: '2019-02-19T13:27:14.946038Z',
  finishedAt: '2019-02-19T13:27:14.669965Z',
  id: 'QT5J8rNsgnumXH67JwTr',
  isolatedOutput: 'f43fcadbbffe79a92f5da6792ed992581aa620bd',
  jobs: [job1ID],
  maxAttempts: 2,
  parentTaskIds: [task0.id, task1.id],
  properties: {},
  retryOf: '',
  startedAt: '2019-02-19T13:25:33.920761Z',
  status: TaskStatus.TASK_STATUS_FAILURE,
  swarmingBotId: 'skia-gce-215',
  swarmingTaskId: '431ec237d09f5410',
  taskKey: {
    repoState: repoState,
    name: 'Test-Debian9-EMCC-GCE-GPU-AVX2-wasm-Release-All-CanvasKit',
    forcedJobId: '',
  },
  stats: {
    totalOverheadS: '15.0',
    downloadOverheadS: '15.0',
    uploadOverheadS: '0.0',
  },
  taskExecutor: 'chromium-swarm.appspot.com',
};

export const task3: Task = {
  attempt: 0,
  commits: [],
  createdAt: '2019-02-19T13:28:33.565771Z',
  dbModifiedAt: '2019-02-19T13:30:14.946038Z',
  finishedAt: '2019-02-19T13:30:14.669965Z',
  id: 'fmHFVsREalHNMozGW7Pg',
  isolatedOutput: 'f43fcadbbffe79a92f5da6792ed992581aa620bd',
  jobs: [job1ID, job2ID],
  maxAttempts: 2,
  parentTaskIds: [task0.id, task1.id],
  properties: {},
  retryOf: '',
  startedAt: '2019-02-19T13:28:33.920761Z',
  status: TaskStatus.TASK_STATUS_SUCCESS,
  swarmingBotId: 'skia-gce-215',
  swarmingTaskId: '431ec43eb083e010',
  taskKey: {
    repoState: repoState,
    name: 'Test-Debian9-EMCC-GCE-GPU-AVX2-wasm-Release-All-CanvasKit',
    forcedJobId: '',
  },
  stats: {
    totalOverheadS: '25.0',
    downloadOverheadS: '15.0',
    uploadOverheadS: '40.0',
  },
  taskExecutor: 'chromium-swarm.appspot.com',
};

export const task4: Task = {
  attempt: 0,
  commits: [],
  createdAt: '2019-02-19T13:30:33.565771Z',
  dbModifiedAt: '2019-02-19T13:32:14.946038Z',
  finishedAt: '2019-02-19T13:32:14.669965Z',
  id: '6qz24baK8BCl8ubhKo5K',
  isolatedOutput: 'f43fcadbbffe79a92f5da6792ed992581aa620bd',
  jobs: [job1ID, job2ID],
  maxAttempts: 2,
  parentTaskIds: [task0.id, task1.id],
  properties: {},
  retryOf: '',
  startedAt: '2019-02-19T13:30:33.920761Z',
  status: TaskStatus.TASK_STATUS_SUCCESS,
  swarmingBotId: 'skia-gce-215',
  swarmingTaskId: '431ec69823433510',
  taskKey: {
    repoState: repoState,
    name: 'Upload-Test-Debian9-EMCC-GCE-GPU-AVX2-wasm-Release-All-CanvasKit',
    forcedJobId: '',
  },
  stats: {
    totalOverheadS: '25.0',
    downloadOverheadS: '25.0',
    uploadOverheadS: '0.0',
  },
  taskExecutor: 'chromium-swarm.appspot.com',
};

// job1 represents real task data.
export const job1: Job = {
  buildbucketBuildId: '8855126827389415264',
  buildbucketLeaseKey: '',
  createdAt: new Date('2019-02-19T13:20:52.277737Z').toString(),
  dbModifiedAt: new Date('2019-02-19T13:33:14.64704Z').toString(),
  dependencies: [
    {
      task: task1.taskKey!.name,
      dependencies: [task0.taskKey!.name],
    },
    {
      task: task0.taskKey!.name,
      dependencies: [],
    },
    {
      task: task2.taskKey!.name,
      dependencies: [task0.taskKey!.name, task1.taskKey!.name],
    },
    {
      task: task4.taskKey!.name,
      dependencies: [task0.taskKey!.name, task2.taskKey!.name],
    },
  ],
  finishedAt: new Date('2019-02-19T13:32:46.274182Z').toString(),
  id: task2.jobs![0],
  isForce: false,
  name: task2.taskKey!.name,
  priority: '0.8', // TODO: Why is this a string??
  repoState: repoState,
  requestedAt: new Date('2019-02-19T13:20:32.277737Z').toString(),
  status: JobStatus.JOB_STATUS_SUCCESS,
  statusDetails: 'job finished successfully',
  tasks: [
    {
      name: task1.taskKey!.name,
      tasks: [
        {
          attempt: task1.attempt,
          id: task1.id,
          maxAttempts: task1.maxAttempts,
          status: task1.status,
          swarmingTaskId: task1.swarmingTaskId,
        },
      ],
    },
    {
      name: task0.taskKey!.name,
      tasks: [
        {
          attempt: task0.attempt,
          id: task0.id,
          maxAttempts: task0.maxAttempts,
          status: task0.status,
          swarmingTaskId: task0.swarmingTaskId,
        },
      ],
    },
    {
      name: task2.taskKey!.name,
      tasks: [
        {
          attempt: task2.attempt,
          id: task2.id,
          maxAttempts: task2.maxAttempts,
          status: task2.status,
          swarmingTaskId: task2.swarmingTaskId,
        },
        {
          attempt: task3.attempt,
          id: task3.id,
          maxAttempts: task3.maxAttempts,
          status: task3.status,
          swarmingTaskId: task3.swarmingTaskId,
        },
      ],
    },
    {
      name: task4.taskKey!.name,
      tasks: [
        {
          attempt: task4.attempt,
          id: task4.id,
          maxAttempts: task4.maxAttempts,
          status: task4.status,
          swarmingTaskId: task4.swarmingTaskId,
        },
      ],
    },
  ],
  taskSpecSummaries: [
    {
      taskName: task1.taskKey!.name,
      dimensions: ['key:val'],
      taskExecutor: 'chromium-swarm.appspot.com',
    },
    {
      taskName: task0.taskKey!.name,
      dimensions: ['key:val'],
      taskExecutor: 'chromium-swarm.appspot.com',
    },
    {
      taskName: task2.taskKey!.name,
      dimensions: ['key:val'],
      taskExecutor: 'chromium-swarm.appspot.com',
    },
    {
      taskName: task4.taskKey!.name,
      dimensions: ['key:val'],
      taskExecutor: 'chromium-swarm.appspot.com',
    },
  ],
};

// job2 is fake data but is more visually interesting.
export const job2: Job = {
  buildbucketBuildId: '8855126827389415264',
  buildbucketLeaseKey: '',
  createdAt: new Date('2019-10-10T13:56:44.572122663Z').toUTCString(),
  dbModifiedAt: new Date('2019-10-10T19:57:44.572122663Z').toUTCString(),
  dependencies: [
    {
      task: 'F',
      dependencies: ['E'],
    },
    {
      task: 'E',
      dependencies: ['B'],
    },
    {
      task: 'D',
      dependencies: ['B'],
    },
    {
      task: task2.taskKey!.name,
      dependencies: ['A'],
    },
    {
      task: 'B',
      dependencies: ['A'],
    },
    {
      task: 'A',
      dependencies: [],
    },
  ],
  finishedAt: '0001-01-01T00:00:00Z',
  id: job2ID,
  isForce: false,
  name: 'ABCDEF',
  priority: '0.8',
  repoState: repoState,
  requestedAt: new Date('2019-10-10T13:55:44.572122663Z').toUTCString(),
  startedAt: new Date('2019-10-10T13:57:44.572122663Z').toUTCString(),
  status: JobStatus.JOB_STATUS_IN_PROGRESS,
  statusDetails: 'job not finished',
  tasks: [
    {
      name: 'A',
      tasks: [
        {
          id: 'A1',
          attempt: 1,
          maxAttempts: 2,
          status: TaskStatus.TASK_STATUS_SUCCESS,
          swarmingTaskId: '31cd28b854e04d10',
        },
      ],
    },
    {
      name: 'B',
      tasks: [
        {
          id: 'B1',
          attempt: 1,
          maxAttempts: 2,
          status: TaskStatus.TASK_STATUS_FAILURE,
          swarmingTaskId: '31cd28b854e04d10',
        },
        {
          id: 'B2',
          attempt: 2,
          maxAttempts: 2,
          status: TaskStatus.TASK_STATUS_SUCCESS,
          swarmingTaskId: '31cd28b854e04d10',
        },
      ],
    },
    {
      name: task2.taskKey!.name,
      tasks: [
        {
          id: task2.id,
          attempt: 1,
          maxAttempts: 2,
          status: task2.status,
          swarmingTaskId: task2.swarmingTaskId,
        },
      ],
    },
    {
      name: 'D',
      tasks: [
        {
          id: 'D1',
          attempt: 1,
          maxAttempts: 2,
          status: TaskStatus.TASK_STATUS_PENDING,
          swarmingTaskId: '31cd28b854e04d10',
        },
      ],
    },
    {
      name: 'E',
      tasks: [
        {
          id: 'E1',
          attempt: 1,
          maxAttempts: 2,
          status: TaskStatus.TASK_STATUS_RUNNING,
          swarmingTaskId: '31cd28b854e04d10',
        },
      ],
    },
  ],
  taskSpecSummaries: [
    {
      taskName: 'F',
      dimensions: ['key:val'],
      taskExecutor: 'chromium-swarm.appspot.com',
    },
    {
      taskName: 'E',
      dimensions: ['key:val'],
      taskExecutor: 'alternate-swarming-for-E.appspot.com',
    },
    {
      taskName: 'D',
      dimensions: ['key:val'],
      taskExecutor: 'chromium-swarm.appspot.com',
    },
    {
      taskName: task2.taskKey!.name,
      dimensions: ['key:val'],
      taskExecutor: 'chromium-swarm.appspot.com',
    },
    {
      taskName: 'B',
      dimensions: ['key:val'],
      taskExecutor: 'chromium-swarm.appspot.com',
    },
    {
      taskName: 'A',
      dimensions: ['key:val'],
      taskExecutor: 'chromium-swarm.appspot.com',
    },
  ],
};

export const skipRule1: SkipTaskRule = {
  addedBy: 'you@google.com',
  taskSpecPatterns: ['Test-.*', 'Perf-.*'],
  commits: ['abc123'],
  description: 'Skip all test and perf tasks at abc123',
  name: 'No test/perf @ abc',
};

export const skipRule2: SkipTaskRule = {
  addedBy: 'me@google.com',
  commits: ['def456'],
  description: 'Skip everything at def456',
  name: 'def456 is bad',
};

export const skipRule3: SkipTaskRule = {
  addedBy: 'you@google.com',
  taskSpecPatterns: ['BadTask'],
  description: 'Skip all BadTasks at every commit',
  name: 'BadTask is bad!',
};
