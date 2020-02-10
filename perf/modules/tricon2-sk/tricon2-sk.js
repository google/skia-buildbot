/**
 * @module modules/tricon2-sk
 * @description <h2><code>tricon2-sk</code></h2>
 *
 * The triage state icons.
 *
 * @attr {string} value - A string representing the triage status, one of
 *     "untriaged", "positive", or "negative".
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/icon/check-circle-icon-sk';
import 'elements-sk/icon/cancel-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/styles/buttons';

const template = (ele) => {
  switch (ele.value) {
    case 'positive':
      return html`<check-circle-icon-sk title='Positive'></check-circle-icon-sk>`;
    case 'negative':
      return html`<cancel-icon-sk title='Negative'></cancel-icon-sk>`;
    default:
      return html`<help-icon-sk title='Untriaged'></help-icon-sk>`;
  }
};

define('tricon2-sk', class extends ElementSk {
  constructor() {
    super(template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._upgradeProperty('value');
    this._render();
  }

  static get observedAttributes() {
    return ['value'];
  }

  /** @prop value {string} Mirrors the 'value' attribute. */
  get value() { return this.getAttribute('value'); }

  set value(val) { this.setAttribute('value', val); }

  attributeChangedCallback() {
    this._render();
  }
});
