import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';
import { languageList } from './test_data';

function newTaskQueue(parentSelector) {
  const si = document.createElement('suggest-input-sk');
  si.options = languageList;
  si.label = 'Select a language';
  $$(parentSelector).appendChild(si);
}

newTaskQueue('#container');
