import { Commit } from '../commits-table-sk/commits-table-sk';
import { Branch } from '../rpc/status';
import { } from '../rpc-mock/test_data';

const fixedDate = Date.now();
const timestampSinceFixed = (seconds: number = 0) => new Date(fixedDate - 1000 * seconds).toISOString();
const numCommits = 0;
function nextTimestamp() {
  return timestampSinceFixed(10 * numCommits);
}
const commitTemplate: Commit = {
  hash: 'abc0',
  author: 'alice@example.com',
  parents: ['abc1'],
  subject: 'current HEAD',
  body: 'the most recent commit',
  timestamp: '',
  shortAuthor: '',
  shortHash: '',
  shortSubject: '',
  issue: '',
  patchStorage: '',
  isReland: false,
  isRevert: false,
  ignoreFailure: false,
};

function commit(curr: number, parent: number) {
  return { ...commitTemplate, hash: `abc${curr}`, parents: [`abc${parent}`] };
}

export const singleBranchData: [Array<Branch>, Array<Commit>] = [
  [{ name: 'main', head: 'abc0' }],
  (() => {
    const ret: Array<Commit> = [];
    for (let i = 0; i < 20; ++i) {
      ret.push(commit(i, i + 1));
    }
    return ret;
  })(),
];

export const doubleBranchData: [Array<Branch>, Array<Commit>] = [
  [
    { name: 'main', head: 'abc0' },
    { name: 'branch', head: 'abc1' },
  ],
  (() => {
    const ret: Array<Commit> = [];
    // Evens are main, odds are the branch.
    for (let i = 0; i < 30; i += 2) {
      ret.push(commit(i, i + 2));
      if (i < 6) {
        ret.push(commit(i + 1, i + 3));
      } else if (i === 6) {
        ret.push(commit(i + 1, i + 2));
      }
    }
    return ret;
  })(),
];

export const complexBranchData: [Array<Branch>, Array<Commit>] = [
  [
    { name: 'main', head: 'abc0' },
    { name: 'branch1', head: 'abc1' },
    { name: 'branch2', head: 'abc22' },
    { name: 'branch3', head: 'abc3' },
    { name: 'Fizz Rolled', head: 'abc20' },
    { name: 'Baz Rolled', head: 'abc20' },
  ],
  (() => {
    const ret: Array<Commit> = [];
    // x0 (0, 10, 20, etc) is main branch, x1 is branch1, x2 is branch2, etc.
    for (let i = 0; i < 100; i += 10) {
      // The main branch.
      ret.push(commit(i, i + 10));
      // branch1
      if (i < 80) {
        ret.push(commit(i + 1, i + 11));
      } else if (i === 80) {
        ret.push(commit(i + 1, i + 10));
      }
      // branch2
      if (i < 60 && i > 10) {
        ret.push(commit(i + 2, i + 12));
      } else if (i === 60) {
        ret.push(commit(i + 2, i + 10));
      }
      // branch3
      ret.push(commit(i + 3, i + 13));
    }
    // Have main branch merge in other branches.
    ret[0].parents!.push('abc22', 'abc1');
    // Have random commit in branch1 pull in branch2.
    ret[1].parents!.push('abc22');
    return ret;
  })(),
];
