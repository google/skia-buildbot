/**
 * @module module/cluster-digests-sk
 * @description <h2><code>cluster-digests-sk</code></h2>
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

define('cluster-digests-sk', class extends ElementSk {
  constructor() {
    super(template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }
});
