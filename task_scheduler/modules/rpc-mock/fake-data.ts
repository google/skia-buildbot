import {
  Job,
  Task,
} from '../rpc';
import { TaskStatus, JobStatus } from '../rpc/rpc';

export const task1: Task = {
  attempt: 0,
  commits: [],
  createdAt: "2019-02-19T13:27:33.565771Z",
  dbModifiedAt: "2019-02-19T13:29:14.946038Z",
  finishedAt: "2019-02-19T13:29:14.669965Z",
  id: "QT5J8rNsgnumXH67JwTr",
  isolatedOutput: "f43fcadbbffe79a92f5da6792ed992581aa620bd",
  jobs: [
    "aYwjrLWysQRUW2lGFQvR",
    // This is job2.id.
    "ABCDEF"
  ],
  maxAttempts: 2,
  parentTaskIds: [
    "J1iwABakhHfGNzGc0n0G",
    "db0BuaY14LAtgqirQr0h"
  ],
  properties: {},
  retryOf: "",
  startedAt: "2019-02-19T13:27:33.920761Z",
  status: TaskStatus.TASK_STATUS_FAILURE,
  swarmingBotId: "skia-gce-215",
  swarmingTaskId: "431ec237d09f5410",
  taskKey: {
    repoState: {
      patch: {
        issue: "193176",
        patchRepo: "https://skia.googlesource.com/skia.git",
        patchset: "2",
        server: "https://skia-review.googlesource.com",
      },
    repo: "https://skia.googlesource.com/skia.git",
    revision: "9883def4f8661f8eec4ccbae2e34d7fcb14bf65d",
    },
    name: "Test-Debian9-EMCC-GCE-GPU-AVX2-wasm-Release-All-CanvasKit",
    forcedJobId: ""
  },
};

  // job1 represents real task data.
export const job1: Job = {
  buildbucketBuildId: 8921090193851453000,
  buildbucketLeaseKey: 0,
  createdAt: new Date("2019-02-19T13:20:52.277737Z").toString(),
  dbModifiedAt: new Date("2019-02-19T13:33:14.64704Z").toString(),
  dependencies: [
    {
      task: "Build-Debian9-EMCC-wasm-Release-CanvasKit",
      dependencies: [
        "Housekeeper-PerCommit-BundleRecipes",
      ],
    },
    {
      task: "Housekeeper-PerCommit-BundleRecipes",
      dependencies: [],
    },
    {
      task: task1.taskKey!.name,
      dependencies: [
        "Housekeeper-PerCommit-BundleRecipes",
        "Build-Debian9-EMCC-wasm-Release-CanvasKit"
      ],
    },
    {
      task: "Upload-Test-Debian9-EMCC-GCE-GPU-AVX2-wasm-Release-All-CanvasKit",
      dependencies: [
        "Housekeeper-PerCommit-BundleRecipes",
        task1.taskKey!.name
      ],
    },
  ],
  finishedAt: new Date("2019-02-19T13:32:46.274182Z").toString(),
  id: task1.jobs![0],
  isForce: false,
  name: "Test-Debian9-EMCC-GCE-GPU-AVX2-wasm-Release-All-CanvasKit",
  priority: "0.8", // TODO: Why is this a string??
  repoState: {
    patch: {
      issue: "193176",
      patchRepo: "https://skia.googlesource.com/skia.git",
      patchset: "2",
      server: "https://skia-review.googlesource.com",
    },
    repo: "https://skia.googlesource.com/skia.git",
    revision: "9883def4f8661f8eec4ccbae2e34d7fcb14bf65d",
  },
  status: JobStatus.JOB_STATUS_SUCCESS,
  tasks: [
    {
      name: "Build-Debian9-EMCC-wasm-Release-CanvasKit",
      tasks: [
        {
          attempt: 0,
          id: "J1iwABakhHfGNzGc0n0G",
          maxAttempts: 2,
          status: TaskStatus.TASK_STATUS_SUCCESS,
          swarmingTaskId: "431ebd696ac4fe10"
        },
      ],
    },
    {
      name: "Housekeeper-PerCommit-BundleRecipes",
      tasks: [
        {
          attempt: 0,
          id: "db0BuaY14LAtgqirQr0h",
          maxAttempts: 2,
          status: TaskStatus.TASK_STATUS_SUCCESS,
          swarmingTaskId: "431ebc8bdc182810"
        }
      ],
    },
    {
      name: task1.taskKey!.name,
      tasks: [
        {
          attempt: task1.attempt,
          id: task1.id,
          maxAttempts: task1.maxAttempts,
          status: task1.status,
          swarmingTaskId: task1.swarmingTaskId
        },
        {
          attempt: 1,
          id: "fmHFVsREalHNMozGW7Pg",
          maxAttempts: 2,
          status: TaskStatus.TASK_STATUS_SUCCESS,
          swarmingTaskId: "431ec43eb083e010"
        }
      ],
    },
    {
      name: "Upload-Test-Debian9-EMCC-GCE-GPU-AVX2-wasm-Release-All-CanvasKit",
      tasks: [
        {
          attempt: 0,
          id: "6qz24baK8BCl8ubhKo5K",
          maxAttempts: 2,
          status: TaskStatus.TASK_STATUS_SUCCESS,
          swarmingTaskId: "431ec69823433510"
        }
      ],
    },
  ],
  taskDimensions: [
    {
      taskName: "Build-Debian9-EMCC-wasm-Release-CanvasKit",
      dimensions: ["key:val"],
    },
    {
      taskName: "Housekeeper-PerCommit-BundleRecipes",
      dimensions: ["key:val"],
    },
    {
      taskName: task1.taskKey!.name,
      dimensions: ["key:val"],
    },
    {
      taskName: "Upload-Test-Debian9-EMCC-GCE-GPU-AVX2-wasm-Release-All-CanvasKit",
      dimensions: ["key:val"],
    },
  ]
};

// job2 is fake data but is more visually interesting.
export const job2: Job = { // TODO
  buildbucketBuildId: 8921090193851453000,
  buildbucketLeaseKey: 0,
  createdAt: new Date("2016-10-10T13:56:44.572122663Z").toUTCString(),
  dbModifiedAt: new Date("2016-10-10T19:56:44.572122663Z").toString(),
  dependencies: [
    {
      task: "F",
      dependencies: ["E"],
    },
    {
      task: "E",
      dependencies: ["B"],
    },
    {
      task: "D",
      dependencies: ["B"],
    },
    {
      task: task1.taskKey!.name,
      dependencies: ["A"],
    },
    {
      task: "B",
      dependencies: ["A"],
    },
    {
      task: "A",
      dependencies: [],
    },
  ],
  finishedAt: "",
  id: "ABCDEF",
  isForce: false,
  name: "ABCDEF",
  priority: "0.8",
  repoState: {
    patch: {
      issue: "2410843002",
      patchset: "1",
      patchRepo: "https://skia.googlesource.com/skia.git",
      server: "https://codereview.chromium.org",
    },
    repo: "https://skia.googlesource.com/skia.git",
    revision: "6ca48820407244bbdeb8f9e0ed7d76dd94270460",
  },
  status: JobStatus.JOB_STATUS_IN_PROGRESS,
  tasks: [
    {
      name: "A",
      tasks: [{
        id: "A1",
        attempt: 1,
        maxAttempts: 2,
        status: TaskStatus.TASK_STATUS_SUCCESS,
        swarmingTaskId: "31cd28b854e04d10",
      }],
    },
    {
      name: "B",
      tasks: [{
        id: "B1",
        attempt: 1,
        maxAttempts: 2,
        status: TaskStatus.TASK_STATUS_FAILURE,
        swarmingTaskId: "31cd28b854e04d10",
      }, {
        id: "B2",
        attempt: 2,
        maxAttempts: 2,
        status: TaskStatus.TASK_STATUS_SUCCESS,
        swarmingTaskId: "31cd28b854e04d10",
      }],
    },
    {
      name: task1.taskKey!.name,
      tasks: [
        {
          id: task1.id,
          attempt: 1,
          maxAttempts: 2,
          status: task1.status,
          swarmingTaskId: task1.swarmingTaskId,
        },
      ],
    },
    {
      name: "D",
      tasks: [{
        id: "D1",
        attempt: 1,
        maxAttempts: 2,
        status: TaskStatus.TASK_STATUS_PENDING,
        swarmingTaskId: "31cd28b854e04d10",
      }],
    },
    {
      name: "E",
      tasks: [{
        id: "E1",
        attempt: 1,
        maxAttempts: 2,
        status: TaskStatus.TASK_STATUS_RUNNING,
        swarmingTaskId: "31cd28b854e04d10",
      }],
    },
  ],
};