import './index';
import 'elements-sk/styles/buttons';
import { $$ } from 'common-sk/modules/dom';
import { SortSk } from './sort-sk';

const sortSk = $$('#as_table')! as SortSk;
sortSk.sort('name', 'up', true);
