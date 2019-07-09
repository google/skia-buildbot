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
import { html, render } from 'lit-html'
import 'elements-sk/icon/check-circle-icon-sk';
import 'elements-sk/icon/cancel-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/styles/buttons';
import { upgradeProperty } from 'elements-sk/upgradeProperty';

const template = (ele) => {
  switch (ele.value) {
    case 'positive':
      return html`<check-circle-icon-sk title='Positive'></check-circle-icon-sk>`
    case 'negative':
      return html`<cancel-icon-sk title='Negative'></cancel-icon-sk>`
    default:
      return html`<help-icon-sk title='Untriaged'></help-icon-sk>`
  }
}

window.customElements.define('tricon2-sk', class extends HTMLElement {
  constructor() {
    super();
  }

  connectedCallback() {
    upgradeProperty(this, 'value');
    this._render();
  }

  static get observedAttributes() {
    return ['value'];
  }

  /** @prop value {string} Mirrors the 'value' attribute. */
  get value() { return this.getAttribute('value'); }
  set value(val) { this.setAttribute('value', val); }

  attributeChangedCallback(name, oldValue, newValue) {
    this._render();
  }

  _render() {
    render(template(this), this, {eventContext: this});
  }

});

