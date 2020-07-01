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
import { upgradeProperty } from 'elements-sk/upgradeProperty'

const _row = (roller: Roller) => html`
  <div class="tr">
    <div class="td">
      <a href="/r/${roller.id}">${roller.childName} into ${roller.parentName}</a>
    </div>
    <div class="td">${roller.mode}</div>
    <div class="td">${roller.numBehind}</div>
    <div class="td">${roller.numFailed}</div>
  </div>
`;

const _table = (rollers: Roller[]) => html`
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
    ${rollers.map(_row)}
  </div>
`;

class Roller {
  id: string;
  mode: string;
  childName: string;
  parentName: string;
  numBehind: number;
  numFailed: number;

  constructor(){
    this.id = "";
    this.mode = "";
    this.childName = "";
    this.parentName = "";
    this.numBehind = 0;
    this.numFailed = 0;
  }
}

define('arb-table-sk', class extends HTMLElement {
  private _rollers: Roller[];

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

  _render() {
    render(_table(this._rollers), this, {eventContext: this});
  }
});