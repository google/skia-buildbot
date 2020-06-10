/**
 * @module modules/autogrow-textarea-sk
 * @description A custom element wrapping a textarea with logic to dynamically
 * adjust number of rows to fit its content
 *
 * @attr {string} placeholder - Placeholder text for the textarea.
 *
 * @attr {number} minRows - Minimum (and initial) rows in the textarea.
 */

import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';

import { ElementSk } from '../ElementSk';

import 'elements-sk/collapse-sk';
import 'elements-sk/icon/expand-more-icon-sk';
import 'elements-sk/icon/expand-less-icon-sk';

const defaultRows = 5;
const template = (ele) => html`
<textarea placeholder=${ele.placeholder} @input=${ele._computeResize}></textarea>
`;

define('autogrow-textarea-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._upgradeProperty('placeholder');
    this._upgradeProperty('minRows');
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this._computeResize();
  }

  _render() {
    super._render();
    this._textarea = $$('textarea', this);
  }

  /**
   * @prop {string} value - Content of the textarea element.
   */
  get value() {
    // We back our value with textarea.value directly to avoid issues with
    // the value changing without changing our value property, causing
    // element re-rendering to be skipped.
    return this._textarea.value;
  }

  set value(v) {
    this._textarea.value = v;
    this._computeResize();
  }

  /**
   * @prop {string} placeholder - Placeholder content of the textarea,
   * mirrors the attribute. Returns empty string when not set, for convenience
   * in passing to child elements in templates.
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
    return (+this.getAttribute('minRows') || defaultRows);
  }

  set minRows(val) {
    if (val) {
      this.setAttribute('minRows', val);
    } else {
      this.removeAttribute('minRows');
    }
  }

  _computeResize() {
    // Rather than increment/decrement, we just set rows each time
    // to handle copy and paste of multiple lines cleanly.
    this._textarea.rows = this.minRows;
    const heightDiff = this._textarea.scrollHeight - this._textarea.clientHeight;
    if (heightDiff > 0) {
      // We floor the rowHeight as a lazy way to counteract rounded results
      // returned from clientHeight and scrollHeight causing too few rows added.
      const rowHeight = Math.floor(
        this._textarea.clientHeight / this._textarea.rows,
      );
      this._textarea.rows += Math.ceil(heightDiff / rowHeight);
    }
  }
});
