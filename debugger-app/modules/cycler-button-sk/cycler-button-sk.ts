/**
 * @module modules/cycler-button-sk
 * @description A button initialized with an array of numbers.
 * Every time it's clicked it emits an event with the next item
 * in the array, looping around indefinitely.
 *
 * @evt next-item: emitted when clicked. detail.item contains one of the numbers
 *        from the list.
 *
 * @example
 *    <cycler-button-sk .text=${'next'} .list=${[1, 2, 3]} @next-item=${(e: Event)=>{}}>
 *    </cycler-button-sk>
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

export interface CyclerButtonNextItemEventDetail {
  item: number;
}

export class CyclerButtonSk extends ElementSk {
  private static template = (ele: CyclerButtonSk) => html`<button @click=${ele._click}>${ele.text}</button>`;

  public text = '';

  public list: number[] = [];

  private _index = 0;

  constructor() {
    super(CyclerButtonSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  private _click() {
    this.dispatchEvent(
      new CustomEvent<CyclerButtonNextItemEventDetail>(
        'next-item', {
          detail: { item: this.list[this._index] },
          bubbles: true,
        },
      ),
    );
    this._index = (this._index + 1) % this.list.length;
  }
}

define('cycler-button-sk', CyclerButtonSk);
