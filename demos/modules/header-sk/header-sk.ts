/**
 * @fileoverview A custom element for the basic demos.skia.org header.
 */

import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import '../../../infra-sk/modules/theme-chooser-sk';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../../../elements-sk/modules/error-toast-sk';

const template = () => html`
  <header>
    <div class="title">Skia Demos</div>
    <div>
      <theme-chooser-sk></theme-chooser-sk>
    </div>
  </header>
  <footer><error-toast-sk></error-toast-sk></footer>
`;

define(
  'header-sk',
  class extends ElementSk {
    constructor() {
      super(template);
    }

    connectedCallback() {
      super.connectedCallback();
      this._render();
    }
  }
);
