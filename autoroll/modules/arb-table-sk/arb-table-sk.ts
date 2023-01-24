/**
 * @module autoroll/modules/arb-table-sk
 * @description <h2><code>arb-table-sk</code></h2>
 *
 * <p>
 * This element displays the list of active Autorollers.
 * </p>
 */

import { html } from 'lit-html';
import { $$ } from 'common-sk/modules/dom';
import { HintableObject } from 'common-sk/modules/hintable';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { define } from 'elements-sk/define';
import 'elements-sk/styles/table';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
  AutoRollMiniStatus,
  AutoRollService,
  GetAutoRollService,
  GetRollersResponse,
  Mode,
} from '../rpc';
import { GetLastCheckInTime, LastCheckInSpan } from '../utils';

/**
 * hideOutdatedRollersThreshold is the threshold at which we'll stop displaying
 * a roller in the table by default. It can still be found if the user provides
 * their own filter or if they visit the roller's status page directly.
 */
const hideOutdatedRollersThreshold = 7 * 24 * 60 * 60 * 1000; // 7 days.

class State {
  filter: string = ''; // Regular expression used to filter rollers.
}

export class ARBTableSk extends ElementSk {
  private static template = (ele: ARBTableSk) => html`
  <div>
    Filter: <input type="text"
        value="${ele.filter}"
        @input="${(e: InputEvent) => {ele.filter = (e.target as HTMLInputElement).value}}"
        ></input>
  </div>
  <table>
    <tr>
      <th>Roller ID</th>
      <th>Current Mode</th>
      <th>Num Behind</th>
      <th>Num Failed</th>
      <th></th>
    </tr>
    ${ele.filtered.map(
      (st) => html`
        <tr>
          <td>
            <a href="/r/${st.rollerId}"
              >${st.childName} into ${st.parentName}</a
            >
          </td>
          <td class="${ele.modeClass(st.mode)}">${st.mode.toLowerCase()}</td>
          <td>${st.numBehind}</td>
          <td>${st.numFailed}</td>
          <td>${LastCheckInSpan(st)}</td>
        </tr>
      `,
    )}
  </table>
`;

  private rollers: AutoRollMiniStatus[] = [];
  private filtered: AutoRollMiniStatus[] = [];
  private rpc: AutoRollService;
  private state: State = {
    filter: '',
  };
  private stateHasChanged = () => {};

  constructor() {
    super(ARBTableSk.template);
    this.rpc = GetAutoRollService(this);
  }

  get filter(): string {
    return this.state.filter;
  }
  set filter(filter: string) {
    this.state.filter = filter;
    this.stateHasChanged();
  }

  connectedCallback() {
    super.connectedCallback();
    this.stateHasChanged = stateReflector(
      /* getState */ () => (this.state as unknown) as HintableObject,
      /* setState */ (newState) => {
        this.state = (newState as unknown) as State;
        this.updateFiltered();
      },
    );
    this.reload();
  }

  private modeClass(mode: Mode) {
    switch (mode) {
      case Mode.RUNNING:
        return "fg-running";
      case Mode.DRY_RUN:
        return "fg-dry-run";
      case Mode.STOPPED:
        return "fg-stopped";
      case Mode.OFFLINE:
        return "fg-offline";
    }
  }

  private reload() {
    this.rpc.getRollers({}).then((resp: GetRollersResponse) => {
      this.rollers = resp.rollers!;
      this.updateFiltered();
    });
  }

  private updateFiltered() {
    this.filtered = this.rollers;
    if (this.filter) {
      // If a filter was provided in the text box, use that.
      const regex = new RegExp(this.filter);
      this.filtered = this.rollers.filter((st: AutoRollMiniStatus) => (
        st.rollerId.match(regex)
        || st.childName.match(regex)
        || st.parentName.match(regex)
      ));
    } else {
      // If no filter was provided, filter out any rollers which have not
      // checked in for longer than hideOutdatedRollersThreshold.
      this.filtered = this.rollers.filter((st: AutoRollMiniStatus) => {
        const lastCheckedIn = GetLastCheckInTime(st).getTime();
        const now = new Date().getTime()
        if (now - lastCheckedIn > hideOutdatedRollersThreshold) {
          return false;
        }
        return true;
      });
    }
    this._render();
  }
}

define('arb-table-sk', ARBTableSk);
