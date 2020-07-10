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

const _row = (id: string, roller: Roller) => html`
  <tr>
    <td>
      <a href="/r/${id}">${roller.childName} into ${roller.parentName}</a>
    </td>
    <td>${roller.mode}</td>
    <td>${roller.numBehind}</td>
    <td>${roller.numFailed}</td>
  </tr>
`;

const _table = (rollers: {[key:string]: Roller}) => html`
  <table>
    <tr>
      <th>Roller ID</th>
      <th>Current Mode</th>
      <th>Num Behind</th>
      <th>Num Failed</th>
    </tr>
    ${Object.keys(rollers).sort().map((id) => _row(id, rollers[id]))}
  </table>
`;

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

define('arb-table-sk', class extends HTMLElement {
  private _rollers: {[key:string]: Roller};

  constructor() {
    super();
    this._rollers = {};
  }

  connectedCallback() {
    upgradeProperty(this, 'rollers');
    this._render();
  }

  get rollers() { return this._rollers; }
  set rollers(val: {[key:string]: Roller}) {
    this._rollers = val;
    this._render();
  }

  _render() {
    render(_table(this._rollers), this, {eventContext: this});
  }
});