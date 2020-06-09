/**
 * @module modules/input-sk
 * @description A custom element that is a styled, labeled input.
 *
 * @attr {Boolean} label - Placeholder style text that moves out of the way
 * when element is in focus.
 */

import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';

const template = (ele) => html`
<div class=input-container>
  <input autocomplete=off required
    @focus=${ele._refresh}
    @input=${ele._refresh}
    @keyup=${ele._keyup}
    @blur=${ele._blur}>
  </input>
  <label>${ele.label}</label>
  <div class=underline-container>
    <div class=underline></div>
    <div class=underline-background ></div>
  </div>
</div>
`;

define('input-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._upgradeProperty('label');
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  /**
   * @prop {string} value - Content of the input element from typing,
   * selection, etc.
   */
  get value() {
    // We back our value with input.value directly, to avoid issues with the
    // input value changing without changing our value property, causing
    // element re-rendering to be skipped.
    return $$('input', this).value;
  }

  set value(v) {
    $$('input', this).value = v;
  }

  /**
   * @prop {Array<string>} label - Label to display to guide user input.
   * Mirrors the attribute.
   */
  get label() {
    return this.getAttribute('label') || '';
  }

  set label(val) {
    this.setAttribute('label', val);
    this._render();
  }
});
