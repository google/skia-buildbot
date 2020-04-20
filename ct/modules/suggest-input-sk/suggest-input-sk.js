/**
 * @fileoverview A custom element that loads the CT pending tasks queue and
 * displays it as a table.
 */

import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/cancel-icon-sk';
import 'elements-sk/icon/check-circle-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/toast-sk';
import '../../../infra-sk/modules/confirm-dialog-sk';

import { $$, DomReady } from 'common-sk/modules/dom';
import { fromObject } from 'common-sk/modules/query';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { html } from 'lit-html';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
  getFormattedTimestamp, taskDescriptors, getTimestamp, getCtDbTimestamp,
} from '../ctfe_utils';

const template = (el) => html`
<style>
    #suggestions_div {
      border: 1px solid black;
      position: absolute;
      z-index: 1;
    }
</style>
    <!--
    This element just wraps a paper-input. We trap the on-change events because
    we only want to fire that event when the user has typed or accepted one of
    the autocomplete suggestions. Every key press occurring when the paper-input
    has focus results in a call to _keyup. We treat some key presses specially;
    up and down arrow cause different autocomplete suggestions to be
    highlighted, and the enter key accepts a suggestion when one is highlighted.
    Otherwise, we recompute the suggestions and display them. When the user
    accepts a suggestion by clicking it, hitting enter when it is highlighted,
    or simply by typing it out and unfocusing the paper-input, we call _commit,
    which fires the on-change event.
    -->
    <paper-input id="input" label="[[label]]"
      on-change="_change"
      on-keyup="_keyup"
      on-focus="_onFocus"
      on-blur="_keyup"
    ></paper-input>
    <div id="suggestions_div" hidden$="{{_show_suggestions(_suggestions.*)}}">
      <paper-listbox id="suggestions_box" on-iron-select="_suggestion_changed">
        <template is="dom-repeat" items="[[_suggestions]]">
          <paper-item class="suggestion" value="{{item}}">{{item}}</paper-item>
        </template>
      </paper-listbox>
    </div>
  </template>
`;

const taskRowTemplate = (el, task, index) => html`
<tr>
  <td class=nowrap>
    ${index + 1}
    <delete-icon-sk title="Delete this task" alt=Delete ?hidden=${!task.canDelete}
      @click=${() => el.confirmDeleteTask(index)}></delete-icon-sk>
  </td>
  <td>
    ${getFormattedTimestamp(task.TsAdded)}
    ${task.FutureDate
    ? html`</br><span class=error-themes-sk>(scheduled in the future)</span>`
    : ''}
  </td>
  <td>${task.TaskType}</td>
  <td>${task.Username}</td>
  <td class=nowrap>${
  task.FutureDate
    ? html`N/A`
    : task.SwarmingLogs
      ? html`<a href="${task.SwarmingLogs}" rel=noopener target=_blank>Swarming Logs</a>`
      : html`No Swarming Logs`}</td>
  <td class=nowrap>
    <a href=# class=details
      @click=${() => el.showDetailsDialog(index)}>Task Details</a>
  </td>
</tr>`;

const taskDetailDialogTemplate = (task, index) => html`
<div id=${`detailsDialog${index}`} class="dialog-background hidden overlay-themes-sk"
  @click=${hideDialog}>
  <div class="dialog-content surface-themes-sk">
    <pre>${formatTask(task)}</pre>
  </div>
</div>
`;

function hideDialog(e) {
  if (e.target.classList.contains('dialog-background')) {
    e.target.classList.add('hidden');
  }
}

function formatTask(task) {
  return JSON.stringify(task, null, 4);
}

define('chromium-perf-sk', class extends ElementSk {
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



  <script>
  (function(){
    var DOWN_ARROW = 40;
    var UP_ARROW = 38;
    var ENTER = 13;

    Polymer({
      is:"autocomplete-input-sk",

      properties: {
        autocomplete: {
          type: Array,
        },

        label: {
          type: String,
        },

        value: {
          type: String,
          value: "",
          notify: true,
          observer: "_value_changed",
        },

        displayOptionsOnFocus: {
          type: Boolean,
          value: false,
        },

        acceptCustomValue: {
          type: Boolean,
          value: false,
        },

        _arrow_key_pressed: {
          type: Boolean,
          value: false,
        },

        _committing: {
          type: Boolean,
          value: false,
        },

        _suggestions: {
          type: Array,
          value: function() {
            return [];
          },
        },
      },

      _onFocus: function(e) {
        if (this.displayOptionsOnFocus && (
                this.value === '' || this.value === undefined)) {
          this.value = '';
          this._keyup(e);
        }
      },

      _value_changed: function() {
        // This event fires when we set this.value or when our parent sets it.
        // In the latter case, we need to propagate the change down to our
        // child paper-input.
        if (this._committing) {
          // The event fired because _commit() was called. Don't propagate the
          // change or we'll get an infinite loop.
          this._committing = false;
        } else {
          // Our parent set this.value. Propagate the change.
          this.$.input.value = this.value;
        }
      },

      _change: function(e) {
        // This event fires when the user "finishes" typing in the text box.
        // This occurs when the enter key is pressed, or when the text box
        // loses focus. We only want our parent to receive a change event when
        // the user has finished typing, so we need to handle the case in which
        // this event fires because the user clicked one of the suggestions.
        e.preventDefault();
        e.stopPropagation();
        var v = this.$.input.value;
        if (this.autocomplete && this.autocomplete.indexOf(v) === -1) {
          // If we're autocompleting, don't fire the change event unless we
          // chose a suggested value.
        } else {
          this._commit()
        }
      },

      _commit: function() {
        // The user has "finished" typing. This occurs when the user clicks
        // a suggestion, or, once the user has typed out an exact match to one
        // of the suggestions, they hit the enter key or click out of the text
        // box. Set this.value and trigger a "change" event.
        this._committing = true;
        this._suggestions = [];
        this.$.suggestions_box.selected = -1;
        this.value = this.$.input.value;
        this.fire("change", {"value": this.value});
      },

      _keyup: function(e) {
        if (e.type === "blur") {
          // Ignore the blur event if it was caused by clicking a suggestion.
          var blurredElem = e.relatedTarget;
          if (!blurredElem) {
            blurredElem = e.detail.sourceEvent.relatedTarget;
          }
          if (blurredElem && blurredElem.classList.contains("suggestion")) {
            return;
          }
        }
        if (this.autocomplete) {
          // Allow the user to scroll through suggestions using arrow keys.
          if (e.keyCode === DOWN_ARROW && this._suggestions.length > 0) {
            this._arrow_key_pressed = true;
            this.$.suggestions_box.selected = this.$.suggestions_box.selected + 1;
          } else if (e.keyCode === UP_ARROW && this._suggestions.length > 0) {
            this._arrow_key_pressed = true;
            this.$.suggestions_box.selected = this.$.suggestions_box.selected - 1;
          } else if (e.type === "blur" || e.keyCode === ENTER) {
            if (this.$.suggestions_box.selected > -1) {
              // If the enter key is pressed while one of the suggestions is
              // highlighted, we accept that selection.
              this.$.input.value = this.$.suggestions_box.selectedItem.value;
              this._commit();
            } else if (this.autocomplete.includes(this.$.input.value)) {
              // The user manually typed one of the acceptable values.
              this._commit();
            } else if (this.acceptCustomValue) {
              // If the enter key is pressed without highlighting a suggestion
              // then accept the current input value.
              this._commit();
            } else {
              // User entered an invalid selection, and custom values are not
              // allowed.
              this.$.input.value = "";
              this._commit();
            }
          } else {
            // The user has entered text. Recompute autocomplete suggestions.
            var v = this.$.input.value;
            this._suggestions = [];
            // Give suggestions on empty input only if displayOptionsOnFocus
            // is true.
            if (v != "" || this.displayOptionsOnFocus) {
              // If possible, treat partially-entered input as a regular
              // expression.
              var re;
              try {
                re = new RegExp(v, "i");  // case-insensitive.
              } catch (e) {
                // If the user enters an invalid expression, just use substring
                // match.
                re = new Object();
                re.test = function(str) {
                  return str.indexOf(v) != -1;
                };
              }
              // Include anything in the autocomplete list which matches the
              // text in the box.
              for (var i = 0; i < this.autocomplete.length; ++i) {
                if (re.test(this.autocomplete[i])) {
                  this.push("_suggestions", this.autocomplete[i]);
                }
              }
            }
            // If the user types an exact match to one of the suggestions,
            // remove the suggestions box.
            if (this._suggestions.length === 1 && this._suggestions[0] === v) {
              this._suggestions = [];
            }
            // Ensure that no suggestions are selected after recomputing.
            this.$.suggestions_box.selected = -1;
          }
        }
      },

      _suggestion_changed: function() {
        if (this._arrow_key_pressed) {
          // This event fires while we're arrowing through the options. We don't
          // want to accept a suggestion while this is happening, so ignore the
          // event in that case.
          this._arrow_key_pressed = false;
        } else {
          // The user has clicked on a suggestion. Accept it.
          var selected = this.$.suggestions_box.selectedItem.value;
          this.$.input.value = selected;
          this._commit();
        }
      },

      _show_suggestions: function() {
        return !(this._suggestions && this._suggestions.length > 0);
      },
    });
  })()
  </script>
