import './index';
import '../theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';

function newTaskQueue(parentSelector) {
  const si = document.createElement('autogrow-textarea-sk');
  $$(parentSelector).appendChild(si);
}

newTaskQueue('#container');
