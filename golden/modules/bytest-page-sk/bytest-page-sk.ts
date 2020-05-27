/**
 * @module modules/bytest-page-sk
 * @description <h2><code>bytest-page-sk</code></h2>
 *
 * Displays the summary of tests that match the search query in the URL.
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

// TODO(lovisolo): Implement.

export class BytestPageSk extends ElementSk {
  private static _template = (el: BytestPageSk) => html`
    Hello, world!
  `;

  constructor() {
    super(BytestPageSk._template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }
}

define('bytest-page-sk', BytestPageSk);
