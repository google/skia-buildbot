/**
 * @module modules/autoroller-status-sk
 * @description <h2><code>autoroller-status-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { StatusService, GetStatusService, AutorollerStatus } from '../rpc';

interface Roller {
  name: string;
  mode: string;
  class: string;
  url: string;
  numFailed: number;
  numBehind: number;
}
function modeToClass(mode: string) {
  return '';
}

export class AutorollerStatusSk extends ElementSk {
  private client: StatusService = GetStatusService();
  private rollers: Array<AutorollerStatus> = [];

  private static template = (el: AutorollerStatusSk) =>
    html`
      <div class="table">
        <div class="tr">
          <div class="th">Roller</div>
          <div class="th">Mode</div>
          <div class="th">Failed</div>
          <div class="th">Behind</div>
        </div>
        ${el.rollers.map(
          (roller) => html`
            <a
              class$="tr ${modeToClass(roller.mode)}"
              href$=${roller.url}
              target="_blank"
              rel="noopener noreferrer"
            >
              <div class="td">${roller.name}</div>
              <div class="td">${roller.mode}</div>
              <div class="td number">${roller.numFailed}</div>
              <div class="td number">${roller.numBehind}</div>
            </a>
          `
        )}
      </div>
    `;

  constructor() {
    super(AutorollerStatusSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this.client.getAutorollerStatuses({}).then((resp) => {
      this.rollers = resp.rollers || [];
      this._render();
    });
  }
}

define('autoroller-status-sk', AutorollerStatusSk);
