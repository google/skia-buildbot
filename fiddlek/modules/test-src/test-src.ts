/**
 * @module module/test-src
 * @description <h2><code>test-src</code></h2>
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

export class TestSrc extends ElementSk {
  private static template = (ele: TestSrc) => html`<h3>Hello world</h3>`;

  constructor() {
    super(TestSrc.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }
};

define('test-src', TestSrc);