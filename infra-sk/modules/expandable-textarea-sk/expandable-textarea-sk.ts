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

import { define } from 'elements-sk/define';
import { html } from 'lit-html';

import '../autogrow-textarea-sk';
import { AutogrowTextareaSk } from '../autogrow-textarea-sk/autogrow-textarea-sk';
import { ElementSk } from '../ElementSk';

import { CollapseSk } from 'elements-sk/collapse-sk/collapse-sk';
import 'elements-sk/collapse-sk';
import 'elements-sk/icon/expand-more-icon-sk';
import 'elements-sk/icon/expand-less-icon-sk';
import 'elements-sk/styles/buttons';

export class ExpandableTextareaSk extends ElementSk {
  private static template = (ele: ExpandableTextareaSk) => html`
    <button class=expander @click=${ele.toggle}>
      ${!ele.open
          ? html`<expand-more-icon-sk></expand-more-icon-sk>`
          : html`<expand-less-icon-sk></expand-less-icon-sk>`}${ele.displayText}
    </button>
    <collapse-sk ?closed=${!ele.open}>
      <autogrow-textarea-sk placeholder=${ele.placeholder}
        minRows=${ele.minRows}></autogrow-textarea-sk>
    </collapse-sk>
  `;

  private collapseSk: CollapseSk | null = null;
  private autogrowTextareaSk: AutogrowTextareaSk | null = null;

  constructor() {
    super(ExpandableTextareaSk.template);

    this._upgradeProperty('displayText');
    this._upgradeProperty('minRows');
    this._upgradeProperty('open');
    this._upgradeProperty('placeholderText');
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this.collapseSk = this.querySelector('collapse-sk');
    this.autogrowTextareaSk = this.querySelector('autogrow-textarea-sk');
  }

  /** Content of the textarea element. */
  get value(): string {
    // We back our value with textarea.value directly to avoid issues with the
    // value changing without changing our value property, causing
    // element re-rendering to be skipped.
    return this.autogrowTextareaSk!.value;
  }

  set value(v: string) {
    this.autogrowTextareaSk!.value = v;
  }

  /** Placeholder content of the textarea, mirrors the attribute. */
  get placeholder(): string {
    return this.getAttribute('placeholder') || '';
  }

  set placeholder(v: string) {
    this.setAttribute('placeholder', v);
  }

  /** Minimum (and initial) number of rows in the textarea, mirrors the attribute. */
  get minRows(): number {
    return +this.getAttribute('minRows')!;
  }

  set minRows(val: number) {
    if (val) {
      this.setAttribute('minRows', val.toString());
    } else {
      this.removeAttribute('minRows');
    }
  }

  /** Clickable text to toggle the textarea, mirrors the attribute. */
  get displayText(): string {
    return this.getAttribute('displayText') || '';
  }

  set displayText(v: string) {
    this.setAttribute('displayText', v);
  }

  /** State of the expandable panel, mirrors the attribute. */
  get open(): boolean {
    return this.hasAttribute('open');
  }

  set open(val: boolean) {
    if (val) {
      this.setAttribute('open', '');
    } else {
      this.removeAttribute('open');
    }
  }

  private toggle() {
    this.collapseSk!.closed = !this.collapseSk!.closed;
    this.open = !this.collapseSk!.closed;
    if (this.open) {
      this.autogrowTextareaSk!.computeResize();
      this.autogrowTextareaSk!.querySelector('textarea')!.focus();
    }
    this._render();
  }
}

define('expandable-textarea-sk', ExpandableTextareaSk);
