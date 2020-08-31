import { Branch, IncrementalUpdate, IncrementalCommitsRequest, IncrementalCommitsResponse, LongCommit, Comment, Task } from '../rpc/statusFe';
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
const randomAuthor = () => ['alice', 'bob', 'charles', 'diane'][Math.floor(Math.random() * 4)];
const taskSpecs = ['Build-Android-Stuff-Metal', 'Build-iOS-Mac15.5', 'Build-iOS-Mac15.5-ASAN', 'Test-Android-Pixel-Swiftshader', 'Test-iOS-Mac15.5-Metal', 'Housekeeper-PerCommit-Small', 'Housekeeper-PerCommit-Medium', 'Housekeeper-PerCommit-Large'];



const commitTemplate = { hash: 'abc0', author: 'bob@example.com', parents: ['abc4'], subject: 'current HEAD', body: 'the most recent commit', timestamp: '34613488' }
const taskTemplate = { commits: ['abc0'], id: '1', name: 'Build-Android-Stuff-Metal', revision: 'whatsarevisioninthiscontext', status: 'SUCCESS', swarmingtaskid: 'idforswarming' };
const commits: Array<LongCommit> = [];
const tasks: Array<Task> = [];
let nextId = 0;
// When we have a task cover multiple commits, we mark the future commits off here, so we don't duplicate the task.
const alreadyFilled: Set<string> = new Set();
// Return true if the key has been encountered, and marks it as encountered.
const seen = (key: string) => { const ret = alreadyFilled.has(key); alreadyFilled.add(key); return ret; }
for(let i = 0; i < 30; i++) {
  const hash = `abc${i}`;
  commits.push(Object.assign(JSON.parse(JSON.stringify(commitTemplate)),
    { hash: hash, parents: [`abc${i + 1}`], subject: 'something', timestamp: (parseInt(commitTemplate.timestamp) - (100 * i)).toString(), author: randomAuthor() })
  );

  for (let spec of taskSpecs) {
    if (!seen(`${hash}/${spec}`)) {
      const hashes = [hash];
      const additionalTasks = [0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 2, 3, 4, 5][Math.floor(Math.random() * 16)];
      for (let n = 0; n < additionalTasks; n++) {
        const earlierTaskHash = `abc${i + 1 + n}`;
        seen(`${earlierTaskHash}/${spec}`);
        hashes.push(earlierTaskHash);
      }
      console.log(hashes);

      const status = ["SUCCESS", "SUCCESS", "SUCCESS", "SUCCESS", "SUCCESS","SUCCESS","SUCCESS","SUCCESS", "FAILURE", "MISHAP", "RUNNING", ""][Math.floor(Math.random() * (i < 4 ? 12 : 10))];
      tasks.push(Object.assign(JSON.parse(JSON.stringify(taskTemplate)),
        { id: nextId++, name: spec, commits: hashes, status: status })
      );
    } else {
      console.log(`Skipping ${hash}/${spec}`);
    }
  }
}
  // Lets make one of our commits on a different branch, to test task splitting behavior.
  commits.splice(4, 0, Object.assign(JSON.parse(JSON.stringify(commitTemplate)),
    { hash: 'def0', parents: [`diffBranch`], subject: 'otherBranchSubject', timestamp: (parseInt(commitTemplate.timestamp) - (100 * 4 -5)).toString(), author: 'branchAuthor'})
  );

export const incrementalResponse0: IncrementalCommitsResponse = {
  metadata: { pod: 'podd' },
  update: {
    branchheads: [branch0, branch1],
    swarmingurl: 'swarmyswarm',
    startover: true,
    taskschedulerurl: 'tasky',
    timestamp: 'timey',
    comments: [commentCommit, commentTask, commentTaskSpec],
    commits: commits,
    tasks: tasks,
  }
};
