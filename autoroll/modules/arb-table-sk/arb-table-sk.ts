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
import { AutoRollMiniStatuses, AutoRollRPCs, GetAutoRollRPCs} from '../rpc';

export class ARBTableSk extends ElementSk {
  private static template = (ele: ARBTableSk) => html`
  <table>
    <tr>
      <th>Roller ID</th>
      <th>Current Mode</th>
      <th>Num Behind</th>
      <th>Num Failed</th>
    </tr>
    ${ele.rollers.statuses?.map((st) => html`
    <tr>
      <td>
        <a href="/r/${st.roller}">${st.childname} into ${st.parentname}</a>
      </td>
      <td>${st.mode}</td>
      <td>${st.numbehind}</td>
      <td>${st.numfailed}</td>
    </tr>
  `)}
  </table>
`;
  private rollers: AutoRollMiniStatuses = {statuses:[]};
  private rpcs: AutoRollRPCs;

  constructor() {
    super(ARBTableSk.template);
    this.rpcs = GetAutoRollRPCs(this);
  }

  connectedCallback() {
    super.connectedCallback();
    this.reload();
  }

  private reload() {
    this.rpcs.view_GetRollers({}).then((rollers: AutoRollMiniStatuses) => {
      this.rollers = rollers;
      this._render();
    });
  }
}

define('arb-table-sk', ARBTableSk);