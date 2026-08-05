import { LongCommit } from '../rpc/status';

// 3 of the commits have the same timestamp, 2 of them are out of order, but parents can sort it out.
export const sameTimestamp: Array<LongCommit> = [
  {
    hash: 'abc0',
    author: 'An Example (example@google.com)',
    subject: 'MakeFromYUVATexturesCopyToExternal check texture valid before',
    parents: ['abc1'],
    body: 'body',
    timestamp: '2020-11-05T16:26:42Z',
  },
  {
    hash: 'abc1',
    author: 'An Example (example@google.com)',
    subject: 'Revert "Add memsets to the GrBlockAllocator unit tests."',
    parents: ['abc2'],
    body: 'body',
    timestamp: '2020-11-05T16:17:44Z',
  },
  {
    hash: 'abc3',
    author: 'An Example (example@google.com)',
    subject: 'Revert "move subrun instances and support to .cpp"',
    parents: ['abc4'],
    body: 'body',
    timestamp: '2020-11-05T16:03:33Z',
  },
  {
    hash: 'abc2',
    author: 'An Example (example@google.com)',
    subject: 'Revert "cull glyphs that have far out positions"',
    parents: ['abc3'],
    body: 'body',
    timestamp: '2020-11-05T16:03:33Z',
  },
  {
    hash: 'abc4',
    author: 'An Example (example@google.com)',
    subject: 'Revert "move subrun code to anonymous namespace"',
    parents: ['abc5'],
    body: 'body',
    timestamp: '2020-11-05T16:03:33Z',
  },
  {
    hash: 'abc5',
    author: 'An Example (example@google.com)',
    subject: 'Migrate work from constructors to onOnceBeforeDraw.',
    parents: ['abc6'],
    body: 'body',
    timestamp: '2020-11-05T15:43:42Z',
  },
];
