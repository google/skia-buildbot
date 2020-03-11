import './index';
import { $$ } from 'common-sk/modules/dom';

const tasks = [
  {
    TsAdded: 20200121171359,
    RepeatAfterDays: 2,
    TaskDone: true,
    TaskType: 'ChromiumPerf',
    Username: 'westont',
    SwarmingLogs: 'http://go/secretlink',
  },
  {
    TsAdded: 20200121174220,
    RepeatAfterDays: 2,
    TaskDone: true,
    TaskType: 'ChromiumPerf',
    Username: 'westont',
    SwarmingLogs: 'http://go/secretlink',
  },
  {
    TsAdded: 20200121103324,
    TaskType: 'ChromiumPerf',
    Username: 'westont',
    SwarmingLogs: 'http://go/secretlink',
  },
  {
    TsAdded: 20200121221652,
    RepeatAfterDays: 2,
    TaskDone: true,
    TaskType: 'ChromiumPerf',
    Username: 'westont',
    SwarmingLogs: 'http://go/secretlink',
  },
  {
    TsAdded: 20200121104855,
    RepeatAfterDays: 2,
    TaskDone: true,
    TaskType: 'ChromiumPerf',
    Username: 'westont',
    SwarmingLogs: 'http://go/secretlink',
  },
  {
    TsAdded: 20200121102500,
    RepeatAfterDays: 2,
    TaskDone: true,
    TaskType: 'ChromiumPerf',
    Username: 'westont',
    SwarmingLogs: 'http://go/secretlink',
  },
];
const permissions = [{}, {}, { DeleteAllowed: true }, {}, {}, {}];
const ids = [123, 456, 789, 234, 345, 567];
const taskList0 = {
  data: tasks,
  permissions: permissions,
  ids: ids,
};

function newTaskQueue(parentSelector, id, taskList) {
  const tq = document.createElement('task-queue-sk');
  tq.id = id;
  tq.updatePendingTasks(taskList, {});
  tq._render();

  $$(parentSelector).appendChild(tq);
}

newTaskQueue('#task-queue-container', 'tasks', taskList0);
