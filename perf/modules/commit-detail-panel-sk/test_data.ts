import { Commit, CommitNumber } from '../json';

export const twoCommits: Commit[] = [
  {
    offset: CommitNumber(1),
    hash: 'f481669539277216352de81f3d3212b5f29b3b13',
    author: 'test (test@google.com)',
    message: 'A commit message.',
    url: 'https://ska.googlesource.com/skia/+/f481669539277216352de81f3d3212b5f29b3b13',
    ts: 1618324800,
    body: 'A longer commit message.',
  },
  {
    offset: CommitNumber(2),
    hash: 'a481669539277216352de81f3d3212b5f29b3b1a',
    author: 'test2 (test2@google.com)',
    message: 'Another commit message.',
    url: 'https://ska.googlesource.com/skia/+/a481669539277216352de81f3d3212b5f29b3b1a',
    ts: 1618325800,
    body: 'Another longer commit message.',
  },
];
