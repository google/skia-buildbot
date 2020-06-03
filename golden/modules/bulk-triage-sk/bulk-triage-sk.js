/**
 * @module module/bulk-triage-sk
 * @description <h2><code>bulk-triage-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import 'elements-sk/checkbox-sk';
import 'elements-sk/icon/cancel-icon-sk';
import 'elements-sk/icon/check-circle-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/icon/view-agenda-icon-sk';
import 'elements-sk/styles/buttons';

const POSITIVE = 'positive';
const NEGATIVE = 'negative';
const UNTRIAGED = 'untriaged';
const CLOSEST = 'closest';

const template = (el) => html`
<h2>Bulk Triage</h2>
<p>Assign the status to all images on this page at once.</p>

<div class=status>
  <button class="positive ${el.value === POSITIVE ? 'selected' : ''}"
          @click=${() => el._setDesiredLabel(POSITIVE)}>
    <check-circle-icon-sk></check-circle-icon-sk>
  </button>
  <button class="negative ${el.value === NEGATIVE ? 'selected' : ''}"
          @click=${() => el._setDesiredLabel(NEGATIVE)}>
    <cancel-icon-sk></cancel-icon-sk>
  </button>
  <button class="untriaged ${el.value === UNTRIAGED ? 'selected' : ''}"
          @click=${() => el._setDesiredLabel(UNTRIAGED)}>
    <help-icon-sk></help-icon-sk>
  </button>
  <button class="closest ${el.value === CLOSEST ? 'selected' : ''}"
          @click=${() => el._setDesiredLabel(CLOSEST)}>
    <view-agenda-icon-sk></view-agenda-icon-sk>
  </button>
</div>

<div>
  <checkbox-sk @change=${el._toggleAll} label="Triage all ${el._allDigestCount} digests"
     title='Choose whether to triage just the digests on this page or all that match the query'
     ?checked=${el._triageAll}></checkbox-sk>
</div>

<div class=controls>
    <button>Cancel (do nothing)</button>
    <button>Triage ${el._triageAll ? el._allDigestCount : el._pageDigestCount} digests as ${el._value}</button>
</div>
`;

function countDigests(map) {
  let count = 0;
  for (const byTestMap of map) {
    count += Object.keys(byTestMap);
  }
  return count;
}

define('bulk-triage-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._value = CLOSEST;
    this._triageAll = false;

    this._pageDigests = {};
    this._pageDigestCount = 50;
    this._allDigests = {};
    this._allDigestCount = 117;
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
    if (![POSITIVE, NEGATIVE, UNTRIAGED, CLOSEST].includes(newValue)) {
      throw new RangeError(`Invalid bulk-triage-sk value: "${newValue}".`);
    }
    this._value = newValue;
    this._render();
  }

  _setDesiredLabel(newValue) {
    this.value = newValue;
  }

  _toggleAll(e) {
    e.preventDefault();
    this._triageAll = !this._triageAll;
    this._render();
  }
});
