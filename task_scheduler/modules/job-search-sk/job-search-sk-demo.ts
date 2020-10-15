import './index';

import { task2, FakeTaskSchedulerService } from '../rpc-mock';
import './index';
import { JobSearchSk } from './job-search-sk';

const ele = <JobSearchSk>document.querySelector('job-search-sk')!;
ele.rpc = new FakeTaskSchedulerService();
