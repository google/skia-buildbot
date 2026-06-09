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
import { LitElement, html } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import '../../../elements-sk/modules/icons/check-circle-icon-sk';
import '../../../elements-sk/modules/icons/cancel-icon-sk';
import '../../../elements-sk/modules/icons/help-icon-sk';

@customElement('tricon2-sk')
export class TriconSk extends LitElement {
  @property({ type: String, reflect: true })
  value: string = '';

  protected createRenderRoot() {
    return this;
  }

  render() {
    switch (this.value) {
      case 'positive':
        return html`<check-circle-icon-sk></check-circle-icon-sk>`;
      case 'negative':
        return html`<cancel-icon-sk></cancel-icon-sk>`;
      default:
        return html`<help-icon-sk></help-icon-sk>`;
    }
  }
}
