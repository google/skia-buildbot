import 'elements-sk/error-toast-sk';
import { TaskGraphSk } from './task-graph-sk';
import { job2 } from '../rpc-mock';

import './index';

const ele = <TaskGraphSk>document.getElementsByTagName("task-graph-sk")[0];
ele.draw([job2], "chromium-swarm.appspot.com");
