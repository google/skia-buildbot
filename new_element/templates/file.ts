/**
 * @module modules/{{.ElementName}}
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

export class {{.ClassName}} extends ElementSk {
  private static template = (ele: {{.ClassName}}) =>
    html`<h3>Hello world</h3>`;

  constructor() {
    super({{.ClassName}}.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }
};

define('{{.ElementName}}', {{.ClassName}});
