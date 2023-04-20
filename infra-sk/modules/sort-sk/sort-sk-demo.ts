import './index';
import { $$ } from '../dom';
import { SortSk } from './sort-sk';

const sortSk = $$('#as_table')! as SortSk;
sortSk.sort('name', 'up', true);
