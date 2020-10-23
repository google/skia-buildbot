/**
 * @module modules/trybot-page-sk
 * @description <h2><code>trybot-page-sk</code></h2>
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

export class TrybotPageSk extends ElementSk {
  private static template = (ele: TrybotPageSk) =>
    html`<h3>Hello world</h3>`;

  constructor() {
    super(TrybotPageSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }
};

define('trybot-page-sk', TrybotPageSk);
