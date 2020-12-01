import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';
import { languageList } from './test_data';
import { SuggestInputSk } from './suggest-input-sk';

function newTaskQueue(parentSelector: string) {
  const si = document.createElement('suggest-input-sk') as SuggestInputSk;
  si.options = languageList;
  si.label = 'Select a language';
  ($$(parentSelector) as HTMLElement).appendChild(si);
}

newTaskQueue('#container');
