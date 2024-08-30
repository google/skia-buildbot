/**
 * @module module/skcq
 * @description <h2><code>skcq</code></h2>
 *
 */

import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import '../../../elements-sk/modules/error-toast-sk';
import '../../../elements-sk/modules/icons/folder-icon-sk';
import '../../../elements-sk/modules/icons/help-icon-sk';
import '../../../elements-sk/modules/icons/home-icon-sk';
import '../../../elements-sk/modules/spinner-sk';

import '../../../infra-sk/modules/app-sk';
import '../../../infra-sk/modules/theme-chooser-sk';

import '../processing-table-sk';

export class SkCQ extends ElementSk {
  constructor() {
    super(SkCQ.template);
  }

  private static template = () => html`
    <processing-table-sk id="cq-table"></processing-table-sk>
    <processing-table-sk id="dry-runs-table" dryrun></processing-table-sk>
  `;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
  }
}

define('skcq-sk', SkCQ);
