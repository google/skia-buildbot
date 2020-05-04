/**
 * @fileoverview A custom element for the basic demos.skia.org header.
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';

const template = (el) => html`
<style>

</style>
<div>
<a href="/demo/skottiekit0">Demo2</a>
<a href="/demo/skottiekit0">Demo3</a>
${el._demos.map((demoname) => demolinkTemplate(demoname))}
</div>
`;
const demolinkTemplate = (demoname) => html`
<a href="/demo/${demoname}">${demoname}</a>
`;

define('demo-list-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._demos = [];
    fetch('/demolist', { method: 'GET' })
      .then(jsonOrThrow)
      .then((json) => {
        this._demos = json.Demos;
        this._render();
      });
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }
});
