import fetchMock from 'fetch-mock';
import { task2, FakeTaskSchedulerService, fakeNow } from '../rpc-mock';
import './index';
import { TaskSk } from './task-sk';
import '../../../infra-sk/modules/theme-chooser-sk';

// Override the current date to keep puppeteer tests consistent.
Date.now = () => fakeNow;

fetchMock.get(`/json/task-summary/${task2.id}`, {
  errorMessage: 'Line 42: compile error: undefined reference to function',
  analysis: 'Compile failure due to non-existent function.',
});

const ele = <TaskSk>document.querySelector('task-sk')!;
ele.rpc = new FakeTaskSchedulerService();
ele.taskID = task2.id;
