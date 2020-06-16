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

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/select-sk';

const template = (el) => html`
<div class=tr-container>
  <select-sk>
    ${el._priorities.map((p) => html`
    <div>${p[1]}</div>`)}
  </select-sk>
</div>
`;

define('task-priority-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._priorities = [];
  }

  connectedCallback() {
    super.connectedCallback();
    fetch('/_/task_priorities/', { method: 'GET' })
      .then(jsonOrThrow)
      .then((json) => {
        // { 'p1' : 'Desc1', ...}  -> [['p1', 'Desc1'], ...]
        this._priorities = Object.entries(json.task_priorities);
        this._render();
        this._selector.selection = 1;
      })
      .catch(errorMessage);
    this._render();
    this._selector = $$('select-sk', this);
  }

  /**
   * @prop {string} priority - Priority string representing user selected
   * priority of their task.
   */
  get priority() {
    return this._priorities[this._selector.selection][0];
  }

  set priority(val) {
    this._selector.selection = this._priorities.findIndex((p) => p[0] === val);
  }
});
