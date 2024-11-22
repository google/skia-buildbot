/**
 * @module modules/skip-tasks-sk
 * @description <h2><code>skip-tasks-sk</code></h2>
 *
 * Provides UI for manipulating rules to prevent triggering of matching tasks.
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../../../infra-sk/modules/multi-input-sk';
import { MultiInputSk } from '../../../infra-sk/modules/multi-input-sk/multi-input-sk';
import {
  AddSkipTaskRuleResponse,
  TaskSchedulerService,
  SkipTaskRule,
  GetSkipTaskRulesResponse,
  DeleteSkipTaskRuleResponse,
} from '../rpc';
import { $$ } from '../../../infra-sk/modules/dom';
import '../../../elements-sk/modules/icons/add-icon-sk';
import '../../../elements-sk/modules/icons/delete-icon-sk';

export class SkipTasksSk extends ElementSk {
  private static template = (ele: SkipTasksSk) => html`
    ${
      ele.rules
        ? html`
            <table>
              <tr>
                <th><!-- delete button--></th>
                <th>Name</th>
                <th>Added by</th>
                <th>TaskSpec Patterns</th>
                <th>Commits</th>
                <th>Description</th>
              </tr>
              ${ele.rules.map(
                (rule) => html`
                  <tr>
                    <td>
                      <button @click="${() => ele.deleteRule(rule)}">
                        <delete-icon-sk></delete-icon-sk>
                      </button>
                    </td>
                    <td>${rule.name}</td>
                    <td>${rule.addedBy}</td>
                    <td>
                      ${rule.taskSpecPatterns?.map(
                        (pattern) => html`
                          <div class="task_spec_pattern">${pattern}</div>
                        `
                      )}
                    </td>
                    <td>
                      ${rule.commits?.map(
                        (commit) => html` <div class="commit">${commit}</div> `
                      )}
                    </td>
                    <td>${rule.description}</td>
                  </tr>
                `
              )}
            </table>
          `
        : html``
    }
    <button class="secondary-container-themes-sk" @click="${() => {
      $$<HTMLDialogElement>('dialog', ele)!.showModal();
    }}">
      <add-icon-sk></add-icon-sk>Add Rule
    </button>
    <dialog>
      <h2>New Rule</h2>
      <table class="new-rule">
        <tr>
          <td>
            <label for="input-name">Rule Name</label>
          </td>
          <td>
            <input id="input-name"></input>
          </td>
        </tr>
        <tr>
          <td>
            <label for="input-task-specs">Task Spec(s)</label>
          </td>
          <td>
            <multi-input-sk id="input-task-specs"></multi-input-sk>
          </td>
        </tr>
        <tr>
          <td>
            <label for="range-checkbox">Commit Range?</label>
          </td>
          <td>
            <input
              type="checkbox"
              id="range-checkbox"
              ?checked="${ele.isCommitRange}"
              @change="${(ev: Event) => {
                ele.isCommitRange = (<HTMLInputElement>ev.target).checked;
                ele._render();
              }}"
              >
            </input>
          </td>
        </tr>
        <tr>
          <td>
            <label for="input-range-start">
              ${
                ele.isCommitRange ? 'Range start (oldest; inclusive)' : 'Commit'
              }
            </label>
          </td>
          <td>
            <input id="input-range-start"></input>
          </td>
        </tr>
        ${
          ele.isCommitRange
            ? html`
        <tr>
          <td>
            <label for="input-range-end">Range end (newest; non-inclusive)</label>
          </td>
          <td>
            <input id="input-range-end"></input>
          </td>
        </tr>
        `
            : html``
        }
        <tr>
          <td>
            <label for="description">Rule Description</label>
          </td>
          <td>
            <textarea id="input-description" rows="5"></textarea>
          </td>
        </tr>
      </table>
      <button id="add-button" class="secondary-container-themes-sk" @click="${
        ele.addRule
      }">Add Rule</button>
      <button @click="${() => {
        $$<HTMLDialogElement>('dialog', ele)?.close();
      }}">
        Cancel
      </button>
    </dialog>
  `;

  private isCommitRange: boolean = false;

  private _rpc: TaskSchedulerService | null = null;

  private rules: SkipTaskRule[] = [];

  get rpc(): TaskSchedulerService | null {
    return this._rpc;
  }

  set rpc(rpc: TaskSchedulerService | null) {
    this._rpc = rpc;
    this.reload();
  }

  constructor() {
    super(SkipTasksSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
  }

  private addRule() {
    const inputName = $$<HTMLInputElement>('#input-name', this)!;
    const inputDescription = $$<HTMLTextAreaElement>(
      '#input-description',
      this
    )!;
    const inputRangeStart = $$<HTMLInputElement>('#input-range-start', this)!;
    const inputRangeEnd = $$<HTMLInputElement>('#input-range-end', this);
    const inputIsRange = $$<HTMLInputElement>('#range-checkbox', this)!;
    const inputTaskSpecs = $$<MultiInputSk>('#input-task-specs')!;
    const name = inputName.value;
    const description = inputDescription.value;
    const commitStart = inputRangeStart.value;
    const commits = [];
    if (commitStart) {
      commits.push(commitStart);
      if (this.isCommitRange) {
        const commitEnd = inputRangeEnd!.value;
        if (commitEnd) {
          commits.push(commitEnd);
        }
      }
    }
    const taskSpecs = inputTaskSpecs.values;
    this.rpc!.addSkipTaskRule({
      name: name,
      commits: commits,
      description: description,
      taskSpecPatterns: taskSpecs,
    }).then((resp: AddSkipTaskRuleResponse) => {
      this.rules = resp.rules!;
      this._render();
      inputName.value = '';
      inputDescription.value = '';
      inputRangeStart.value = '';
      if (inputRangeEnd) {
        inputRangeEnd.value = '';
      }
      inputIsRange.checked = false;
      inputTaskSpecs.values = [];
    });
    $$<HTMLDialogElement>('dialog', this)!.close();
  }

  private deleteRule(rule: SkipTaskRule) {
    this.rpc!.deleteSkipTaskRule({ id: rule.name }).then(
      (resp: DeleteSkipTaskRuleResponse) => {
        this.rules = resp.rules!;
        this._render();
      }
    );
  }

  private reload() {
    this.rpc!.getSkipTaskRules({}).then((resp: GetSkipTaskRulesResponse) => {
      this.rules = resp.rules!;
      this._render();
    });
  }
}

define('skip-tasks-sk', SkipTasksSk);
