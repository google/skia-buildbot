import './index';
import { SkipTasksSk } from './skip-tasks-sk';
import { FakeTaskSchedulerService } from '../rpc-mock';
import '../../../infra-sk/modules/theme-chooser-sk';

const ele = <SkipTasksSk>document.querySelector('skip-tasks-sk')!;
ele.rpc = new FakeTaskSchedulerService();
