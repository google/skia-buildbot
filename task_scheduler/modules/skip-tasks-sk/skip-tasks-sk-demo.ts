import './index';
import { SkipTasksSk } from './skip-tasks-sk';
import { FakeTaskSchedulerService } from '../rpc-mock';

const ele = <SkipTasksSk>document.querySelector('skip-tasks-sk')!;
ele.rpc = new FakeTaskSchedulerService();
