/**
 * @module modules/scrapexchange-sk
 * @description <h2><code>scrapexchange-sk</code></h2>
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

export class ScrapexchangeSk extends ElementSk {
  private static template = (ele: ScrapexchangeSk) =>
    html`<h3>Hello world</h3>`;

  constructor() {
    super(ScrapexchangeSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }
};

define('scrapexchange-sk', ScrapexchangeSk);
