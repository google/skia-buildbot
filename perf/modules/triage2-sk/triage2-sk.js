/**
 * @module modules/triage2-sk
 * @description <h2><code>triage2-sk</code></h2>
 *  A custom element that allows toggling between the three
 *  states of triage: positive, negative, and untriaged.
 *
 * @evt change - The value of e.detail is the new triage value,
 *    one of "positive", "negative", or "untriaged".
 *
 * @attr value - The state of triage, either "positive", "negative", or
 *    "untriaged".
 *
 * @example
 *   <triage2-sk value=positive></triage2-sk>
 */
import { define } from 'elements-sk/define'
import { html, render } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import 'elements-sk/icon/check-circle-icon-sk';
import 'elements-sk/icon/cancel-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/styles/buttons';

const _match = (a,b) => { return a === b };

const template = (ele) => html`
  <button class=positive @click=${(e) => ele.value = 'positive'} ?selected=${_match(ele.value, 'positive')}>
    <check-circle-icon-sk title='Positive'></check-circle-icon-sk>
  </button>
  <button class=negative @click=${(e) => ele.value = 'negative'} ?selected=${_match(ele.value, 'negative')}>
    <cancel-icon-sk title='Negative'></cancel-icon-sk>
  </button>
  <button class=untriaged @click=${(e) => ele.value = 'untriaged'} ?selected=${_match(ele.value, 'untriaged')}>
    <help-icon-sk title='Untriaged'></help-icon-sk>
  </button>
  `;

define('triage2-sk', class extends ElementSk {
  constructor() {
    super(template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._upgradeProperty('value');
    if (!this.value) {
      this.value = 'untriaged';
    }
    this._render();
  }

  static get observedAttributes() {
    return ['value'];
  }

  /** @prop value {string} A value of 'positive', 'negative', or 'untriaged'. */
  get value() { return this.getAttribute('value'); }
  set value(val) { this.setAttribute('value', val); }

  attributeChangedCallback(name, oldValue, newValue) {
    if (oldValue != newValue) {
      this._render();
      this.dispatchEvent(new CustomEvent('change', { detail: newValue, bubbles: true }));
    }
  }
});
