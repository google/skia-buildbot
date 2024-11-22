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
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../../../elements-sk/modules/icons/check-circle-icon-sk';
import '../../../elements-sk/modules/icons/cancel-icon-sk';
import '../../../elements-sk/modules/icons/help-icon-sk';

export class TriconSk extends ElementSk {
  constructor() {
    super(TriconSk.template);
  }

  private static template = (ele: TriconSk) => {
    switch (ele.value) {
      case 'positive':
        return html` <check-circle-icon-sk></check-circle-icon-sk> `;
      case 'negative':
        return html` <cancel-icon-sk></cancel-icon-sk> `;
      default:
        return html` <help-icon-sk></help-icon-sk> `;
    }
  };

  connectedCallback(): void {
    super.connectedCallback();
    this._upgradeProperty('value');
    this._render();
  }

  static get observedAttributes(): string[] {
    return ['value'];
  }

  /** @prop value {string} Mirrors the 'value' attribute. */
  get value(): string {
    return this.getAttribute('value') || '';
  }

  set value(val: string) {
    this.setAttribute('value', val);
  }

  attributeChangedCallback(): void {
    this._render();
  }
}

define('tricon2-sk', TriconSk);
