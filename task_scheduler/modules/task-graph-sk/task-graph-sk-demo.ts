import '../../../elements-sk/modules/error-toast-sk';
import { TaskGraphSk } from './task-graph-sk';
import { job2 } from '../rpc-mock';
import '../../../infra-sk/modules/theme-chooser-sk';

import './index';

const ele = <TaskGraphSk>document.getElementsByTagName('task-graph-sk')[0];
ele.draw([job2]);
