/**
 * @fileoverview A custom element for the basic demos.skia.org header.
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';

const template = (el) => html`
<div>
<h1>TODO(westont): demos.skia.org</h1>
</div>
`;

define('header-sk', class extends ElementSk {
  constructor() {
    super(template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }
});
