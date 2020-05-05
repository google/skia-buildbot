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
  ${demolist(el)}
  </tbody>
</table>
`;

function demolist(el) {
  const templates = [];
  for (const [name, metadata] of Object.entries(el._demos)) {
    templates.push(
      html`<tr>
        <td><a href="/demo/${name}">${name}</a></td>
        <td>${metadata.author}</td>
      </tr>`
    );
  }
  return templates;
}

define('demo-list-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._demos = {};
    fetch('/demo/metadata.json', { method: 'GET' })
      .then(jsonOrThrow)
      .then((json) => {
        this._demos = json;
        console.log('Got shit')
        console.log(json)
        this._render();
      })
     .catch(errorMessage);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }
});
