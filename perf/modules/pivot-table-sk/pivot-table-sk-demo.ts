import { $$ } from 'common-sk/modules/dom';
import { DataFrame, pivot } from '../json';
import './index';
import { PivotTableSk } from './pivot-table-sk';

const df: DataFrame = {
  header: [],
  paramset: {},
  traceset: {
    ',arch=x86,config=8888,': [1, 1.3e27],
    ',arch=arm,config=8888,': [2, 2.3e27],
    ',arch=x86,config=gpu,': [3, 1.2345],
    ',arch=arm,config=gpu,': [3, Math.PI],
  },
  skip: 0,
};
const req: pivot.Request = {
  group_by: ['config', 'arch'],
  operation: 'avg',
  summary: ['avg', 'sum'],
};

$$<PivotTableSk>('#good')!.set(df, req);
$$<PivotTableSk>('#invalid-pivot')!.set(df, null as unknown as pivot.Request);
$$<PivotTableSk>('#null-df')!.set(null as unknown as DataFrame, req);
