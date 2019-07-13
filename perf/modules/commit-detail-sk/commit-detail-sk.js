/**
 * @module modules/commit-detail-sk
 * @description <h2><code>commit-detail-sk</code></h2>
 *
 * An element to display information around a single commit.
 *
 * The element takes as data a serialized cid.CommitDetail.
 *
 */
import { html, render } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import { $$ } from 'common-sk/modules/dom.js';
import { upgradeProperty } from 'elements-sk/upgradeProperty';

const template = (ele) => html`
<div @click=${(e) => ele._click(e)} class=linkish><pre>${ele.cid.message}</pre></div>
<div class="tip hidden">
  <a href="/g/e/${ele.cid.hash}">Explore</a>
  <a href="/g/c/${ele.cid.hash}">Cluster</a>
  <a href="/g/t/${ele.cid.hash}">Triage</a>
  <a href="${ele.cid.url}">Commit</a>
</div>`;

window.customElements.define('commit-detail-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._cid = {};
  }

  connectedCallback() {
    super.connectedCallback();
    upgradeProperty(this, 'cid');
    this._render();
  }

  _click() {
    $$('.tip', this).classList.toggle('hidden');
  }

  /** @prop cid {Object} cid.CommitId. */
  get cid() { return this._cid }
  set cid(val) {
    this._cid = val;
    this._render();
  }

});
