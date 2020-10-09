import { task2, FakeTaskSchedulerService } from '../rpc-mock';
import './index';
import { TaskSk } from './task-sk';

const ele = <TaskSk>document.querySelector('task-sk')!;
ele.rpc = new FakeTaskSchedulerService();
ele.taskID = task2.id;
