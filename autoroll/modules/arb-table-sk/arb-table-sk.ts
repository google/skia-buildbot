/**
 * @module autoroll/modules/arb-table-sk
 * @description <h2><code>arb-table-sk</code></h2>
 *
 * <p>
 * This element displays the list of active Autorollers.
 * </p>
 */
import { define } from 'elements-sk/define'
import { html, render } from 'lit-html'
import { $$ } from 'common-sk/modules/dom'
import { upgradeProperty } from 'elements-sk/upgradeProperty'

const roller = (ele: ArbTableSK, roller: Roller) => html`
  <div class="tr">
    <div class="td"><a href="/r/${roller}">${ele._name(roller)}</a></div>
    <div class="td">${ele._mode(roller)}</div>
    <div class="td">${ele._numBehind(roller)}</div>
    <div class="td">${ele._numFailed(roller)}</div>
  </div>
`;

const table = (ele: ArbTableSK, rollers: Roller[]) => html`
  <style include="styles-sk">
  div.table{
    margin: 20px;
  }
  </style>
  <div class="table">
    <div class="th">Roller ID</div>
    <div class="th">Current Mode</div>
    <div class="th">Num Behind</div>
    <div class="th">Num Failed</div>
    ${rollers.map((r: Roller) => roller(ele, r))}
  </div>
`;

class Roller {

}

class ArbTableSK extends HTMLElement {
  private _rollers: Roller[];
  private _statuses: 

  constructor() {
    super();
    this._rollers = [];
  }

  connectedCallback() {
    upgradeProperty(this, 'rollers');
    this._render();
  }

  get rollers() { return this._rollers; }
  set rollers(val: Roller[]) {
    this._rollers = val;
    this._render();
  }

  _mode(roller: Roller) {
    return this._statuses[roller].mode;
  }

  _name(roller: Roller) {
    return this._statuses[roller].childName + " into " + this._statuses[roller].parentName;
  }

  _numBehind(roller) {
    return this._statuses[roller].numBehind;
  }

  _numFailed(roller) {
    return this._statuses[roller].numFailed;
  }
}

define('arb-table-sk', ArbTableSK);