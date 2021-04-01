import './index';
import { TaskDriverSk } from './task-driver-sk';
import { taskDriverData } from './test_data';

const ele = document.getElementById('ele') as TaskDriverSk;
ele.data = taskDriverData;
