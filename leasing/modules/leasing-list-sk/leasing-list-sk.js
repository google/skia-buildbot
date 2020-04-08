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
import { html } from 'lit-html';
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

function displayTasks(ele) {
  return ele._tasks.map((task) => html`
    <leasing-task-sk .task=${task}></leasing-task-sk>
  `);
}

const template = (ele) => html`${displayTasks(ele)}`;

define('leasing-list-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._tasks = [];

    this._fetchTasks();
  }

  _fetchTasks() {
    const url = '/_/get_leases';
    const details = {
      filter_by_user: this.filterByUser,
    };
    doImpl(url, details, (json) => {
      this._tasks = json;
      this._render();
    });
  }

  connectedCallback() {
    super.connectedCallback();
    upgradeProperty(this, 'filterByUser');
    this._render();
  }

  static get observedAttributes() {
    return ['filter_by_user'];
  }

  attributeChangedCallback(name, oldValue, newValue) {
    switch (name) {
      case 'filter_by_user':
        if (newValue !== '') {
          this._fetchTasks();
        }
        break;
      default:
    }
  }

  /** @prop filter_by_user {String} User tasks should be filtered by. */
  get filterByUser() {
    return this.getAttribute('filter_by_user');
  }

  set filterByUser(val) {
    this.setAttribute('filter_by_user', val);
  }

  disconnectedCallback() {
    super.disconnectedCallback();
  }
});
