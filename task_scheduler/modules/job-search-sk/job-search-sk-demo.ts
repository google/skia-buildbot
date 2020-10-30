import './index';

import { FakeTaskSchedulerService, fakeNow } from '../rpc-mock';
import { JobSearchSk } from './job-search-sk';
import '../../../infra-sk/modules/theme-chooser-sk';

// Override the current date to keep puppeteer tests consistent.
Date.now = () => fakeNow;

const ele = <JobSearchSk>document.querySelector('job-search-sk')!;
ele.rpc = new FakeTaskSchedulerService();
