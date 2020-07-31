/**
 * @module modules/commits-table-sk
 * @description A custom element for allowing the user to choose whether
 * their task should be repeated either never, daily, every other day,
 * or weekly.
 */

import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/select-sk';

import { TaskDetails } from '../types';

const frequencies = [
  { num: '0', desc: 'Never repeat' },
  { num: '1', desc: 'Repeat daily' },
  { num: '2', desc: 'Repeat every other day' },
  { num: '7', desc: 'Repeat weekly' }];

const template = () => html`
<div class=tr-container>
  <select-sk>
    ${frequencies.map((f) => html`
    <div>
      <span class=num>${f.num}</span><span>${f.desc}</span>
    </div>`)}
  </select-sk>
</div>
`;

export class CommitsTableSk extends ElementSk {
  task_details: TaskDetails | null = null;
  task_specs: object | null = null;
  tasks: object | null = null;
  categories: object | null = null;
  category_list: Array<string> | null = null;
  commits: Array<object> | null = null;
  commits_map: object | null = null;
  logged_in: boolean | null = null;
  relanded_map: object | null = null;
  repo: string | null = null;
  repo_base: string | null = null;
  reverted_map: object | null = null;
  swarming_url: string | null = null;
  task_scheduler_url: string | null = null;

  private static template = (ele: CommitsTableSk) => html`<div>Hello World!</div>`;
  constructor() {
    super(CommitsTableSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    // this._selector = $$('select-sk', this);
    //:this._selector.selection = 0;
  }
};

define('commits-table-sk', CommitsTableSk);