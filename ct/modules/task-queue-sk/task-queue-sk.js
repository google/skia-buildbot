/**
 * @fileoverview Description of this file.
 */

import '../../res/common/js/common'
import '../../res/js/core'
import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/cancel-icon-sk';
import 'elements-sk/icon/check-circle-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import '../../../infra-sk/modules/confirm-dialog-sk';

import {$} from 'common-sk/modules/dom';
import {define} from 'elements-sk/define';
import {html} from 'lit-html';

import {ElementSk,} from '../../../infra-sk/modules/ElementSk';
import {ctfe} from '../../res/js/ctfe'

const template = (el) => html`
 <table class="queue" id="queue">
      <tr class="headers">
        <td>Queue Position</td>
        <td>Added</td>
        <td>Task Type</td>
        <td>User</td>
        <td>Swarming Logs</td>
        <td>Request</td>
      </tr>
      ${el._pendingTasks.map((task, index) => taskRowTemplate(el, task, index))}
 </table>
${el._pendingTasks.map((task, index) => taskDetailDialogTemplate(task, index))}
<confirm-dialog-sk id="confirm_dialog"></confirm-dialog-sk>

`;

const taskRowTemplate = (el, task, index) => html`<tr>
<td class="nowrap"><span>${index + 1} </span>
  <delete-icon-sk alt="Delete" ?hidden=${!task.canDelete}
  @click=${() => {
  el.confirmDeleteTask(index);
}}></delete-icon-sk></td>
<td>${ctfe.getFormattedTimestamp(task.TsAdded)}${
    task.FutureDate ? html`</br>
  <div style="color:red;">(scheduled in the future)</div>` :
                      html``}</td>
<td>${task.TaskType}</td>
<td>${task.Username}</td>
<td class="nowrap">${
    task.FutureDate ?
        html`N/A` :
        task.SwarmingLogs ?
        html`<a href="${task.SwarmingLogs}" target="_blank">Swarming Logs</a>` :
        html`No Swarming Logs`}</td>
<td class="nowrap">
  <a href="javascript:void(0)"
    @click=${(e) => {
  showDetailsDialog(index);
}}>Task Details</a>
</td>
</tr>`;

const taskDetailDialogTemplate = (task, index) => html`
<div id="${'detailsDialog' + index}" class="dialog-background" @click=${(e) => {
  hideDialog(e);
}}>
 <div class="dialog-content">
   <pre>${formatTask(task)}</pre>
 </div>
</div>
`;

define('task-queue-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._pendingTasks = [];

    this.taskDescriptors = [
      {
        type: 'ChromiumPerf',
        get_url: '/_/get_chromium_perf_tasks',
        delete_url: '/_/delete_chromium_perf_task'
      },
      {
        type: 'ChromiumAnalysis',
        get_url: '/_/get_chromium_analysis_tasks',
        delete_url: '/_/delete_chromium_analysis_task'
      },
      {
        type: 'MetricsAnalysis',
        get_url: '/_/get_metrics_analysis_tasks',
        delete_url: '/_/delete_metrics_analysis_task'
      },
      {
        type: 'CaptureSkps',
        get_url: '/_/get_capture_skp_tasks',
        delete_url: '/_/delete_capture_skps_task'
      },
      {
        type: 'LuaScript',
        get_url: '/_/get_lua_script_tasks',
        delete_url: '/_/delete_lua_script_task'
      },
      {
        type: 'ChromiumBuild',
        get_url: '/_/get_chromium_build_tasks',
        delete_url: '/_/delete_chromium_build_task'
      },
      {
        type: 'RecreatePageSets',
        get_url: '/_/get_recreate_page_sets_tasks',
        delete_url: '/_/delete_recreate_page_sets_task'
      },
      {
        type: 'RecreateWebpageArchives',
        get_url: '/_/get_recreate_webpage_archives_tasks',
        delete_url: '/_/delete_recreate_webpage_archives_task'
      },
    ];
  }
  connectedCallback() {
    console.log('connectedCallback fired');
    super.connectedCallback();
    this.loadTaskQueue().then(() => {
      this._render();
    });
  }

  // Dispatch requests to fetch tasks in queue. Returns a promise that resolves
  // when all task data fetching/updating is complete.
  loadTaskQueue() {
    console.log('Reseting Pending Tasks.');
    this._pendingTasks = [];
    var queryParams = {
      'size': 100,
      'not_completed': true,
    };
    var queryStr = '?' + sk.query.fromObject(queryParams);
    var allPromises = [];
    this.taskDescriptors.forEach(function(obj) {
      console.log('Sending a not_completed request to ', obj.get_url);
      allPromises.push(sk.post(obj.get_url + queryStr)
                           .then(JSON.parse)
                           .then(function(json) {
                             this.updatePendingTasks(json, obj);
                           }.bind(this))
                           .catch(sk.errorMessage));
    }.bind(this));

    // Find all tasks scheduled in the future.
    var queryParams = {
      'include_future_runs': true,
    } var queryStr = '?' + sk.query.fromObject(queryParams);
    this.taskDescriptors.forEach(function(obj) {
      console.log('Sending a future_runs request to ', obj.get_url);
      allPromises.push(sk.post(obj.get_url + queryStr)
                           .then(JSON.parse)
                           .then(function(json) {
                             this.updatePendingTasks(json, obj);
                           }.bind(this))
                           .catch(sk.errorMessagea));
    }.bind(this));
    console.log('waiting on some promises: ', allPromises.length);
    return Promise.all(allPromises);
  }

  // Add responses to pending tasks list.
  updatePendingTasks(json, taskDescriptor) {
    var tasks = json.data;
    for (var i = 0; i < tasks.length; i++) {
      console.log('weston: have a task we\'re adding!');
      var task = tasks[i];
      task['canDelete'] = json.permissions[i].DeleteAllowed;
      task['Id'] = json.ids[i];
      task['TaskType'] = taskDescriptor.type;
      task['GetURL'] = taskDescriptor.get_url;
      task['DeleteURL'] = taskDescriptor.delete_url;
      // Check if this is a completed task set to repeat.
      if (task['RepeatAfterDays'] != 0 && task['TaskDone']) {
        // Calculate the future date.
        var timestamp = ctfe.getTimestamp(task['TsAdded']);
        timestamp.setDate(timestamp.getDate() + task['RepeatAfterDays']);
        task['FutureDate'] = true;
        task['TsAdded'] = 20200125120000;
        task['TsAdded'] = ctfe.getCtDbTimestamp(new Date(timestamp));
      }
    }
    this._pendingTasks = this._pendingTasks
                             .concat(tasks)
                         // Sort pending tasks according to TsAdded.
                         this._pendingTasks.sort(function(a, b) {
                           return a['TsAdded'] - b['TsAdded']
                         });
  }


  confirmDeleteTask(index) {
    document.getElementById('confirm_dialog')
        .open('Proceed with deleting task?')
        .then(() => {
          this.deleteTask(index);
          console.log('the deed is done');
        })
        .catch(() => {});
  }
  deleteTask(index) {
    var pendingTask = this._pendingTasks[index];
    var params = {};
    params['id'] = pendingTask.Id;
    sk.post(pendingTask.DeleteURL, JSON.stringify(params))
        .then(function() {
          $$$('#confirm_toast').text =
              'Deleted ' + pendingTask.TaskType + ' task ' + pendingTask.Id;
          $$$('#confirm_toast').show();
        }.bind(this))
        .catch(sk.errorMessage)
        .then(function() {
          this.connectedCallback();
        }.bind(this));
  }
});


function showDetailsDialog(index) {
  document.getElementById('detailsDialog' + index).style.display = 'block';
}

function hideDialog(e) {
  console.log('in the div we want?: ', e.target.classList);
  if (e.target.classList.contains('dialog-background')) {
    e.target.style.display = 'none';
  }
}

function formatTask(task) {
  return JSON.stringify(task, null, 4);
}
