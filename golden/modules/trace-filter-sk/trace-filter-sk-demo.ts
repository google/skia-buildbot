import './index';
import '../gold-scaffold-sk';
import { $$ } from 'common-sk/modules/dom';
import { TraceFilterSk } from './trace-filter-sk';
import { ParamSet } from 'common-sk/modules/query';

const paramSet: ParamSet = {
  'car make': ['chevrolet', 'dodge', 'ford', 'lincoln motor company'],
  'color': ['blue', 'green', 'red'],
  'used': ['yes', 'no'],
  'year': ['2020', '2019', '2018', '2017', '2016', '2015']
};

const traceFilterSk = new TraceFilterSk();
traceFilterSk.paramSet = paramSet;
traceFilterSk.selection = {'car make': ['dodge', 'ford'], 'color': ['red']};
$$('.container')!.appendChild(traceFilterSk);
