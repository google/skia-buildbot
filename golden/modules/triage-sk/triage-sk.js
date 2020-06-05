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

// The "bulk triage" dialog offers more than the tree options below, so we need
// triage-sk to support an empty state where no button is toggled.
const NONE = '';
const POSITIVE = 'positive';
const NEGATIVE = 'negative';
const UNTRIAGED = 'untriaged';

const template = (el) => html`
<button class="positive ${el.value === POSITIVE ? 'selected' : ''}"
        @click=${() => el._buttonClicked(POSITIVE)}>
  <check-circle-icon-sk></check-circle-icon-sk>
</button>
<button class="negative ${el.value === NEGATIVE ? 'selected' : ''}"
        @click=${() => el._buttonClicked(NEGATIVE)}>
  <cancel-icon-sk></cancel-icon-sk>
</button>
<button class="untriaged ${el.value === UNTRIAGED ? 'selected' : ''}"
        @click=${() => el._buttonClicked(UNTRIAGED)}>
  <help-icon-sk></help-icon-sk>
</button>
`;

define('triage-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._value = UNTRIAGED;
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  /** @prop value {string} One of "untriaged", "positive" or "negative". */
  get value() {
    return this._value;
  }

  set value(newValue) {
    if (![NONE, POSITIVE, NEGATIVE, UNTRIAGED].includes(newValue)) {
      throw new RangeError(`Invalid triage-sk value: "${newValue}".`);
    }
    this._value = newValue;
    this._render();
  }

  _buttonClicked(newValue) {
    if (this.value === newValue) {
      return;
    }
    this.value = newValue;
    this.dispatchEvent(
      new CustomEvent('change', { detail: newValue, bubbles: true }),
    );
  }
});
