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

import '../processing-table-sk';

export class SkCQ extends ElementSk {
  constructor() {
    super(SkCQ.template);
  }

  private static template = () => html`
  <processing-table-sk id=cq-table></processing-table-sk>
  <processing-table-sk id=dry-runs-table dryrun></processing-table-sk>
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
