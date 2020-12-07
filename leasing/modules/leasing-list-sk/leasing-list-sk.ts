/**
 * @module module/leasing-list-sk
 * @description <h2><code>leasing-list-sk</code></h2>
 *
 * <p>
 *   Displays leasing tasks.
 * </p>
 *
 */

import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';
import { upgradeProperty } from 'elements-sk/upgradeProperty';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import '../leasing-task-sk';

import 'elements-sk/error-toast-sk';
import 'elements-sk/icon/folder-icon-sk';
import 'elements-sk/icon/gesture-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/icon/home-icon-sk';
import 'elements-sk/icon/star-icon-sk';
import 'elements-sk/nav-button-sk';
import 'elements-sk/nav-links-sk';
import { doImpl } from '../leasing';

import '../../../infra-sk/modules/login-sk';

import { Task } from '../json';

export class LeasingListSk extends ElementSk {
  private tasks: Task[] = [];

  constructor() {
    super(LeasingListSk.template);

    this.fetchTasks();
  }

  private static template = (ele: LeasingListSk) => html`${ele.displayTasks()}`;

  connectedCallback(): void {
    super.connectedCallback();
    upgradeProperty(this, 'filterByUser');
    this._render();
  }

  attributeChangedCallback(name: string, oldValue: string, newValue: string): void {
    switch (name) {
      case 'filter_by_user':
        if (newValue !== '') {
          this.fetchTasks();
        }
        break;
      default:
    }
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
  }

  private displayTasks(): TemplateResult[] {
    return this.tasks.map((task) => html`
      <leasing-task-sk .task=${task}></leasing-task-sk>
    `);
  }

  private fetchTasks(): void {
    const url = '/_/get_leases';
    const details = {
      filter_by_user: this.filterByUser,
    };
    doImpl(url, details, (json) => {
      this.tasks = json;
      this._render();
    });
  }

  static get observedAttributes(): string[] {
    return ['filter_by_user'];
  }

  /** @prop filter_by_user {String} User tasks should be filtered by. */
  get filterByUser(): string {
    return this.getAttribute('filter_by_user')!;
  }

  set filterByUser(val: string) {
    this.setAttribute('filter_by_user', val);
  }
}

define('leasing-list-sk', LeasingListSk);
