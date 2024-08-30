/**
 * @module autoroll/modules/arb-roll-history-sk
 * @description <h2><code>arb-roll-history-sk</code></h2>
 *
 * <p>
 * This element displays the roll history for a roller.
 * </p>
 */

import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../../../infra-sk/modules/human-date-sk';

import {
  AutoRollService,
  GetAutoRollService,
  GetRollsResponse,
  AutoRollCL,
  AutoRollCL_Result,
  GetStatusResponse,
} from '../rpc';

export class ARBRollHistorySk extends ElementSk {
  private static template = (ele: ARBRollHistorySk) => html`
    <a href="/r/${ele.roller}" class="small">back to roller status</a>
    <br />
    <table>
      <tr>
        <th>Roll</th>
        <th>Creation Time</th>
        <th>Result</th>
      </tr>
      ${ele.history.map(
        (roll: AutoRollCL) => html`
          <tr>
            <td>
              ${ele.issueURLBase !== ''
                ? html`
                    <a href="${ele.issueURL(roll)}" target="_blank"
                      >${roll.subject}</a
                    >
                  `
                : html` ${roll.subject} `}
            </td>
            <td>
              <human-date-sk
                .date="${roll.created}"
                .diff="${true}"></human-date-sk>
            </td>
            <td>
              <span class="${ele.rollClass(roll)}">${roll.result}</span>
            </td>
          </tr>
        `
      )}
    </table>
    <br />
    <button
      @click="${() => {
        ele.loadPrevious();
      }}"
      ?disabled="${!ele.canLoadPrevious}">
      Previous
    </button>
    <button
      @click="${() => {
        ele.loadNext();
      }}"
      ?disabled="${!ele.canLoadNext}">
      Next
    </button>
  `;

  private history: AutoRollCL[] = [];

  private cursorHistory: string[] = ['']; // [..., prevCursor, currentCursor, nextCursor]

  private canLoadPrevious: boolean = false;

  private canLoadNext: boolean = false;

  private issueURLBase: string = '';

  private rpc: AutoRollService;

  constructor() {
    super(ARBRollHistorySk.template);
    this.rpc = GetAutoRollService(this);
  }

  connectedCallback() {
    super.connectedCallback();
    this.loadStatus();
    this.load('');
  }

  get roller() {
    return this.getAttribute('roller') || '';
  }

  set roller(v: string) {
    this.setAttribute('roller', v);
    this.cursorHistory = [''];
    this.load('');
  }

  // TODO(borenet): Share this code with arb-status-sk.
  private rollClass(roll: AutoRollCL) {
    if (!roll) {
      return 'unknown';
    }
    switch (roll.result) {
      case AutoRollCL_Result.SUCCESS:
        return 'fg-success';
      case AutoRollCL_Result.FAILURE:
        return 'fg-failure';
      case AutoRollCL_Result.IN_PROGRESS:
        return 'fg-unknown';
      case AutoRollCL_Result.DRY_RUN_SUCCESS:
        return 'fg-success';
      case AutoRollCL_Result.DRY_RUN_FAILURE:
        return 'fg-failure';
      case AutoRollCL_Result.DRY_RUN_IN_PROGRESS:
        return 'fg-unknown';
      default:
        return 'fg-unknown';
    }
  }

  private issueURL(roll: AutoRollCL) {
    return this.issueURLBase + roll.id;
  }

  private load(cursor: string) {
    const req = {
      rollerId: this.roller,
      cursor: cursor,
    };
    this.rpc.getRolls(req).then((resp: GetRollsResponse) => {
      this.history = resp.rolls!;
      this.cursorHistory.push(resp.cursor);
      this.canLoadNext =
        this.cursorHistory[this.cursorHistory.length - 1] !== '';
      this.canLoadPrevious = this.cursorHistory.length > 2;
      this._render();
    });
  }

  private loadNext() {
    if (!this.canLoadNext) {
      return;
    }
    this.load(this.cursorHistory[this.cursorHistory.length - 1]);
  }

  private loadPrevious() {
    if (!this.canLoadPrevious) {
      return;
    }
    // [..., previous, current, next] => [..., previous]
    this.cursorHistory = this.cursorHistory.slice(
      0,
      this.cursorHistory.length - 2
    );
    this.load(this.cursorHistory[this.cursorHistory.length - 1]);
  }

  private loadStatus() {
    this.rpc
      .getStatus({ rollerId: this.roller })
      .then((resp: GetStatusResponse) => {
        this.issueURLBase = resp.status!.issueUrlBase;
        this._render();
      });
  }
}

define('arb-roll-history-sk', ARBRollHistorySk);
