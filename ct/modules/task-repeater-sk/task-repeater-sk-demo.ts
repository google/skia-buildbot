import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';
import { TaskRepeaterSk } from './task-repeater-sk';

const taskRepeater = document.createElement('task-repeater-sk') as TaskRepeaterSk;
($$('#container') as HTMLElement).appendChild(taskRepeater);
