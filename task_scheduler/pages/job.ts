import '../modules/job-sk';
import '../modules/task-scheduler-scaffold-sk';
import { JobSk } from '../modules/job-sk/job-sk';
import { GetTaskSchedulerService } from '../modules/rpc';

const ele = <JobSk>document.querySelector('job-sk');
ele.rpc = GetTaskSchedulerService(ele);
