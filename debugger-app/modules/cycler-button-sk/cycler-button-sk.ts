/**
 * @module modules/cycler-button-sk
 * @description A button initialized with an array and a function
 * Every time it's clicked it calls your function with the next item
 * in the array, looping around indefinitely.
 *
 * @example
 *    <cycler-button-sk .text=${'next'} .list=${[1, 2, 3]} .fn=${(i: number)=>{}}>
 *    </cycler-button-sk>
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

export class CyclerButtonSk extends ElementSk {
  private static template = (ele: CyclerButtonSk) =>
    html`<button @click=${ele._click}>${ele.text}</button>`;

  public text = '';
  public list: number[] = [];
  public fn: (item: number) => void = (n: number) =>{};

  private _index = 0;

  constructor() {
    super(CyclerButtonSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  private _click() {
    this.fn(this.list[this._index]);
    this._index = (this._index + 1) % this.list.length;
  }
};

define('cycler-button-sk', CyclerButtonSk);
