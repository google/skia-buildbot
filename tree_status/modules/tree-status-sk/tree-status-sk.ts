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

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { errorMessage } from 'elements-sk/errorMessage';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';

import '../display-tree-status-sk';
import '../enter-tree-status-sk';

import { $$ } from 'common-sk/modules/dom';
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
  <enter-tree-status-sk .autorollers=${ele.autorollers}></enter-tree-status-sk>
  <display-tree-status-sk .statuses=${ele.statuses}></display-tree-status-sk>
`;

  connectedCallback(): void {
    super.connectedCallback();

    this.addEventListener('new-tree-status', (e) => this.saveStatus(e));
    this.addEventListener('set-tree-status', (e) => this.setTreeStatus(e));

    this.poll();
    this._render();
  }

  private poll() {
    this.getStatuses();
    this.getAutorollers();
    window.setTimeout(() => this.poll(), 10000);
  }

  // Common work done for all fetch requests.
  private doImpl(url: string, detail: any, action: (json: any)=> void) {
    fetch(url, {
      body: JSON.stringify(detail),
      headers: {
        'content-type': 'application/json',
      },
      credentials: 'include',
      method: 'POST',
    }).then(jsonOrThrow).then((json) => {
      action(json);
      this._render();
    }).catch(errorMessage);
  }

  private saveStatus(e: Event) {
    this.doImpl('/_/add_tree_status', (e as CustomEvent).detail, (json: Status[]) => { this.statuses = json; });
  }

  private getStatuses() {
    this.doImpl('/_/recent_statuses', {}, (json: Status[]) => { this.statuses = json; });
  }

  private getAutorollers() {
    this.doImpl('/_/get_autorollers', {}, (json: AutorollerSnapshot[]) => { this.autorollers = json; });
  }

  private setTreeStatus(e: Event) {
    ($$('enter-tree-status-sk') as EnterTreeStatus).status_value = (e as CustomEvent).detail;
  }
}

define('tree-status-sk', TreeStatusSk);
