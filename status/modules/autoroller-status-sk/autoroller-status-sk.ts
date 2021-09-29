/**
 * @module modules/autoroller-status-sk
 * @description <h2><code>autoroller-status-sk</code></h2>
 *
 * Custom element for displaying status of Skia autorollers.
 *
 * @event rollers-update: Periodic event for updated roller status. Detail
 * is of type Array<AutorollerStatus>
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { StatusService, GetStatusService, AutorollerStatus } from '../rpc';

// Type of rollers-update event.detail.
declare global {
  interface DocumentEventMap {
    'rollers-update': CustomEvent<Array<AutorollerStatus>>;
  }
}

function colorClass(status: AutorollerStatus) {
  // Find a color class for the roller.
  // TODO(borenet): These numbers (especially number of commits behind)
  // are probably going to differ from roller to roller. How can we give
  // each roller its own definition of "bad"?
  let badness = status.numFailed / 2.0;
  const badnessBehind = status.numBehind / 20.0;
  if (status.mode !== 'dry run' && badnessBehind > badness) {
    badness = badnessBehind;
  }
  if (status.mode === 'stopped') {
    return 'bg-unexpected';
  } if (badness < 0.5) {
    return 'bg-success';
  } if (badness < 1.0) {
    return 'bg-warning';
  }
  return 'bg-failure';
}

export class AutorollerStatusSk extends ElementSk {
  private client: StatusService = GetStatusService();

  private rollers: Array<AutorollerStatus> = [];

  private static template = (el: AutorollerStatusSk) => html`
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
              class="tr roller ${colorClass(roller)}"
              href=${roller.url}
              target="_blank"
              rel="noopener noreferrer"
            >
              <div class="td">${roller.name}</div>
              <div class="td">${roller.mode}</div>
              <div class="td number">${roller.numFailed}</div>
              <div class="td number">${roller.numBehind}</div>
            </a>
          `,
  )}
      </div>
    `;

  constructor() {
    super(AutorollerStatusSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this.refresh();
  }

  private refresh() {
    this.client
      .getAutorollerStatuses({})
      .then((resp) => {
        this.rollers = resp.rollers || [];
        this._render();
        this.dispatchEvent(
          new CustomEvent<Array<AutorollerStatus>>('rollers-update', {
            bubbles: true,
            detail: this.rollers.slice(),
          }),
        );
      })
      .finally(() => {
        // Updates happen periodically on the backend, this being configurable provides no benefit.
        window.setTimeout(() => this.refresh(), 60 * 1000);
      });
  }
}

define('autoroller-status-sk', AutorollerStatusSk);
