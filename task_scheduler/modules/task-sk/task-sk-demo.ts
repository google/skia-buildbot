import { task2, FakeTaskSchedulerService, fakeNow } from '../rpc-mock';
import './index';
import { TaskSk } from './task-sk';

// Override the current date to keep puppeteer tests consistent.
Date.now = () => fakeNow;

const ele = <TaskSk>document.querySelector('task-sk')!;
ele.rpc = new FakeTaskSchedulerService();
ele.taskID = task2.id;
