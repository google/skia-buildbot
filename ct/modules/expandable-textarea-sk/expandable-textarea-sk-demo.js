import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';
import { languageList } from './test_data';

function newTaskQueue(parentSelector) {
  const si = document.createElement('expandable-textarea-sk');
  si.options = languageList;
  si.closed = true;
  si.placeholderText = 'Your text here';
  $$(parentSelector).appendChild(si);
}

newTaskQueue('#container');
