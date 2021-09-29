/**
 * @module modules/example-control-sk
 * @description <h2><code>example-control-sk</code></h2>
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

export class ExampleControlSk extends ElementSk {
  constructor() {
    super(ExampleControlSk.template);
  }

  private static template = (ele: ExampleControlSk) => html`<h3>Hello world</h3>`;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }
}

define('example-control-sk', ExampleControlSk);
