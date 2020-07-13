/**
 * @module autoroll/modules/arb-table-sk
 * @description <h2><code>arb-table-sk</code></h2>
 *
 * <p>
 * This element displays the list of active Autorollers.
 * </p>
 */

import { html, render } from 'lit-html'

import { define } from 'elements-sk/define'
import 'elements-sk/styles/table';
import { upgradeProperty } from 'elements-sk/upgradeProperty'


class Roller {
  mode: string;
  childName: string;
  parentName: string;
  numBehind: number;
  numFailed: number;

  constructor(){
    this.mode = "";
    this.childName = "";
    this.parentName = "";
    this.numBehind = 0;
    this.numFailed = 0;
  }
}

class ARBTableSk extends HTMLElement {
  private static template = (rollers: {[key:string]: Roller}) => html`
  <table>
    <tr>
      <th>Roller ID</th>
      <th>Current Mode</th>
      <th>Num Behind</th>
      <th>Num Failed</th>
    </tr>
    ${Object.keys(rollers).sort().map((id) => html`
    <tr>
      <td>
        <a href="/r/${id}">${rollers[id].childName} into ${rollers[id].parentName}</a>
      </td>
      <td>${rollers[id].mode}</td>
      <td>${rollers[id].numBehind}</td>
      <td>${rollers[id].numFailed}</td>
    </tr>
  `)}
  </table>
`;
  private _rollers: {[key:string]: Roller} = {};

  connectedCallback() {
    upgradeProperty(this, 'rollers');
    this.render();
  }

  get rollers() { return this._rollers; }
  set rollers(val: {[key:string]: Roller}) {
    this._rollers = val;
    this.render();
  }

  private render() {
    render(ARBTableSk.template(this._rollers), this, {eventContext: this});
  }
}

define('arb-table-sk', ARBTableSk);