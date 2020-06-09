/**
 * @fileoverview A custom element that loads the CT pending tasks queue and
 * displays it as a table.
 * 
 * @attr {string} patchType - Specifies the project for the patch. Must be
 * set. Supported values include "chromium" and "skia".
 */

import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/cancel-icon-sk';
import 'elements-sk/icon/check-circle-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/toast-sk';
import '../../../infra-sk/modules/confirm-dialog-sk';
import '../../../infra-sk/modules/expandable-textarea-sk';

import { $$, DomReady } from 'common-sk/modules/dom';
import { fromObject } from 'common-sk/modules/query';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { html } from 'lit-html';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../input-sk';

const template = (ele) => html`
<style>
  .simple-patch {
    display: flex;
    justify-content: space-around;
    flex-direction: row;
    justify-content: flex-start;
  }
  table {
    border: none;
    width: 100%;
  }
  td {
    vertical-align: middle;
  }
  .patch-manual {
    padding-left: 50px;
  }

</style>
<table>
  <tr>
    <td>CL:</td>
    <td><input-sk @input=${ele._clChanged} label="Please paste a complete Gerrit URL"></input-sk></td>
    <td><div class=patch-details>CL Details</div></td>
  </tr>
  <tr>
    <td colspan=3 class=patch-manual>
      <expandable-textarea-sk displaytext="Specify Patch Manually"></expandable-textarea-sk>
    </td>
  </tr>
</table>
`;

define('patch-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._pendingTasks = [];
    this._running = false;
  }

  connectedCallback() {
    super.connectedCallback();

    if (this._running) {
      return;
    }
    this._running = true;
    // We wait for everything to load so scaffolding event handlers are
    // attached.
    DomReady.then(() => {
      this.dispatchEvent(new CustomEvent('begin-task', { bubbles: true }));
      this._render();
      this.loadTaskQueue().then(() => {
        this._render();
        this._running = false;
        this.dispatchEvent(new CustomEvent('end-task', { bubbles: true }));
      });
    });
  }

  _clChanged(e) {
    const newValue = e.target.value;
    if (!newValue || newValue.length < 3) {
      this._clData = null;
      this._loadingClDetail = false;
      return;
    }
    this.loadingClDetail = true;
    const queryParams = { cl: newValue };
    const url = '/_/cl_data?' + `${fromObject(queryParams)}`;

    fetch(url, { method: 'POST' })
      .then(jsonOrThrow)
      .then((json) => {
        if (this.cl === newValue) {
          if (json.cl) {
            this.clData = json;
            const patch = this.clData[`${this.patchType}_patch`];
            if (!patch) {
              this.clData = { error: { response: `This is not a ${this.patchType} CL.` } };
            } else {
              this.patch = patch;
            }
          } else {
            this.clData = null;
          }
          this.loadingClDetail = false;
        }
      })
      .catch((err) => {
        if (this.cl === newValue) {
          this.clData = { error: err };
          this.loadingClDetail = false;
        }
      });

    sk.post(`/_/cl_data?${  sk.query.fromObject(params)}`).then(JSON.parse).then(function (json) {
      
    }.bind(this)).catch(function (err) {
    }.bind(this));
  }

  /**
   * @prop {string} patchType - Specifies the project for the patch. Must be set. Supported values include
      "chromium" and "skia". Mirrors the attribute.
   */
  get patchType() {
    return this.getAttribute('patchType');
  }

  set patchType(val) {
    this.setAttribute('patchType', val);
  }

  showDetailsDialog(index) {
    $$(`#detailsDialog${index}`, this).classList.remove('hidden');
  }

  // Dispatch requests to fetch tasks in queue. Returns a promise that resolves
  // when all task data fetching/updating is complete.
  loadTaskQueue() {
    this._pendingTasks = [];
    let queryParams = {
      size: 100,
      not_completed: true,
    };
    let queryStr = `?${fromObject(queryParams)}`;
    const allPromises = [];
    for (const obj of taskDescriptors) {
      allPromises.push(fetch(obj.get_url + queryStr, { method: 'POST' })
        .then(jsonOrThrow)
        .then((json) => {
          this.updatePendingTasks(json, obj);
        })
        .catch(errorMessage));
    }

    // Find all tasks scheduled in the future.
    queryParams = {
      include_future_runs: true,
    };
    queryStr = `?${fromObject(queryParams)}`;
    for (const obj of taskDescriptors) {
      allPromises.push(fetch(obj.get_url + queryStr, { method: 'POST' })
        .then(jsonOrThrow)
        .then((json) => {
          this.updatePendingTasks(json, obj);
        })
        .catch(errorMessage));
    }
    return Promise.all(allPromises);
  }

  // Add responses to pending tasks list.
  updatePendingTasks(json, taskDescriptor) {
    const tasks = json.data;
    for (let i = 0; i < tasks.length; i++) {
      const task = tasks[i];
      task.canDelete = json.permissions[i].DeleteAllowed;
      task.Id = json.ids[i];
      task.TaskType = taskDescriptor.type;
      task.GetURL = taskDescriptor.get_url;
      task.DeleteURL = taskDescriptor.delete_url;
      // Check if this is a completed task set to repeat.
      if (task.RepeatAfterDays !== 0 && task.TaskDone) {
        // Calculate the future date.
        const timestamp = getTimestamp(task.TsAdded);
        timestamp.setDate(timestamp.getDate() + task.RepeatAfterDays);
        task.FutureDate = true;
        task.TsAdded = getCtDbTimestamp(new Date(timestamp));
      }
    }
    this._pendingTasks = this._pendingTasks.concat(tasks);
    // Sort pending tasks according to TsAdded.
    this._pendingTasks.sort((a, b) => a.TsAdded - b.TsAdded);
  }


  confirmDeleteTask(index) {
    document.getElementById('confirm_dialog')
      .open('Proceed with deleting task?')
      .then(() => {
        this.deleteTask(index);
      })
      .catch(() => {});
  }

  deleteTask(index) {
    const pendingTask = this._pendingTasks[index];
    const params = {};
    params.id = pendingTask.Id;
    fetch(pendingTask.DeleteURL, { method: 'POST', body: JSON.stringify(params) })
      .then((res) => {
        if (res.ok) {
          this._pendingTasks.splice(index, 1);
          $$('#confirm_toast').innerText = `Deleted ${pendingTask.TaskType} task ${pendingTask.Id}`;
          $$('#confirm_toast').show();
          return;
        }
        // Non-OK status. Read the response and punt it to the catch.
        return res.text().then((text) => { throw `Failed to delete the task: ${text}`; });
      })
      .then(() => {
        this._render();
      })
      .catch(errorMessage);
  }
});
