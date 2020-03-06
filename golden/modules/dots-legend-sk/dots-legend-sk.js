/**
 * @module modules/dots-legend-sk
 * @description <h2><code>dots-legend-sk</code></h2>
 *
 * A legend for the dots-sk element.
 */

import { define } from 'elements-sk/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { html } from 'lit-html';
import {
  DOT_STROKE_COLORS,
  DOT_FILL_COLORS,
  MAX_UNIQUE_DIGESTS,
} from '../dots-sk/constants';
import 'elements-sk/icon/cancel-icon-sk';
import 'elements-sk/icon/check-circle-icon-sk';
import 'elements-sk/icon/help-icon-sk';

const template = (el) => html`
  ${el._digests
      .slice(0, MAX_UNIQUE_DIGESTS)
      .map((digest, index) => digestTemplate(digest, index, el))}

  ${el._digests.length > MAX_UNIQUE_DIGESTS
      ? oneOfManyOtherDigestsTemplate(el.totalDigests)
      : ''}
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

const oneOfManyOtherDigestsTemplate = (totalDigests) => html`
  ${dotTemplate(MAX_UNIQUE_DIGESTS)}
  <span class=one-of-many-other-digests>
    One of ${totalDigests - MAX_UNIQUE_DIGESTS} other digests (${totalDigests} in total).
  </span>
`;

const dotTemplate = (index) => {
  const style =
      `border-color: ${DOT_STROKE_COLORS[index]};` +
      `background-color: ${DOT_FILL_COLORS[index]};`;
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
    this._issue = '';
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
   * @prop issue {string} An issue number/ID.
   */
  get issue() { return this._issue; }
  set issue(issue) {
    this._issue = issue;
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
    return `/detail`
        + `?test=${encodeURIComponent(this._test)}`
        + `&digest=${this._digests[index].digest}`
        + (this._issue ? `&issue=${this._issue}` : '');
  }

  _digestDiffHref(index) {
    return `/diff`
        + `?test=${encodeURIComponent(this._test)}`
        + `&left=${this._digests[0].digest}`
        + `&right=${this._digests[index].digest}`
        + (this._issue ? `&issue=${this._issue}` : '');
  }
});
