import fetchMock from 'fetch-mock';
import { FakeTaskSchedulerService, job2, task2, fakeNow } from '../rpc-mock';
import './index';
import { JobSk } from './job-sk';
import '../../../infra-sk/modules/theme-chooser-sk';

// Override the current date to keep puppeteer tests consistent.
Date.now = () => fakeNow;

fetchMock.get(`/json/task-summary/B1`, {
  errorMessage: 'Step "test" failed on Bot "skia-gce-101".\nExit code: 1\nRun-time: 12m',
  analysis: 'Hardware issue suspected on GCE instance.',
});

fetchMock.get(`/json/task-summary/${task2.id}`, {
  errorMessage:
    'Something went wrong while executing the task in EMCC release run.\nLine 42: compile error: undefined reference to function',
  analysis: 'Compile failure in CanvasKit target.',
});

const ele = <JobSk>document.querySelector('job-sk')!;
ele.rpc = new FakeTaskSchedulerService();
ele.jobID = job2.id;
