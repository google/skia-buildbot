/**
 * @module modules/shaders-app-sk
 * @description <h2><code>shaders-app-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk/ElementSk';

export class ShadersAppSk extends ElementSk {
  constructor() {
    super(ShadersAppSk.template);
  }

  private static template = (ele: ShadersAppSk) => html`<h3>Hello world</h3>`;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }
}

define('shaders-app-sk', ShadersAppSk);
