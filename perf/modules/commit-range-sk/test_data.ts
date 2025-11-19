import { CommitNumber, TimestampSeconds } from '../json';
import { CommitRangeSk } from './commit-range-sk';

export const commitRangeSK = {
  commitIndex: 2,
  trace: [1, 2, 3],
  header: [
    {
      offset: CommitNumber(64809),
      timestamp: TimestampSeconds(0),
      author: '',
      hash: '',
      message: '',
      url:
        'https://chromium.googlesource.com/' +
        'chromium/src/+show/36e1d589c4586587458c8b153bad026d1dba088e',
    },
    {
      offset: CommitNumber(64810),
      timestamp: TimestampSeconds(0),
      author: '',
      hash: '',
      message: '',
      url: '',
    },
    {
      offset: CommitNumber(64811),
      timestamp: TimestampSeconds(0),
      author: '',
      hash: '',
      message: '',
      url: '',
    },
  ],
} as CommitRangeSk;
