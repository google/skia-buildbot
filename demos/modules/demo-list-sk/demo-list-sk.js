/**
 * @fileoverview A custom element for the basic demos.skia.org header.
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';

const temp = {
  skottiekit0: {
    hash: "1e8ec1badefd48a018db3f9180f32fb76088f15b",
    author: "Weston Tracey (westont@google.com)",
    subject: "[demos] Add skottiekit demo to test skia-demos.corp.goog.",
    parent: [
      "a7c50b24ccaa50c4d4c62be3c77ab6ebf1f878f1"
    ],
    body: "Change-Id: I5a132a5c83bf5a30f419d52e558b4f18e266f044\nReviewed-on: https://skia-review.googlesource.com/c/infra-internal/+/285786\nAuto-Submit: Weston Tracey \u003cwestont@google.com\u003e\nReviewed-by: Kevin Lubick \u003ckjlubick@google.com\u003e\nCommit-Queue: Kevin Lubick \u003ckjlubick@google.com\u003e\n",
    timestamp: "2020-04-28T16:13:24Z"
  }
};
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
    this._demos = temp;
    fetch('/demo/metadata.json', { method: 'GET' })
      .then(jsonOrThrow)
      .then((json) => {
        this._demos = json;
        this._render();
      });
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }
});
