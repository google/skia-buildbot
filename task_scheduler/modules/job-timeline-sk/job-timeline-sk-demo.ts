import './index';
import { JobTimelineSk } from './job-timeline-sk';
import { job1, task0, task1, task2, task3, task4 } from '../rpc-mock';

const ele = <JobTimelineSk>document.querySelector('job-timeline-sk')!;
ele.draw(job1, [task0, task1, task2, task3, task4], []);
