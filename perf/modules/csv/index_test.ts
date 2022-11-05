import { assert } from 'chai';
import { DataFrame } from '../json';
import { dataFrameToCSV } from './index';

const df: DataFrame = {
  header: [
    {
      offset: 0,
      timestamp: 1660000000,
    },
    {
      offset: 1,
      timestamp: 1660000100,
    },
  ],
  paramset: {},
  traceset: {
    ',arch=x86,config=8888,': [1, 1.3e27],
    ',arch=arm,config=8888,': [2, 2.3e27],
    ',arch=x86,config=gpu,': [3, 1.2345],
    ',arch=arm,config=gpu,': [3, Math.PI],
    ',arch=riscv,os=linux,': [3, NaN],
    ',arch=riscv,os=win,': [-NaN, 4],
  },
  skip: 0,
};

describe('csv', () => {
  it('builds csv file from DataFrame', () => {
    const expected = `arch,config,os,2022-08-08T23:06:40.000Z,2022-08-08T23:08:20.000Z
x86,8888,,1,1.3e+27
arm,8888,,2,2.3e+27
x86,gpu,,3,1.2345
arm,gpu,,3,3.141592653589793
riscv,,linux,3,
riscv,,win,,4`;
    assert.equal(dataFrameToCSV(df), expected);
  });
});
