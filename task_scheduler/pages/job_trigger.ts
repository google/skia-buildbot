import '../modules/job-trigger-sk';
import '../modules/task-scheduler-scaffold-sk';
import '../modules/colors.css';

import { GetTaskSchedulerService } from '../modules/rpc';
import { JobTriggerSk } from '../modules/job-trigger-sk/job-trigger-sk';

const ele = <JobTriggerSk>document.querySelector('job-trigger-sk');
ele.rpc = GetTaskSchedulerService(ele);
