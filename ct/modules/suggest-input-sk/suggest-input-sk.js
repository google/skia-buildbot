/**
 * @module modules/suggest-input-sk
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

const template = (ele) => html`
<input autocomplete=off
  @focus=${ele._refresh}
  @input=${ele._refresh}
  @keyup=${ele._keyup}
  @blur=${ele._blur}>
</input>
<div ?hidden=${!(ele._suggestions && ele._suggestions.length > 0)} @click=${ele._suggestionClick}>
  <ul>
  ${ele._suggestions.map((s, i) => (ele._suggestionSelected === i
    ? selectedOptionTemplate(s) : optionTemplate(s)))}
  </ul>
</div>
`;

// tabindex so the fields populate FocusEvent.relatedTarget on blur.
const optionTemplate = (option) => html`
<li tabindex=-1 class=suggestion>${option}</li>
`;
const selectedOptionTemplate = (option) => html`
<li tabindex=-1 class="suggestion selected">${option}</li>
`;

const DOWN_ARROW = 40;
const UP_ARROW = 38;
const ENTER = 13;

define('suggest-input-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._options = [];
    this._suggestions = [];
    this._suggestionSelected = -1;

    this._upgradeProperty('options');
    this._upgradeProperty('acceptCustomValue');
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
   * @prop {Array<string>} options - Values for suggestion list.
   */
  get options() {
    return this._options;
  }

  set options(o) {
    this._options = o;
  }

  /**
   * @prop {Boolean} acceptCustomValue - Mirrors the
   * 'accept-custom-value' attribute.
   */
  get acceptCustomValue() {
    return this.hasAttribute('accept-custom-value');
  }

  set acceptCustomValue(val) {
    if (val) {
      this.setAttribute('accept-custom-value', '');
    } else {
      this.removeAttribute('accept-custom-value');
    }
  }

  _blur(e) {
    // Ignore if this blur is preceding _suggestionClick.
    const blurredElem = e.relatedTarget;
    if (blurredElem && blurredElem.classList.contains('suggestion')) {
      return;
    }
    this._commit();
  }

  _commit() {
    if (this._suggestionSelected > -1) {
      this.value = this._suggestions[this._suggestionSelected];
    } else if (!this._options.includes(this.value) && !this.acceptCustomValue) {
      this.value = '';
    }
    this._suggestions = [];
    this._suggestionSelected = -1;
    this._render();
  }

  _keyup(e) {
    // Allow the user to scroll through suggestions using arrow keys.
    const len = this._suggestions.length;
    const key = e.key || e.keyCode;
    if ((key === 'ArrowDown' || key === DOWN_ARROW) && len > 0) {
      this._suggestionSelected = (this._suggestionSelected + 1) % len;
      this._render();
    } else if ((key === 'ArrowUp' || key === UP_ARROW) && len > 0) {
      this._suggestionSelected = (this._suggestionSelected + len - 1) % len;
      this._render();
    } else if (key === 'Enter' || key === ENTER) {
      // This also commits the current selection (if present) or custom
      // value (if allowed).
      $$('input', this).blur();
    }
  }

  _refresh() {
    const v = this.value;
    let re;
    try {
      re = new RegExp(v, 'i'); // case-insensitive.
    } catch (err) {
      // If the user enters an invalid expression, just use substring
      // match.
      re = {
        test: function(str) {
          return str.indexOf(v) !== -1;
        },
      };
    }
    this._suggestions = this._options.filter((s) => re.test(s));
    this._suggestionSelected = -1;
    this._render();
  }

  _suggestionClick(e) {
    const item = e.target;
    if (item.tagName !== 'LI') {
      return;
    }
    const index = Array.from(item.parentNode.children).indexOf(item);
    this._suggestionSelected = index;
    this._commit();
  }
});
