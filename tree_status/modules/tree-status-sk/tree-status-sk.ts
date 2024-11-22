/**
 * @module tree-status-sk
 * @description <h2><code>tree-status-sk</code></h2>
 *
 * <p>
 *   Displays the enter-tree-status-sk and display-tree-status-sk elements.
 *   Handles calls to the backend from events originating from those elements.
 * </p>
 *
 */

import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';

import '../display-tree-status-sk';
import '../enter-tree-status-sk';

import { $$ } from '../../../infra-sk/modules/dom';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { AutorollerSnapshot, Status } from '../json';
import { EnterTreeStatus } from '../enter-tree-status-sk/enter-tree-status-sk';

export class TreeStatusSk extends ElementSk {
  private statuses: Status[] = [];

  private autorollers: AutorollerSnapshot[] = [];

  constructor() {
    super(TreeStatusSk.template);
  }

  private static template = (ele: TreeStatusSk) => html`
    <br />
    <enter-tree-status-sk
      .autorollers=${ele.autorollers}></enter-tree-status-sk>
    <br />
    <display-tree-status-sk .statuses=${ele.statuses}></display-tree-status-sk>
  `;

  connectedCallback(): void {
    super.connectedCallback();

    this.addEventListener('new-tree-status', (e) => this.saveStatus(e));
    this.addEventListener('set-tree-status', (e) => this.setTreeStatus(e));

    this.poll();
    this._render();
  }

  /** @prop repo {string} Reflects the repo attribute for ease of use. */
  get repo(): string {
    return this.getAttribute('repo') || '';
  }

  set repo(val: string) {
    this.setAttribute('repo', val);
  }

  private poll() {
    this.getStatuses();
    this.getAutorollers();
    window.setTimeout(() => this.poll(), 10000);
  }

  // Common work done for all fetch requests.
  private doImpl(url: string, detail: any, action: (json: any) => void) {
    fetch(url, {
      body: JSON.stringify(detail),
      headers: {
        'content-type': 'application/json',
      },
      credentials: 'include',
      method: 'POST',
    })
      .then(jsonOrThrow)
      .then((json) => {
        action(json);
        this._render();
      })
      .catch(errorMessage);
  }

  private saveStatus(e: Event) {
    this.doImpl(
      `/${this.repo}/_/add_tree_status`,
      (e as CustomEvent).detail,
      (json: Status[]) => {
        this.statuses = json;
      }
    );
  }

  private getStatuses() {
    this.doImpl(`/${this.repo}/_/recent_statuses`, {}, (json: Status[]) => {
      this.statuses = json;
    });
  }

  private getAutorollers() {
    this.doImpl('/_/get_autorollers', {}, (json: AutorollerSnapshot[]) => {
      this.autorollers = json;
    });
  }

  private setTreeStatus(e: Event) {
    ($$('enter-tree-status-sk') as EnterTreeStatus).status_value = (
      e as CustomEvent
    ).detail;
  }
}

define('tree-status-sk', TreeStatusSk);
