/**
 * @module modules/triage-sk
 * @description <h2><code>triage-sk</code></h2>
 *
 * A custom element that allows labeling a digest as positive, negative or
 * untriaged.
 *
 * @evt change - Sent when any of the triage buttons are clicked. The new value
 *     will be contained in event.detail (possible values are "untriaged",
 *     "positive" or "negative").
 */

import '../../../elements-sk/modules/icons/check-circle-icon-sk';
import '../../../elements-sk/modules/icons/cancel-icon-sk';
import '../../../elements-sk/modules/icons/help-icon-sk';
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { Label } from '../rpc_types';

export class TriageSk extends ElementSk {
  private static template = (el: TriageSk) => html`
    <button
      class="positive ${el.value === 'positive' ? 'selected' : ''}"
      @click=${() => el.buttonClicked('positive')}
      ?disabled=${el._readOnly}
      title="Triage the left-hand image as positive.">
      <check-circle-icon-sk></check-circle-icon-sk>
    </button>
    <button
      class="negative ${el.value === 'negative' ? 'selected' : ''}"
      @click=${() => el.buttonClicked('negative')}
      ?disabled=${el._readOnly}
      title="Triage the left-hand image as negative.">
      <cancel-icon-sk></cancel-icon-sk>
    </button>
    <button
      class="untriaged ${el.value === 'untriaged' ? 'selected' : ''}"
      @click=${() => el.buttonClicked('untriaged')}
      ?disabled=${el._readOnly}
      title="Unset the triage status of the left-hand image.">
      <help-icon-sk></help-icon-sk>
    </button>
  `;

  private _value: Label = 'untriaged';

  private _readOnly = false;

  constructor() {
    super(TriageSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  get value(): Label {
    return this._value;
  }

  set value(newValue: Label) {
    this._value = newValue;
    this._render();
  }

  get readOnly(): boolean {
    return this._readOnly;
  }

  set readOnly(val: boolean) {
    this._readOnly = val;
    this._render();
  }

  private buttonClicked(newValue: Label) {
    if (this.value === newValue) {
      return;
    }
    this.value = newValue;
    this.dispatchEvent(
      new CustomEvent<Label>('change', { detail: newValue, bubbles: true })
    );
  }
}

define('triage-sk', TriageSk);
