/**
 * @module modules/clipboard-sk
 * @description <h2><code>clipboard-sk</code></h2>
 *
 * Displays a copy-content icon and when clicked copies the contents of the
 * 'value' attribute into the user's clipboard. Also displays a tooltip letting
 * the user know the value was copied.
 *
 * If the value to be copied is expensive to calculate then compute the value in
 * the `calculatedValue` function. See `clipboard-sk-demo.ts` for an example.
 *
 * @attr value - The content to put into the clipboard.
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { $$ } from 'common-sk/modules/dom';
import { ElementSk } from '../ElementSk';

import 'elements-sk/icon/content-copy-icon-sk';
import '../tooltip-sk';
import { TooltipSk } from '../tooltip-sk/tooltip-sk';

export const defaultToolTipMessage = 'Copy to clipboard';

export const copyCompleteToolTipMessage = 'Copied!';

export const copyFailedToolTipMessage = 'Failed to copy!';

export class ClipboardSk extends ElementSk {
  // We need to assign an id to the content-copy-icon-sk, so that the tooltip-sk
  // has something to use as a target.
  private icon_id: string = `x${`${Math.random()}`.slice(2)}`;

  private tooltip: TooltipSk | null = null;

  /** If the value to be copied is expensive to calculate then compute the value
    * in the `calculatedValue` function. See `clipboard-sk-demo.ts` for an
    * example.
    * */
  calculatedValue: (()=> Promise<string>) | null = null;

  constructor() {
    super(ClipboardSk.template);
  }

  private static template = (ele: ClipboardSk) => html`
  <content-copy-icon-sk
    id=${ele.icon_id}
    @click=${() => ele.copyToClipboard()}>
    @mouseleave=${() => ele.restoreToolTipMessage()}
  </content-copy-icon-sk>
  <tooltip-sk
    target=${ele.icon_id}
    value=${defaultToolTipMessage}>
  </tooltip-sk>`;

  connectedCallback(): void {
    super.connectedCallback();
    this._upgradeProperty('value');
    this._render();
    this.tooltip = $$('tooltip-sk', this);
  }

  private async copyToClipboard(): Promise<void> {
    try {
      if (this.calculatedValue !== null) {
        this.value = await this.calculatedValue();
      }
      await navigator.clipboard.writeText(this.value);
      this.tooltip!.value = copyCompleteToolTipMessage;
    } catch (error) {
      this.tooltip!.value = copyFailedToolTipMessage;
    }
    this._render();
  }

  private restoreToolTipMessage(): void {
    this.tooltip!.value = defaultToolTipMessage;
    this._render();
  }

  /** @prop value {string} The content to put into the clipboard. */
  get value(): string { return this.getAttribute('value') || ''; }

  set value(val: string) { this.setAttribute('value', val); }
}

define('clipboard-sk', ClipboardSk);
