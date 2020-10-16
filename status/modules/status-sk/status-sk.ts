/**
 * @module modules/status-sk
 * @description <h2><code>status-sk</code></h2>
 *
 * The majority of the Status page.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import '../../../infra-sk/modules/theme-chooser-sk';
import '../commits-table-sk';
import '../autoroller-status-sk';
import 'elements-sk/error-toast-sk';

export class StatusSk extends ElementSk {
  private static template = (ele: StatusSk) =>
    html`
      <h2>lit-html/TS/Twirp Status Page Under Development</h2>
      <theme-chooser-sk></theme-chooser-sk>
      <autoroller-status-sk></autoroller-status-sk>
      <commits-table-sk></commits-table-sk>
      <error-toast-sk></error-toast-sk>
    `;

  constructor() {
    super(StatusSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }
}

define('status-sk', StatusSk);
