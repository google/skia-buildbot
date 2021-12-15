import { $$ } from 'common-sk/modules/dom';
import { DataFrame, pivot } from '../json';
import './index';
import { PivotTableSk } from './pivot-table-sk';

const df: DataFrame = {
  header: [],
  paramset: {},
  traceset: {
    ',config=8888,': [1.2345, 1.3e27],
    ',config=gpu,': [0.1234, Math.PI],
  },
  skip: 0,
};
const req: pivot.Request = {
  group_by: ['config'],
  operation: 'avg',
  summary: ['avg', 'sum'],
};

$$<PivotTableSk>('#good')!.set(df, req);
$$<PivotTableSk>('#invalid-pivot')!.set(df, null as unknown as pivot.Request);
$$<PivotTableSk>('#null-df')!.set(null as unknown as DataFrame, req);
