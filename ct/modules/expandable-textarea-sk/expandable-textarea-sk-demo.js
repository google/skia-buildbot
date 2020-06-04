import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';

function newTaskQueue(parentSelector) {
  const si = document.createElement('expandable-textarea-sk');
  si.closed = true;
  si.placeholderText = 'Your text here';
  si.displayText = 'Toggle this textbox';
  $$(parentSelector).appendChild(si);
}

newTaskQueue('#container');
