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
    <tr><th>Demo</th><th>Author</th></tr>
  </thead>
  <tbody>
    ${el._demos.map((demo) => demoTemplate(demo))}
  </tbody>
</table>
`;
const demoTemplate = (demo) => html`
<tr>
  <td><a href="/demo/${demo.name}">${demo.name}</a></td>
  <td>${demo.commit.author}</td>
</tr>
`;

define('demo-list-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._demos = [];
  }

  connectedCallback() {
    super.connectedCallback();
    fetch('/demo/metadata.json', { method: 'GET' })
      .then(jsonOrThrow)
      .then((json) => {
        this._demos = json;
        this._render();
        this.dispatchEvent(new CustomEvent('load-complete', { bubbles: true }));
      })
      .catch(errorMessage);
    this._render();
  }
});
