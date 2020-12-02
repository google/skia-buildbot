import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';
import { PaginationSk } from './pagination-sk';

const p = document.createElement('pagination-sk') as PaginationSk;
p.pagination = { total: 100, size: 10, offset: 0 };
($$('#container') as HTMLElement).appendChild(p);
