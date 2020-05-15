/**
 * @fileoverview A custom element for the basic demos.skia.org header.
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';

import { errorMessage } from 'elements-sk/errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

const template = (el) => html`
<table class=demolist>
  <thead>
    <tr>
      <th>Available Demos (<a href="${el._repoURL}">${el._repoHash.substring(0, 10)}</a>)</th>
    </tr>
  </thead>
  <tbody>
    ${el._demos.map((demo) => demoTemplate(demo))}
  </tbody>
</table>
`;
const demoTemplate = (demo) => html`
<tr>
  <td><a href="/demo/${demo}">${demo}</a></td>
</tr>
`;

define('demo-list-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._demos = [];
    this._repoURL = '';
    this._repoHash = '';
  }

  connectedCallback() {
    super.connectedCallback();
    fetch('/demo/metadata.json', { method: 'GET' })
      .then(jsonOrThrow)
      .then((json) => {
        this._demos = json.demos;
        this._repoURL = json.revision.url;
        this._repoHash = json.revision.hash;
        this._render();
        this.dispatchEvent(new CustomEvent('load-complete', { bubbles: true }));
      })
      .catch(errorMessage);
    this._render();
  }
});
