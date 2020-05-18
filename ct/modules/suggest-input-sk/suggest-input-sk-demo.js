import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';

function newTaskQueue(parentSelector) {
  const si = document.createElement('suggest-input-sk');
  $$(parentSelector).appendChild(si);
}

newTaskQueue('#container');
