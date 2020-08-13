/**
 * @module flaky-tasks-sk
 * @description <h2><code>flaky-tasks-sk</code></h2>
 *
 * <p>
 * This element displays information about flaky tasks.
 * </p>
 *
 */
import { html, render } from 'lit-html'
import { $$ } from 'common-sk/modules/dom'
import { localeTime, strDuration } from 'common-sk/modules/human'
import { upgradeProperty } from 'elements-sk/upgradeProperty'


const byTaskName = (ele) => html`
  <div>
    <h2>By Task Name</h2>
    <table>
      ${ele._byTaskName.map((kv) => html`
        <tr>
          <td>${kv[0]}</td>
          <td class="number">${kv[1]}</td>
        </tr>
      `)}
    </table>
  </div>
`;

const dimTable = (ele, dimList) => html`
    <tr>
      <td>${!dimList[0] ? console.log(dimList) || "wtf" : dimList[0]}:</td>
      <td>
        ${dimList[1].map((e) => html`
            <tr>
              <td>${e[0]}</td>
              ${typeof e[1] === "number"
                ? html`<td class="number">${e[1]}</td>`
                : html`<td>${dimTable(ele, e[1])}</td>`
              }
            </tr>
        `)}
      </td>
    </tr>
`;

const byDimensions = (ele) => html`
  <div>
    <h2>By Dimension Set</h2>
    <table>
    ${dimTable(ele, ele._byDimensions)}
    </table>
  </div>
`;

const template = (ele) => html`
  <div>
    ${ele._numFlakyTasks} flaky tasks out of ${ele._numTotalTasks} (${ele._percentFlaky}%)<br/>
    in ${localeTime(ele._timeStart)} - ${localeTime(ele._timeEnd)} (${strDuration((ele._timeEnd.getTime()-ele._timeStart.getTime())/1000)})<br/>
  </div>
  ${byTaskName(ele)}
  ${byDimensions(ele)}
`;


window.customElements.define('flaky-tasks-sk', class extends HTMLElement {
  constructor() {
    super();
    this._data = [];
  }

  connectedCallback() {
    upgradeProperty(this, 'data');
    this._render();
  }

  _mergeDims(dims) {
    dims.sort();
    return dims.join(" ");
  }

  _mapDims(dims) {
    let dimsMap = {};
    for (var i = 0; i < dims.length; i++) {
      let split = dims[i].split(":");
      if (split.length < 2) {
        console.log("Warning: invalid dimension: " + dims[i]);
      } else {
        dimsMap[split[0]] = split.slice(1).join(":");
      }
    }
    return dimsMap;
  }

  _divideDims(tasks, dims) {
    // Return a map which divides tasks by dimensions.

    // First, find the set of all dimensions.
    let allDims = {};
    for (var i = 0; i < tasks.length; i++) {
      let task = tasks[i]
      let dimsForTask = dims[task.name];
      if (!dimsForTask) {
        console.log("Warning: no dimensions for " + task.name);
      } else {
        let dimsMap = this._mapDims(dimsForTask);
        task.dimensions = dimsMap; // Store the dimensions for easy access later.
        for (var key in dimsMap) {
          allDims[key] = true;
        }
      }
    }

    // Next, ensure that each dimension key also includes tasks which don't have
    // that dimension (eg. key = "device_type", value = "").
    let missing = "(missing)"
    for (var key in allDims) {
      for (var i = 0; i < tasks.length; i++) {
        let task = tasks[i];
        // If the task doesn't have this dimension, add it to the empty list.
        if (task.dimensions && !task.dimensions[key]) {
          task.dimensions[key] = missing;
        }
      }
    }

    var groupTasksByDimensions = function(tasks) {
      let tasksByDims = {};
      for (var i = 0; i < tasks.length; i++) {
        let task = tasks[i];
        if (task.dimensions) {
          for (var key in task.dimensions) {
            if (!tasksByDims[key]) {
              tasksByDims[key] = {};
            }
            let value = task.dimensions[key];
            if (!tasksByDims[key][value]) {
              tasksByDims[key][value] = [];
            }
            tasksByDims[key][value].push(task);
          }
        }
      }
      return tasksByDims;
    };

    let tasksByDims = groupTasksByDimensions(tasks);
    var remainingDimKeys = [];
    for (var key in tasksByDims) {
      remainingDimKeys.push(key);
    }
    remainingDimKeys.sort();

    var mapDims = function(tasksByDims, remainingDimKeys) {
      // Select the remaining dimension key which the most tasks have defined.
      let winnerIdx = -1;
      let winnerCount = 0;
      for (var i = 0; i < remainingDimKeys.length; i++) {
        let key = remainingDimKeys[i]
        let vals = tasksByDims[key];
        let count = 0;
        for (var val in tasksByDims[key]) {
          if (val != missing) {
            count += tasksByDims[key][val].length;
          }
        }
        if (count > winnerCount) {
          winnerIdx = i;
          winnerCount = count;
        }
      }

      // If there's no winner, then all of the remaining dimension keys have no
      // associated tasks in tasksByDims. Just return the total remaining tasks.
      let winner = remainingDimKeys[winnerIdx];
      if (!winner) {
        let tasks = {}
        for (key in tasksByDims) {
          for (val in tasksByDims[key]) {
             for (var i = 0; i < tasksByDims[key][val].length; i++) {
               let task = tasksByDims[key][val][i];
               tasks[task.id] = task;
             }
          }
        }
        return Object.keys(tasks).length;
      }

      // Trim the list of remaining dimension keys, including those which have
      // no non-empty values.
      let newRemainingDimKeys = [];
      for (var i = 0; i < remainingDimKeys.length; i++) {
        if (i == winnerIdx) {
          continue;
        }
        var key = remainingDimKeys[i];
        if (Object.keys(tasksByDims[key]).length == 1 && tasksByDims[key][missing]) {
          continue;
        }
        newRemainingDimKeys.push(key);
      }
      newRemainingDimKeys.sort();

      // Divide the tasks by the chosen dimension.
      let entries = [];
      for (var val in tasksByDims[winner]) {
        let nextTasks = groupTasksByDimensions(tasksByDims[winner][val]);
        let results = mapDims(nextTasks, newRemainingDimKeys);
        entries.push([val, results]);
      }
      entries.sort((a, b) => tasksByDims[winner][b[0]].length - tasksByDims[winner][a[0]].length);
      return [winner, entries];
    }
    return mapDims(tasksByDims, remainingDimKeys);
  }

  get data() { return this._data; }
  set data(data) {
    // High-level stats.
    this._numTotalTasks = data.numTotalTasks;
    this._numFlakyTasks = data.tasks.length;
    this._percentFlaky = this._numFlakyTasks / this._numTotalTasks * 100;
    this._timeStart = new Date(data.timeStart);
    this._timeEnd = new Date(data.timeEnd);

    // Detailed breakdowns.

    // By task name.
    this._tasks = data.tasks;
    let countsByTaskName = {};
    for (var i = 0; i < this._tasks.length; i++) {
      let task = this._tasks[i];
      if (!countsByTaskName[task.name]) {
        countsByTaskName[task.name] = 0;
      }
      countsByTaskName[task.name]++;
    }
    this._byTaskName = [];
    for (var name in countsByTaskName) {
      this._byTaskName.push([name, countsByTaskName[name]]);
    }
    this._byTaskName.sort((a, b) => b[1] - a[1]);

    // By dimension set.
    this._byDimensions = this._divideDims(this._tasks, data.dimensions);
    console.log(this._byDimensions);

    this._render();
  }

  _render() {
    console.log("render");
    render(template(this), this, {eventContext: this});
  }
});
