/**
 * @module modules/task-priority-sk
 * @description A custom element for allowing the user to select their
 * task's priority.
 */

import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { errorMessage } from 'elements-sk/errorMessage';

import { SelectSk } from 'elements-sk/select-sk/select-sk';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/select-sk';

import {
  TaskPrioritiesResponse,
} from '../json';

export class TaskPrioritySk extends ElementSk {
private _priorities: string[][] = [];

private _selector: SelectSk | null = null;

constructor() {
  super(TaskPrioritySk.template);
}

  private static template = (el: TaskPrioritySk) => html`
<div class=tr-container>
  <select-sk>
    ${el._priorities.map((p) => html`
    <div>${p[1]}</div>`)}
  </select-sk>
</div>
`;

  connectedCallback(): void {
    super.connectedCallback();
    fetch('/_/task_priorities/', { method: 'GET' })
      .then(jsonOrThrow)
      .then((json: TaskPrioritiesResponse) => {
        // { 'p1' : 'Desc1', ...}  -> [['p1', 'Desc1'], ...]
        this._priorities = Object.entries<string>(json.task_priorities);
        this._render();
        this._selector!.selection = 1;
      })
      .catch(errorMessage);
    this._render();
    this._selector = $$('select-sk', this);
  }

  /**
   * @prop {string} priority - Priority string representing user selected
   * priority of their task.
   */
  get priority(): string {
    return this._priorities[this._selector!.selection as number][0];
  }

  set priority(val: string) {
    this._selector!.selection = this._priorities.findIndex((p) => p[0] === val);
  }
}

define('task-priority-sk', TaskPrioritySk);
