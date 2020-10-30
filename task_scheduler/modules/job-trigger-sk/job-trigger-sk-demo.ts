import './index';
import { JobTriggerSk } from './job-trigger-sk';
import { FakeTaskSchedulerService } from '../rpc-mock';
import '../../../infra-sk/modules/theme-chooser-sk';

const ele = <JobTriggerSk>document.querySelector('job-trigger-sk')!;
ele.rpc = new FakeTaskSchedulerService();
