import './index';
import '../gold-scaffold-sk';
import { $$ } from 'common-sk/modules/dom';
import { TraceFilterSk } from './trace-filter-sk';

const traceFilterSk = new TraceFilterSk();
traceFilterSk.paramSet = {
  'car make': ['chevrolet', 'dodge', 'ford', 'lincoln motor company'],
  'color': ['blue', 'green', 'red'],
  'used': ['yes', 'no'],
  'year': ['2020', '2019', '2018', '2017', '2016', '2015']
};;
traceFilterSk.selection = {'car make': ['dodge', 'ford'], 'color': ['red']};
$$('.container')!.appendChild(traceFilterSk);
