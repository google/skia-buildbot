/**
 * @module modules/input-sk
 * @description A custom element that is a styled, labeled input.
 * TODO(westont): Move this to infra-sk.
 *
 * @attr {Boolean} label - Placeholder style text that moves out of the way
 * when element is in focus.
 * @attr {string} type - Passed to underlying <input>. e.g. 'number'
 * @attr {string} textPrefix - Optional prefix to put before the input box.
 */

import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ifDefined } from 'lit-html/directives/if-defined';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';

export class InputSk extends ElementSk {
  constructor() {
    super(InputSk.template);
    this._upgradeProperty('label');
    this._upgradeProperty('type');
    this._upgradeProperty('textPrefix');
  }

  private static template = (ele: InputSk) => html`
<div class=input-container>
  <span>${ele.textPrefix}</span>
  <input autocomplete=off required
    type=${ifDefined(ele.type)}>
  </input>
  <label>${ele.label}</label>
  <div class=underline-container>
    <div class=underline></div>
    <div class=underline-background ></div>
  </div>
</div>
`;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    if (this.hasAttribute('value')) {
      this.value = this.getAttribute('value')!;
    }
  }

  /**
   * @prop {string} value - Content of the input element from typing,
   * selection, etc.
   */
  get value(): string {
    // We back our value with input.value directly, to avoid issues with the
    // input value changing without changing our value property, causing
    // element re-rendering to be skipped.
    return ($$('input', this)! as HTMLInputElement).value;
  }

  set value(v: string) {
    ($$('input', this)! as HTMLInputElement).value = v;
  }

  /**
   * @prop {string} label - Label to display to guide user input.
   * Mirrors the attribute.
   */
  get label(): string {
    return this.getAttribute('label') || '';
  }

  set label(val: string) {
    this.setAttribute('label', val);
    this._render();
  }

  /**
   * @prop {string} type - Type of input.  Mirrors the attribute.
   */
  get type(): string {
    return this.getAttribute('type') || '';
  }

  set type(val: string) {
    this.setAttribute('type', val);
    this._render();
  }

  /**
   * @prop {string} textPrefix - Optional prefix to put before the input box.
   * Mirrors the attribute.
   */
  get textPrefix(): string {
    return this.getAttribute('textPrefix') || '';
  }

  set textPrefix(val: string) {
    this.setAttribute('type', val);
    this._render();
  }
}

define('input-sk', InputSk);
