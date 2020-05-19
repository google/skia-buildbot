/**
 * @fileoverview A custom element that implements regex and substring match
 * suggestions. These are selectable via click or up/down/enter.
 *
 *   Properties:
 *      options: Array<string> values for suggestion list.source.source
 *      value: String, content of input element from typing, selection, etc.
 *      acceptCustomValue: Boolean, html attribute 'accept-custom-value', user
 *                         can enter value not in options list.
 */

import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';

const template = (ele) => html`
    <input autocomplete=off .value="${ele._value}" @input=${ele._refresh} @keyup=${ele._keyup} @focus=${ele._refresh} @blur=${ele._blur}></input>
    <div ?hidden=${!ele._show_suggestions()} @click=${ele._suggestion_click}>
      <ul style="list-style-type:none;">
      ${ele._buildSuggestionList()}
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
    this.displayOptionsOnFocus = true;
    this._value = '';
    this._suggestions = [];
    this._suggestion_selected = -1;
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  // We back our value with input.value directly, to avoid issues with the
  // input value changing without changing our value property, causing
  // element re-rendering to be skipped.
  get value() {
    return $$('input', this).value;
  }

  set value(v) {
    $$('input', this).value = v;
  }

  get options() {
    return this._options;
  }

  set options(o) {
    this._options = o;
  }

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

  _buildSuggestionList() {
    const templates = [];
    for (let i = 0; i < this._suggestions.length; ++i) {
      const s = this._suggestions[i];
      templates.push(i === this._suggestion_selected ? selectedOptionTemplate(s) : optionTemplate(s));
    }
    return templates;
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
    this._suggestion_selected = -1;
    this._render();
  }

  _keyup(e) {
    // Allow the user to scroll through suggestions using arrow keys.
    const len = this._suggestions.length;
    if (e.keyCode === DOWN_ARROW && len > 0) {
      this._suggestion_selected = (this._suggestion_selected + 1) % len;
      this._render();
    } else if (e.keyCode === UP_ARROW && len) {
      this._suggestion_selected = (this._suggestion_selected + len - 1) % len;
      this._render();
    } else if (e.keyCode === ENTER) {
      // This also commits the current selection (if present) or custom
      // value (if allowed).
      $$('input', this).blur();
    }
  }

  _suggestion_click(e) {
    const item = e.target;
    if (item.tagName !== 'LI') {
      return;
    }
    const index = Array.from(item.parentNode.children).indexOf(item);
    this._suggestion_selected = index;
    this._commit();
  }

  _blur(e) {
    // Ignore if this blur is preceding _suggestion_click.
    const blurredElem = e.relatedTarget;
    if (blurredElem && blurredElem.classList.contains('suggestion')) {
      return;
    }
    this._commit();
  }

  _commit() {
    if (this._suggestion_selected > -1) {
      this.value = this._suggestions[this._suggestion_selected];
    } else if (!this._options.includes(this.value) && !this.allowCustomValue) {
      this.value = '';
    }
    this._suggestions = [];
    this._suggestion_selected = -1;
    this._render();
  }

  _show_suggestions() {
    return this._suggestions && this._suggestions.length > 0;
  }
});
