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
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

export class {{.ClassName}} extends ElementSk {
  constructor() {
    super({{.ClassName}}.template);
  }

  private static template = (ele: {{.ClassName}}) => html`<h3>Hello world</h3>`;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }
}

define('{{.ElementName}}', {{.ClassName}});
