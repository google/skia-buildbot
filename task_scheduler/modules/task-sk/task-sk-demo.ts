import { SetupMocks, task2 } from '../rpc-mock';

SetupMocks();

import './index';
import { TaskSk } from './task-sk';

const ele = <TaskSk>document.querySelector('task-sk')!;
ele.taskID = task2.id;
