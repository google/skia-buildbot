import './index';
import { JobTriggerSk } from './job-trigger-sk';
import { FakeTaskSchedulerService } from '../rpc-mock';

const ele = <JobTriggerSk>document.querySelector("job-trigger-sk")!;
ele.rpc = new FakeTaskSchedulerService();
