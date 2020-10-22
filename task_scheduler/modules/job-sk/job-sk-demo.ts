import { FakeTaskSchedulerService, job2, fakeNow } from '../rpc-mock';
import './index';
import { JobSk } from './job-sk';

// Override the current date to keep puppeteer tests consistent.
Date.now = () => fakeNow;

const ele = <JobSk>document.querySelector('job-sk')!;
ele.rpc = new FakeTaskSchedulerService();
ele.jobID = job2.id;
