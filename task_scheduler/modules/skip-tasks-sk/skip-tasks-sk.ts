/**
 * @module modules/skip-tasks-sk
 * @description <h2><code>skip-tasks-sk</code></h2>
 *
 * Provides UI for manipulating rules to prevent triggering of matching tasks.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
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
import { $$ } from 'common-sk/modules/dom';
import 'elements-sk/icon/add-icon-sk';
import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/styles/buttons';
import 'elements-sk/styles/table';

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
    const name = $$<HTMLInputElement>('#input-name', this)!.value;
    const description = $$<HTMLTextAreaElement>('#input-description', this)!
      .value;
    const commitStart = $$<HTMLInputElement>('#input-range-start', this)!.value;
    const commits = [];
    if (commitStart) {
      commits.push(commitStart);
      if (this.isCommitRange) {
        const commitEnd = $$<HTMLInputElement>('#input-range-end', this)?.value;
        if (commitEnd) {
          commits.push(commitEnd);
        }
      }
    }
    const taskSpecs = $$<MultiInputSk>('#input-task-specs')!.values;
    this.rpc!.addSkipTaskRule({
      name: name,
      commits: commits,
      description: description,
      taskSpecPatterns: taskSpecs,
    }).then((resp: AddSkipTaskRuleResponse) => {
      this.rules = resp.rules!;
      this._render();
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
