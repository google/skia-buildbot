/**
 * @module modules/dots-legend-sk
 * @description <h2><code>dots-legend-sk</code></h2>
 *
 * A legend for the dots-sk element.
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
  DOT_STROKE_COLORS,
  DOT_FILL_COLORS,
  MAX_UNIQUE_DIGESTS,
} from '../dots-sk/constants';
import { detailHref, diffPageHref } from '../common';

import 'elements-sk/icon/cancel-icon-sk';
import 'elements-sk/icon/check-circle-icon-sk';
import 'elements-sk/icon/help-icon-sk';

const template = (el) => html`
  ${el._digests
    .slice(0, MAX_UNIQUE_DIGESTS - 1)
    .map((digest, index) => digestTemplate(digest, index, el))}

  ${lastDigest(el)}
`;

const digestTemplate = (digest, index, el) => html`
  ${dotTemplate(index)}
  <a target=_blank class=digest href="${el._digestDetailHref(index)}">
    ${digest.digest}
  </a>
  ${statusIconTemplate(digest.status)}
  ${index > 0
    ? html`<a target=_blank class=diff href="${el._digestDiffHref(index)}">
               diff
             </a>`
    : html`<span></span>`}
`;

const lastDigest = (el) => {
  // If the API returns fewer digests than MAX_UNIQUE_DIGESTS, we should compare against
  // the reported totalDigests to determine if we need to display nothing (no more digests),
  // the last digest (if it exactly matches the maximum) or the message saying there were too
  // many digests to display them all.
  if (el.totalDigests < MAX_UNIQUE_DIGESTS) {
    return '';
  }
  if (el.totalDigests === MAX_UNIQUE_DIGESTS) {
    return digestTemplate(el.digests[MAX_UNIQUE_DIGESTS - 1], MAX_UNIQUE_DIGESTS - 1, el);
  }
  return oneOfManyOtherDigestsTemplate(el.totalDigests);
};

const oneOfManyOtherDigestsTemplate = (totalDigests) => html`
  ${dotTemplate(MAX_UNIQUE_DIGESTS - 1)}
  <span class=one-of-many-other-digests>
    One of ${totalDigests - (MAX_UNIQUE_DIGESTS - 1)} other digests
    (${totalDigests} in total).
  </span>
`;

const dotTemplate = (index) => {
  const style = `border-color: ${DOT_STROKE_COLORS[index]};`
      + `background-color: ${DOT_FILL_COLORS[index]};`;
  return html`<div class=dot style="${style}"></div>`;
};

const statusIconTemplate = (status) => {
  switch (status) {
    case 'negative':
      return html`<cancel-icon-sk class=negative-icon></cancel-icon-sk>`;
    case 'positive':
      return html`
        <check-circle-icon-sk class=positive-icon></check-circle-icon-sk>
      `;
    case 'untriaged':
      return html`<help-icon-sk class=untriaged-icon></help-icon-sk>`;
    default:
      throw `Unknown status: "${status}"`;
  }
};

define('dots-legend-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._digests = [];
    this._changeListID = '';
    this._crs = '';
    this._test = '';
    this._totalDigests = 0;
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  /**
   * @prop digests {Array} An array of {digest: 'a4f32...', status: 'positive'}
   *   objects.
   */
  get digests() { return this._digests; }

  set digests(digests) {
    this._digests = digests;
    this._render();
  }

  /**
   * @prop changeListID {string} The changelist id (or empty string if this is the master branch).
   */
  get changeListID() { return this._changeListID; }

  set changeListID(id) {
    this._changeListID = id;
    this._render();
  }

  /**
   * @prop crs {string} The Code Review System (e.g. "gerrit") if changeListID is set.
   */
  get crs() { return this._crs; }

  set crs(c) {
    this._crs = c;
    this._render();
  }

  /**
   * @prop test {string} Test name.
   */
  get test() { return this._test; }

  set test(test) {
    this._test = test;
    this._render();
  }

  /**
   * @prop totalDigests {Number} The total number of digests that were seen in this group of traces,
   *   which can be more than digests.length, due to the fact that the backend limits the length
   *   of digests when it sends it to us.
   */
  get totalDigests() { return this._totalDigests; }

  set totalDigests(td) {
    this._totalDigests = td;
    this._render();
  }

  _digestDetailHref(index) {
    return detailHref(this._test, this._digests[index].digest, this.changeListID, this.crs);
  }

  _digestDiffHref(index) {
    return diffPageHref(this._test, this._digests[0].digest, this._digests[index].digest,
      this.changeListID, this.crs);
  }
});
