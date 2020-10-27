import '../modules/task-scheduler-scaffold-sk';
import '../modules/task-sk';
import '../modules/colors.css';
import { TaskSk } from '../modules/task-sk/task-sk';
import { GetTaskSchedulerService } from '../modules/rpc';

const ele = <TaskSk>document.querySelector('task-sk');
ele.rpc = GetTaskSchedulerService(ele);
ele.taskID = '{{.TaskId}}';
