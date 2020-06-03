/**
 * @module modules/expandable-textarea-sk
 * @description A custom element that expands a textarea when clicked.
 *
 * @attr {boolean} open - Whether the textarea is expanded.
 */

import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import 'elements-sk/collapse-sk';
import 'elements-sk/icon/expand-more-icon-sk';
import 'elements-sk/icon/expand-less-icon-sk';

const template = (ele) => html`
<a href="javascript:void(0);" id="expander"
  @click=${ele._toggle}>
  ${!ele.open
    ? html`<expand-more-icon-sk></expand-more-icon-sk>`
    : html`<expand-less-icon-sk></expand-less-icon-sk>`}${ele.displayText}
</a>
<collapse-sk ?closed=${!ele.open}>
  <textarea rows=5 placeholder=${ele.placeholderText}></textarea>
</collapse-sk>
`;

define('expandable-textarea-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._upgradeProperty('open');
    this._upgradeProperty('displayText');
    this._upgradeProperty('placeholderText');
    this.placeholderText = this.placeholderText || '';
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this._collapser = $$('collapse-sk', this);
  }

  /**
   * @prop {string} value - Content of the textarea element.
   */
  get value() {
    // We back our value with textarea.value directly to avoid issues with the
    // value changing without changing our value property, causing
    // element re-rendering to be skipped.
    return $$('textarea', this._collapser).value;
  }

  set value(v) {
    $$('textarea', this._collapser).value = v;
  }

  /**
   * @prop {string} placeholderText - Placeholder content of the textarea.
   */
  get placeholderText() {
    return this._placeholderText;
  }

  set placeholderText(v) {
    this._placeholderText = v;
  }

  /**
   * @prop {string} displayText - Clickable text to toggle the textarea.
   */
  get displayText() {
    return this._displayText;
  }

  set displayText(v) {
    this._displayText = v;
  }

  /**
   * @prop {boolean} open - State of the expandable panel, mirrors the attribute.
   */
  get open() {
    return this.hasAttribute('open');
  }

  set open(val) {
    if (val) {
      this.setAttribute('open', '');
    } else {
      this.removeAttribute('open');
    }
  }

  _toggle() {
    this._collapser.closed = !this._collapser.closed;
    this.open = !this._collapser.closed;
    if (this.open) {
      $$('textarea', this._collapser).focus();
    }
    this._render();
  }
});
