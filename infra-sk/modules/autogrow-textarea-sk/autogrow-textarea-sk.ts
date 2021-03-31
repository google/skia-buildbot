/**
 * @module modules/autogrow-textarea-sk
 * @description A custom element wrapping a textarea with logic to dynamically
 * adjust number of rows to fit its content
 *
 * @attr {string} placeholder - Placeholder text for the textarea.
 *
 * @attr {number} minRows - Minimum (and initial) rows in the textarea.
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';

import { ElementSk } from '../ElementSk';

import 'elements-sk/collapse-sk';
import 'elements-sk/icon/expand-more-icon-sk';
import 'elements-sk/icon/expand-less-icon-sk';

const defaultRows = 5;

export class AutogrowTextareaSk extends ElementSk {
  private static template = (ele: AutogrowTextareaSk) => html`
    <textarea placeholder=${ele.placeholder} @input=${ele.computeResize}></textarea>
  `;

  private textarea: HTMLTextAreaElement | null = null;

  constructor() {
    super(AutogrowTextareaSk.template);

    this._upgradeProperty('placeholder');
    this._upgradeProperty('minRows');
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this.computeResize();
  }

  _render() {
    super._render();
    this.textarea = this.querySelector('textarea');
  }

  /** Content of the textarea element. */
  get value(): string {
    // We back our value with textarea.value directly to avoid issues with
    // the value changing without changing our value property, causing
    // element re-rendering to be skipped.
    return this.textarea!.value;
  }

  set value(v: string) {
    this.textarea!.value = v;
    this.computeResize();
  }

  /**
   * Placeholder content of the textarea, mirrors the attribute. Returns empty string when not set,
   * for convenience in passing to child elements in templates.
   */
  get placeholder(): string {
    return this.getAttribute('placeholder') || '';
  }

  set placeholder(v: string) {
    this.setAttribute('placeholder', v);
    this._render();
  }

  /** Minimum (and initial) number of rows in the textarea, mirrors the attribute. */
  get minRows(): number {
    return (+this.getAttribute('minRows')! || defaultRows);
  }

  set minRows(val: number) {
    if (val) {
      this.setAttribute('minRows', val.toString());
    } else {
      this.removeAttribute('minRows');
    }
    this.computeResize();
  }

  /**
   * Adjusts the textarea to vertically fit it's contents.
   * May need to be manually called if this.value is set before
   * this object is visible (e.g if it's collapsed).
   */
  computeResize() {
    if (!this.textarea) return;

    // Rather than increment/decrement, we just set rows each time
    // to handle copy and paste of multiple lines cleanly.
    this.textarea.rows = this.minRows;
    const heightDiff = this.textarea.scrollHeight - this.textarea.clientHeight;
    if (heightDiff > 0) {
      // We floor the rowHeight as a lazy way to counteract rounded results
      // returned from clientHeight and scrollHeight causing too few rows added.
      const rowHeight = Math.floor(
        this.textarea.clientHeight / this.textarea.rows,
      );
      this.textarea.rows += Math.ceil(heightDiff / rowHeight);
    }
  }
}

define('autogrow-textarea-sk', AutogrowTextareaSk);
