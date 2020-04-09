/**
 * @module module/{{.ElementName}}
 * @description <h2><code>{{.ElementName}}</code></h2>
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

const template = (ele) => html`
<h3>Hello world</h3>
`;

define('{{.ElementName}}', class extends ElementSk {
  constructor() {
    super(template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }
});
