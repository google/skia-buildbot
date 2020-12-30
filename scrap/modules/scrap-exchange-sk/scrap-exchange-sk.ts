/**
 * @module modules/scrap-exchange-sk
 * @description <h2><code>scrap-exchange-sk</code></h2>
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import '../../../infra-sk/modules/theme-chooser-sk';
import 'elements-sk/error-toast-sk';


export class ScrapExchangeSk extends ElementSk {
  constructor() {
    super(ScrapExchangeSk.template);
  }

  private static template = (ele: ScrapExchangeSk) => html`
  <header>
    <h2>
      Scrap Exchange
    </h2>
    <theme-chooser-sk
      title="Toggle between light and dark mode."
    ></theme-chooser-sk>
  </header>
  <main>
  </main>
  <error-toast-sk></error-toast-sk>`;


  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }
}

define('scrap-exchange-sk', ScrapExchangeSk);
