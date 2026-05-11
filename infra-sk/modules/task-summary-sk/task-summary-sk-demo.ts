import './index';
import { TaskSummarySk } from './task-summary-sk';
import { taskSummaryData } from './test_data';

const ele = document.getElementById('ele') as TaskSummarySk;
ele.data = taskSummaryData;
