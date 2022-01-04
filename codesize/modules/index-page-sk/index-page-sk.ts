/** Home page of codesize.skia.org. */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import '../codesize-scaffold-sk';

export class IndexPageSk extends ElementSk {
  private static template = (el: IndexPageSk) => html`
    <codesize-scaffold-sk>
      <p>Hello from the Index page!</p>
    </codesize-scaffold-sk>
  `;

  constructor() {
    super(IndexPageSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }
}

define('index-page-sk', IndexPageSk);
