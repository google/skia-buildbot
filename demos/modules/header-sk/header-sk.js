/**
 * @fileoverview A custom element for the basic demos.skia.org header.
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { SKIA_VERSION } from '../../build/version';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';

const template = (el) => html`
<header>
  <div class=title>Skia Demos</div>
  <div class=version>
    <a href="https://skia.googlesource.com/skia/+/${SKIA_VERSION}">${SKIA_VERSION.substring(0, 10)}</a>
  </div>
</header>
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
