/**
 * @module autoroll/modules/arb-mode-history-sk
 * @description <h2><code>arb-mode-history-sk</code></h2>
 *
 * <p>
 * This element displays the mode change history for a roller.
 * </p>
 */

import { html } from 'lit-html';
import { define } from 'elements-sk/define';
import 'elements-sk/styles/table';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../../../infra-sk/modules/human-date-sk';

import {
  AutoRollService,
  GetAutoRollService,
  GetModeHistoryResponse,
  ModeChange,
} from '../rpc';

export class ARBModeHistorySk extends ElementSk {
  private static template = (ele: ARBModeHistorySk) => html`
  <a href="/r/${ele.roller}" class="small">back to roller status</a>
  <br/>
  <table>
    <tr>
      <th>Time</th>
      <th>Mode</th>
      <th>User</th>
      <th>Message</th>
    </tr>
    ${ele.history.map((entry: ModeChange) => html`
        <tr>
          <td><human-date-sk .date="${entry.time!}" .diff="${true}"></human-date-sk></td>
          <td>${entry.mode?.toString()}</td>
          <td>${entry.user?.toString()}</td>
          <td>${entry.message}</td>
        </tr>
      `,
  )}
  </table>
  <br/>
  <button
    @click="${() => { ele.load(ele.prevOffset); }}"
    ?disabled="${ele.currentOffset == 0}"
    >Previous</button>
  <button
    @click="${() => { ele.load(ele.nextOffset); }}"
    ?disabled="${ele.nextOffset == 0}"
    >Next</button>
`;

  private history: ModeChange[] = [];
  private prevOffset: number = -1;
  private currentOffset: number = 0;
  private nextOffset: number = 0;
  private rpc: AutoRollService;

  constructor() {
    super(ARBModeHistorySk.template);
    this.rpc = GetAutoRollService(this);
  }

  connectedCallback() {
    super.connectedCallback();
    this.load(0);
  }

  get roller() {
    return this.getAttribute('roller') || '';
  }

  set roller(v: string) {
    this.setAttribute('roller', v);
    this.prevOffset = -1;
    this.currentOffset = 0;
    this.nextOffset = 0;
    this.load(0);
  }

  private load(offset: number) {
    const req = {
      rollerId: this.roller,
      offset: offset,
    };
    this.rpc.getModeHistory(req).then((resp: GetModeHistoryResponse) => {
      this.history = resp.history!;
      if (offset > this.currentOffset) {
        this.prevOffset = this.currentOffset;
      } else {
        this.prevOffset = offset - this.history.length;
      }
      this.currentOffset = offset;
      this.nextOffset = resp.nextOffset;
      this._render();
    });
  }
}

define('arb-mode-history-sk', ARBModeHistorySk);
