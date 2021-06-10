/**
 * @module module/skcq
 * @description <h2><code>skcq</code></h2>
 *
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import 'elements-sk/error-toast-sk';
import 'elements-sk/icon/folder-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/icon/home-icon-sk';
import 'elements-sk/spinner-sk';

import '../../../infra-sk/modules/app-sk';
import '../../../infra-sk/modules/login-sk';
import '../../../infra-sk/modules/theme-chooser-sk';

export class SkCQ extends ElementSk {
  private _main: HTMLElement | null = null;

  constructor() {
    super(SkCQ.template);
  }

  private static template = (el: SkCQ) => html`
  <div>TEST ING TESTING TESTING TESTING</div
`;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
  }
}

define('skcq', SkCQ);
