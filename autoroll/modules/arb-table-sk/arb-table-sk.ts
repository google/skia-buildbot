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


export class Roller {
  mode: string = "";
  childName: string = "";
  parentName: string = "";
  numBehind: number = 0;
  numFailed: number = 0;
}

export class ARBTableSk extends ElementSk {
  private static template = (ele: ARBTableSk) => html`
  <table>
    <tr>
      <th>Roller ID</th>
      <th>Current Mode</th>
      <th>Num Behind</th>
      <th>Num Failed</th>
    </tr>
    ${Object.keys(ele.rollers).sort().map((id) => html`
    <tr>
      <td>
        <a href="/r/${id}">${ele.rollers[id].childName} into ${ele.rollers[id].parentName}</a>
      </td>
      <td>${ele.rollers[id].mode}</td>
      <td>${ele.rollers[id].numBehind}</td>
      <td>${ele.rollers[id].numFailed}</td>
    </tr>
  `)}
  </table>
`;
  private _rollers: {[key:string]: Roller} = {};

  constructor() {
    super(ARBTableSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._upgradeProperty('rollers');
    this._render();
  }

  get rollers() { return this._rollers; }
  set rollers(val: {[key:string]: Roller}) {
    this._rollers = val;
    this._render();
  }
}

define('arb-table-sk', ARBTableSk);