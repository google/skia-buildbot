import { assert } from 'chai';
import { MISSING_DATA_SENTINEL } from '../const/const';
import {
  CommitNumber,
  DataFrame,
  ReadOnlyParamSet,
  TimestampSeconds,
  Trace,
  TraceSet,
} from '../json';
import { dataframeToCSV } from './index';

const df: DataFrame = {
  header: [
    {
      offset: CommitNumber(0),
      timestamp: TimestampSeconds(1660000000),
      author: '',
      hash: 'hash_1',
      message: '',
      url: '',
    },
    {
      offset: CommitNumber(1),
      timestamp: TimestampSeconds(1660000100),
      author: '',
      hash: 'hash_2',
      message: '',
      url: '',
    },
  ],
  paramset: ReadOnlyParamSet({}),
  traceset: TraceSet({
    ',arch=x86,config=8888,': Trace([90131983.0, 90132647.0]),
    ',arch=arm,config=8888,': Trace([90135000.0, MISSING_DATA_SENTINEL]),
  }),
  skip: 0,
  traceMetadata: [],
};

describe('csv', () => {
  it('exports a single trace', () => {
    const singleTraceDf: DataFrame = {
      ...df,
      traceset: TraceSet({
        ',arch=x86,config=8888,': Trace([90131983.0, 90132647.0]),
      }),
    };

    const expected = `offset,hash,arch=x86&config=8888
0,hash_1,90131983
1,hash_2,90132647`;
    assert.equal(dataframeToCSV(singleTraceDf), expected);
  });

  it('exports multiple traces and empty fields', () => {
    const expected = `offset,hash,arch=x86&config=8888,arch=arm&config=8888
0,hash_1,90131983,90135000
1,hash_2,90132647,`;
    assert.equal(dataframeToCSV(df), expected);
  });

  it('returns empty string for empty dataframe or traceset', () => {
    assert.equal(dataframeToCSV(null as any), '');
    assert.equal(dataframeToCSV({ ...df, traceset: TraceSet({}) }), '');
  });
});
