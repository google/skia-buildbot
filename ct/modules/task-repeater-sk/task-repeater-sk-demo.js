import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';

const taskRepeater = document.createElement('task-repeater-sk');
$$('#container').appendChild(taskRepeater);
