/**
 * @fileoverview A custom element for the basic demos.skia.org header.
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { SKIA_VERSION } from '../../build/version';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';

const template = (el) => html`
<style>
</style>
<header>
  <div class=title>Skia Demos</div>
  <div></div>
  <div class=version>
    <a href="https://skia.googlesource.com/skia/+/${SKIA_VERSION}">${SKIA_VERSION.substring(0, 10)}</a>
  </div>
</header>
<div>
<a href="/demo/skottiekit0">Demo2</a>
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
