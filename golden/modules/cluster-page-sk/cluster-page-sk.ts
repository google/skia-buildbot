/**
 * @module modules/cluster-page-sk
 * @description <h2><code>cluster-page-sk</code></h2>
 *
 * Displays the cluster view for a single test.
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

// TODO(lovisolo): Implement.

export class ClusterPageSk extends ElementSk {
  private static _template = (el: ClusterPageSk) => html`
    Hello, world!
  `;

  constructor() {
    super(ClusterPageSk._template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }
}

define('cluster-page-sk', ClusterPageSk);
