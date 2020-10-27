import '../modules/skip-tasks-sk';
import '../modules/task-scheduler-scaffold-sk';
import '../modules/colors.css';
import { SkipTasksSk } from '../modules/skip-tasks-sk/skip-tasks-sk';
import { GetTaskSchedulerService } from '../modules/rpc';

const ele = <SkipTasksSk>document.querySelector('skip-tasks-sk');
ele.rpc = GetTaskSchedulerService(ele);
