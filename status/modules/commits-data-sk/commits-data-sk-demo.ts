import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';

const taskRepeater = document.createElement('commits-data-sk');
($$('#container') as HTMLElement).appendChild(taskRepeater);
