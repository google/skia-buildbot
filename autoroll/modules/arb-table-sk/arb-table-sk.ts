/**
 * @module autoroll/modules/arb-table-sk
 * @description <h2><code>arb-table-sk</code></h2>
 *
 * <p>
 * This element displays the list of active Autorollers.
 * </p>
 */

import { html } from 'lit-html'

import { define } from 'elements-sk/define'
import 'elements-sk/styles/table';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
  AutoRollMiniStatus,
  AutoRollService,
  GetAutoRollService,
  GetRollersResponse,
} from '../rpc';

export class ARBTableSk extends ElementSk {
  private static template = (ele: ARBTableSk) => html`
  <table>
    <tr>
      <th>Roller ID</th>
      <th>Current Mode</th>
      <th>Num Behind</th>
      <th>Num Failed</th>
    </tr>
    ${ele.rollers?.map((st) => html`
    <tr>
      <td>
        <a href="/r/${st.rollerId}">${st.childName} into ${st.parentName}</a>
      </td>
      <td>${st.mode.toLowerCase()}</td>
      <td>${st.numBehind}</td>
      <td>${st.numFailed}</td>
    </tr>
  `)}
  </table>
`;
  private rollers: AutoRollMiniStatus[] = [];
  private rpc: AutoRollService;

  constructor() {
    super(ARBTableSk.template);
    this.rpc = GetAutoRollService(this);
  }

  connectedCallback() {
    super.connectedCallback();
    this.reload();
  }

  private reload() {
    this.rpc.getRollers({}).then((resp: GetRollersResponse) => {
      this.rollers = resp.rollers!;
      this._render();
    });
  }
}

define('arb-table-sk', ARBTableSk);