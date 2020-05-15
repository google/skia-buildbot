/**
 * @fileoverview A custom element for the basic demos.skia.org header.
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { SKIA_VERSION } from '../../build/version';
import '../../../infra-sk/modules/theme-chooser-sk';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/error-toast-sk';

const template = () => html`
<header>
  <div class=title>Skia Demos</div>
  <div>
    <theme-chooser-sk></theme-chooser-sk>
    <a href="https://skia.googlesource.com/skia/+/${SKIA_VERSION}">${SKIA_VERSION.substring(0, 10)}</a>
  </div>
</header>
<footer><error-toast-sk></error-toast-sk></footer>
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
