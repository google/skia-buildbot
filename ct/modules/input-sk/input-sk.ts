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

const template = (ele) => html`
<div class=input-container>
  <span>${ele.textPrefix}</span>
  <input autocomplete=off required
    @focus=${ele._refresh}
    @input=${ele._refresh}
    @keyup=${ele._keyup}
    @blur=${ele._blur}
    type=${ifDefined(ele.type)}>
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
    this._upgradeProperty('type');
    this._upgradeProperty('textPrefix');
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    if (this.hasAttribute('value')) {
      this.value = this.getAttribute('value');
    }
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
   * @prop {string} label - Label to display to guide user input.
   * Mirrors the attribute.
   */
  get label() {
    return this.getAttribute('label') || '';
  }

  set label(val) {
    this.setAttribute('label', val);
    this._render();
  }

  /**
   * @prop {string} type - Type of input.  Mirrors the attribute.
   */
  get type() {
    // We use undefined because it's what ifDefined uses.
    return this.getAttribute('type') || undefined;
  }

  set type(val) {
    this.setAttribute('type', val);
    this._render();
  }

  /**
   * @prop {string} textPrefix - Optional prefix to put before the input box.
   * Mirrors the attribute.
   */
  get textPrefix() {
    return this.getAttribute('textPrefix') || '';
  }

  set textPrefix(val) {
    this.setAttribute('type', val);
    this._render();
  }
});
