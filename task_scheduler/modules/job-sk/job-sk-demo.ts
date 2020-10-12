import { FakeTaskSchedulerService, job2 } from '../rpc-mock';
import './index';
import { JobSk } from './job-sk';

const ele = <JobSk>document.querySelector('job-sk')!;
ele.rpc = new FakeTaskSchedulerService();
ele.jobID = job2.id;
