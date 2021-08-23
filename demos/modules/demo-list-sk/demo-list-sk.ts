/**
 * @fileoverview A custom element for the basic demos.skia.org header.
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';

import { errorMessage } from 'elements-sk/errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { Metadata } from '../rpc_types';

const demoTemplate = (demo: string) => html`
<tr>
  <td><a href="/demo/${demo}">${demo}</a></td>
</tr>
`;

export class DemoListSk extends ElementSk {
    private static template = (el: DemoListSk) => html`
<table class=demolist>
  <thead>
    <tr>
      <th>Available Demos (<a href="${el.repoURL}">${el.repoHash.substring(0, 10)}</a>)</th>
    </tr>
  </thead>
  <tbody>
    ${el.demos.map((demo: string) => demoTemplate(demo))}
  </tbody>
</table>
`;

    private demos: string[] = [];

    private repoURL: string = '';

    private repoHash: string = '';

    constructor() {
      super(DemoListSk.template);
    }

    connectedCallback(): void {
      super.connectedCallback();
      fetch('/demo/metadata.json', { method: 'GET' })
        .then(jsonOrThrow)
        .then((json: Metadata) => {
          this.demos = json.demos || [];
          this.repoURL = json.revision.url;
          this.repoHash = json.revision.hash;
          this._render();
          this.dispatchEvent(new CustomEvent('load-complete', { bubbles: true }));
        })
        .catch(errorMessage);
      this._render();
    }
}

define('demo-list-sk', DemoListSk);
