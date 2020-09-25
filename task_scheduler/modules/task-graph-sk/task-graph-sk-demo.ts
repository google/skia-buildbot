import 'elements-sk/error-toast-sk';
import { TaskGraphSk } from './task-graph-sk';
import { job1 } from '../rpc-mock';

import './index.ts';

const ele = <TaskGraphSk>document.getElementsByTagName("task-graph-sk")[0];
ele.draw([job1], "chromium-swarm.appspot.com");
