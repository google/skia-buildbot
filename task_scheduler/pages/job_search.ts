import '../modules/job-search-sk';
import '../modules/task-scheduler-scaffold-sk';
import '../modules/colors.css';

import { GetTaskSchedulerService } from '../modules/rpc';
import { JobSearchSk } from '../modules/job-search-sk/job-search-sk';

const ele = <JobSearchSk>document.querySelector('job-search-sk');
ele.rpc = GetTaskSchedulerService(ele);
