/**
 * @module modules/triage-sk
 * @description <h2><code>triage-sk</code></h2>
 *
 * A custom element that allows labeling a digest as positive, negative or
 * untriaged.
 *
 * TODO(kjlubick) The search page and the cluster page hack this when doing bulk triage.
 *   If we can handle bulk triage in this element, that will simplify things and we can move the
 *   POST request from digest-details-sk here and all triage logic is consolidated.
 *
 * @evt change - Sent when any of the triage buttons are clicked. The new value
 *     will be contained in event.detail (possible values are "untriaged",
 *     "positive" or "negative").
 */

import 'elements-sk/styles/buttons';
import 'elements-sk/icon/check-circle-icon-sk';
import 'elements-sk/icon/cancel-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { Label } from '../rpc_types';

/**
 * The "bulk triage" dialog offers more than the tree options below, so we need triage-sk to
 * support an empty state where no button is toggled.
 */
export type LabelOrEmpty = Label | '';

export class TriageSk extends ElementSk {
  private static template = (el: TriageSk) => html`
    <button class="positive ${el.value === 'positive' ? 'selected' : ''}"
            @click=${() => el.buttonClicked('positive')}
            title="Triage the left-hand image as positive.">
      <check-circle-icon-sk></check-circle-icon-sk>
    </button>
    <button class="negative ${el.value === 'negative' ? 'selected' : ''}"
            @click=${() => el.buttonClicked('negative')}
            title="Triage the left-hand image as negative.">
      <cancel-icon-sk></cancel-icon-sk>
    </button>
    <button class="untriaged ${el.value === 'untriaged' ? 'selected' : ''}"
            @click=${() => el.buttonClicked('untriaged')}
            title="Unset the triage status of the left-hand image.">
      <help-icon-sk></help-icon-sk>
    </button>
  `;

  private _value: LabelOrEmpty = 'untriaged';

  constructor() {
    super(TriageSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  get value(): LabelOrEmpty {
    return this._value;
  }

  set value(newValue: LabelOrEmpty) {
    const validValues: LabelOrEmpty[] = ['', 'untriaged', 'negative', 'positive'];
    if (!validValues.includes(newValue)) {
      throw new RangeError(`Invalid triage-sk value: "${newValue}".`);
    }
    this._value = newValue;
    this._render();
  }

  private buttonClicked(newValue: LabelOrEmpty) {
    if (this.value === newValue) {
      return;
    }
    this.value = newValue;
    this.dispatchEvent(
      new CustomEvent<LabelOrEmpty>('change', { detail: newValue, bubbles: true }),
    );
  }
}

define('triage-sk', TriageSk);
