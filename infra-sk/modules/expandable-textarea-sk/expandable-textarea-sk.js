/**
 * @module modules/expandable-textarea-sk
 * @description A custom element that expands a textarea when clicked.
 *
 * @attr {boolean} open - Whether the textarea is expanded.
 *
 * @attr {string} displayText - Clickable text to toggle the textarea.
 *
 * @attr {string} placeholder - Placeholder text for the textarea.
 *
 * @attr {number} minRows - Minimum (and initial) rows in the textarea.
 */

import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';

import '../autogrow-textarea-sk';
import { ElementSk } from '../ElementSk';

import 'elements-sk/collapse-sk';
import 'elements-sk/icon/expand-more-icon-sk';
import 'elements-sk/icon/expand-less-icon-sk';
import 'elements-sk/styles/buttons';

const template = (ele) => html`
<button class=expander @click=${ele._toggle}>
  ${!ele.open
    ? html`<expand-more-icon-sk></expand-more-icon-sk>`
    : html`<expand-less-icon-sk></expand-less-icon-sk>`}${ele.displayText}
</button>
<collapse-sk ?closed=${!ele.open}>
  <autogrow-textarea-sk placeholder=${ele.placeholder}
    minRows=${ele.minRows}></autogrow-textarea-sk>
</collapse-sk>
`;

define('expandable-textarea-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._upgradeProperty('displayText');
    this._upgradeProperty('minRows');
    this._upgradeProperty('open');
    this._upgradeProperty('placeholderText');
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this._collapser = $$('collapse-sk', this);
    this._textarea = $$('autogrow-textarea-sk', this);
  }

  /**
   * @prop {string} value - Content of the textarea element.
   */
  get value() {
    // We back our value with textarea.value directly to avoid issues with the
    // value changing without changing our value property, causing
    // element re-rendering to be skipped.
    return this._textarea.value;
  }

  set value(v) {
    this._textarea.value = v;
  }

  /**
   * @prop {string} placeholder - Placeholder content of the textarea,
   * mirrors the attribute.
   */
  get placeholder() {
    return this.getAttribute('placeholder') || '';
  }

  set placeholder(v) {
    this.setAttribute('placeholder', v);
  }

  /**
   * @prop {number} minRows - Minimum (and initial) number of rows in the
   * textarea, mirrors the attribute.
   */
  get minRows() {
    return +this.getAttribute('minRows');
  }

  set minRows(val) {
    if (val) {
      this.setAttribute('minRows', val);
    } else {
      this.removeAttribute('minRows');
    }
  }

  /**
   * @prop {string} displayText - Clickable text to toggle the textarea, mirrors the attribute.
   */
  get displayText() {
    return this.getAttribute('displayText') || '';
  }

  set displayText(v) {
    this.setAttribute('displayText', v);
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
      $$('autogrow-textarea-sk', this).computeResize();
      $$('textarea', this._textarea).focus();
    }
    this._render();
  }
});
