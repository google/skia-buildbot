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
import { dataFrameToCSV } from './index';

const df: DataFrame = {
  header: [
    {
      offset: CommitNumber(0),
      timestamp: TimestampSeconds(1660000000),
    },
    {
      offset: CommitNumber(1),
      timestamp: TimestampSeconds(1660000100),
    },
  ],
  paramset: ReadOnlyParamSet({}),
  traceset: TraceSet({
    ',arch=x86,config=8888,': Trace([1, 1.3e27]),
    ',arch=arm,config=8888,': Trace([2, 2.3e27]),
    ',arch=x86,config=gpu,': Trace([3, MISSING_DATA_SENTINEL]),
    ',arch=arm,config=gpu,': Trace([3, Math.PI]),
    ',arch=riscv,os=linux,': Trace([3, MISSING_DATA_SENTINEL]),
    ',arch=riscv,os=win,': Trace([MISSING_DATA_SENTINEL, 4]),
  }),
  skip: 0,
};

describe('csv', () => {
  it('builds csv file from DataFrame', () => {
    const expected = `arch,config,os,2022-08-08T23:06:40.000Z,2022-08-08T23:08:20.000Z
x86,8888,,1,1.3e+27
arm,8888,,2,2.3e+27
x86,gpu,,3,
arm,gpu,,3,3.141592653589793
riscv,,linux,3,
riscv,,win,,4`;
    assert.equal(dataFrameToCSV(df), expected);
  });
});
