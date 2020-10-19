import './index';

import { FakeTaskSchedulerService } from '../rpc-mock';
import { JobSearchSk } from './job-search-sk';

const ele = <JobSearchSk>document.querySelector('job-search-sk')!;
ele.rpc = new FakeTaskSchedulerService();
