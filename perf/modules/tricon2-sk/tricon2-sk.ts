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

export class TriconSk extends ElementSk {
  private static template = (ele: TriconSk) => {
    switch (ele.value) {
      case 'positive':
        return html`<check-circle-icon-sk
          title="Positive"
        ></check-circle-icon-sk>`;
      case 'negative':
        return html`<cancel-icon-sk title="Negative"></cancel-icon-sk>`;
      default:
        return html`<help-icon-sk title="Untriaged"></help-icon-sk>`;
    }
  };

  constructor() {
    super(TriconSk.template);
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
  get value() {
    return this.getAttribute('value') || '';
  }

  set value(val: string) {
    this.setAttribute('value', val);
  }

  attributeChangedCallback() {
    this._render();
  }
}

define('tricon2-sk', TriconSk);
