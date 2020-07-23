import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';

function newTaskQueue(parentSelector) {
  const p = document.createElement('pagination-sk');
  p.pagination = { total: 100, size: 10, offset: 0 };
  $$(parentSelector).appendChild(p);
}

newTaskQueue('#container');
