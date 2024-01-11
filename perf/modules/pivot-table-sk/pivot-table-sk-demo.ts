import { $$ } from '../../../infra-sk/modules/dom';
import { DataFrame, ReadOnlyParamSet, Trace, TraceSet, pivot } from '../json';
import './index';
import { PivotTableSk } from './pivot-table-sk';

const df: DataFrame = {
  header: [],
  paramset: ReadOnlyParamSet({}),
  traceset: TraceSet({
    ',arch=x86,config=8888,': Trace([1, 1.3e27]),
    ',arch=arm,config=8888,': Trace([2, 2.3e27]),
    ',arch=x86,config=gpu,': Trace([3, 1.2345]),
    ',arch=arm,config=gpu,': Trace([3, Math.PI]),
  }),
  skip: 0,
};
const req: pivot.Request = {
  group_by: ['config', 'arch'],
  operation: 'avg',
  summary: ['avg', 'sum'],
};
const query = 'config=8888&config=gpu&arch=x86&arch=arm';
$$<PivotTableSk>('#good')!.set(df, req, query);
$$<PivotTableSk>('#invalid-pivot')!.set(
  df,
  null as unknown as pivot.Request,
  query
);
$$<PivotTableSk>('#null-df')!.set(null as unknown as DataFrame, req, query);
