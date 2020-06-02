/**
 * @module modules/expandable-textarea-sk
 * @description A custom element that implements regex and substring match
 * suggestions. These are selectable via click or up/down/enter.
 *
 * @attr {Boolean} accept-custom-value - Whether users can enter values not listed
 * in this.options.
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
  ${!ele._expanded
    ? html`<expand-more-icon-sk></expand-more-icon-sk>Specify patch manually`
    : html`<expand-less-icon-sk></expand-less-icon-sk>Collapse manual patch`}
</a>
<collapse-sk ?closed=${ele.closed}><textarea rows=5 .placeholder=${ele.placeholderText}></textarea></collapse-sk>
`;

define('expandable-textarea-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._upgradeProperty('value');
    this._upgradeProperty('closed');
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
   * @prop {string} value - Content of the input element from typing,
   * selection, etc.
   */
  get value() {
    // We back our value with input.value directly, to avoid issues with the
    // input value changing without changing our value property, causing
    // element re-rendering to be skipped.
    return $$('textarea', this._collapser).value;
  }

  set value(v) {
    $$('textarea', this._collapser).value = v;
  }

  /**
   * @prop {string} placeholdertext - Content of the input element from typing,
   * selection, etc.
   */
  get placeholderText() {
    // We back our value with input.value directly, to avoid issues with the
    // input value changing without changing our value property, causing
    // element re-rendering to be skipped.
    return this._placeholderText;
  }

  set placeholderText(v) {
    this._placeholderText = v;
  }

  /**
   * @prop {boolean} closed - State of the expandable panel, mirrors the attribute.
   */
  get closed() {
    return this.hasAttribute('closed');
  }

  set closed(val) {
    if (val) {
      this.setAttribute('closed', '');
    } else {
      this.removeAttribute('closed');
    }
  }

  _toggle() {
    this._collapser.closed = !this._collapser.closed;
    if (!this._collapser.closed) {
      $$('textarea', this._collapser).focus();
    }
  }
});
