import './index.js';
import {$$} from 'common-sk/modules/dom';
// import { sk } from '../../../res/js/common';

const tasks = [
  {
    'TsAdded': 20200121171359,
    'RepeatAfterDays': 2,
    'TaskDone': true,
    'TaskType': 'ChromiumPerf',
    'Username': 'westont',
    'SwarmingLogs': 'http://go/secretlink'
  },
  {
    'TsAdded': 20200121174220,
    'RepeatAfterDays': 2,
    'TaskDone': true,
    'TaskType': 'ChromiumPerf',
    'Username': 'westont',
    'SwarmingLogs': 'http://go/secretlink'
  },
  {
    'TsAdded': 20200121103324,
    'TaskType': 'ChromiumPerf',
    'Username': 'westont',
    'SwarmingLogs': 'http://go/secretlink'
  },
  {
    'TsAdded': 20200121221652,
    'RepeatAfterDays': 2,
    'TaskDone': true,
    'TaskType': 'ChromiumPerf',
    'Username': 'westont',
    'SwarmingLogs': 'http://go/secretlink'
  },
  {
    'TsAdded': 20200121104855,
    'RepeatAfterDays': 2,
    'TaskDone': true,
    'TaskType': 'ChromiumPerf',
    'Username': 'westont',
    'SwarmingLogs': 'http://go/secretlink'
  },
  {
    'TsAdded': 20200121102500,
    'RepeatAfterDays': 2,
    'TaskDone': true,
    'TaskType': 'ChromiumPerf',
    'Username': 'westont',
    'SwarmingLogs': 'http://go/secretlink'
  }
];
const permissions = [{}, {}, {'DeleteAllowed': true}, {}, {}, {}];
const ids = [123, 456, 789, 234, 345, 567];
const taskResponse = {
  'data': tasks,
  'permissions': permissions,
  'ids': ids
};

function newTaskQueue(parentSelector, id, taskList, issue, test) {
  const tq = document.createElement('task-queue-sk');
  tq.id = id;
  tq.updatePendingTasks(taskResponse, {});
  tq._render();
  //  tq.digests = digests;
  //  tq.issue = issue;
  //  tq.test = test;

  $$(parentSelector).appendChild(tq);
  //$$(parentSelector).innerHTML += 'testing js test code';
  //$$(parentSelector).innerHTML += JSON.stringify(obj);
}

newTaskQueue('#task-queue-container', 'tasks', tasks, '123456', 'My-Test');
