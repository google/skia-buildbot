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
import { DigestStatus, Label, Params } from '../rpc_types';

import 'elements-sk/icon/cancel-icon-sk';
import 'elements-sk/icon/check-circle-icon-sk';
import 'elements-sk/icon/help-icon-sk';

export class DotsLegendSk extends ElementSk {
  private static template = (el: DotsLegendSk) => html`
    ${el._digests
    .slice(0, MAX_UNIQUE_DIGESTS - 1)
    .map((digest, index) => DotsLegendSk.digestTemplate(el, digest, index))}

    ${DotsLegendSk.lastDigest(el)}
  `;

  private static digestTemplate = (el: DotsLegendSk, digest: DigestStatus, index: number) => html`
    ${DotsLegendSk.dotTemplate(index)}
    <a target=_blank class=digest href="${el.digestDetailHref(index)}">${digest.digest}</a>
    ${DotsLegendSk.statusIconTemplate(digest.status)}
    ${index > 0
    ? html`<a target=_blank class=diff href="${el.digestDiffHref(index)}">diff</a>`
    : html`<span></span>`}
  `;

  private static lastDigest = (el: DotsLegendSk) => {
    // If the API returns fewer digests than MAX_UNIQUE_DIGESTS, we should compare against
    // the reported totalDigests to determine if we need to display nothing (no more digests),
    // the last digest (if it exactly matches the maximum) or the message saying there were too
    // many digests to display them all.
    if (el.totalDigests < MAX_UNIQUE_DIGESTS) {
      return '';
    }
    if (el.totalDigests === MAX_UNIQUE_DIGESTS) {
      return DotsLegendSk.digestTemplate(
        el, el.digests[MAX_UNIQUE_DIGESTS - 1], MAX_UNIQUE_DIGESTS - 1,
      );
    }
    return DotsLegendSk.oneOfManyOtherDigestsTemplate(el.totalDigests);
  };

  private static oneOfManyOtherDigestsTemplate = (totalDigests: number) => html`
    ${DotsLegendSk.dotTemplate(MAX_UNIQUE_DIGESTS - 1)}
    <span class=one-of-many-other-digests>
      One of ${totalDigests - (MAX_UNIQUE_DIGESTS - 1)} other digests
      (${totalDigests} in total).
    </span>
  `;

  private static dotTemplate = (index: number) => {
    const style = `border-color: ${DOT_STROKE_COLORS[index]};`
        + `background-color: ${DOT_FILL_COLORS[index]};`;
    return html`<div class=dot style="${style}"></div>`;
  };

  private static statusIconTemplate = (status: Label) => {
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

  private _grouping: Params = {};

  private _digests: DigestStatus[] = [];

  private _changeListID = '';

  private _crs = '';

  private _totalDigests = 0;

  constructor() {
    super(DotsLegendSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  /** Grouping. */
  get grouping(): Params { return this._grouping; }

  set grouping(grouping: Params) {
    this._grouping = grouping;
    this._render();
  }

  /** The digests to show. */
  get digests(): DigestStatus[] { return this._digests; }

  set digests(digests: DigestStatus[]) {
    this._digests = digests;
    this._render();
  }

  /** The changelist ID (or empty string if this is the master branch). */
  get changeListID(): string { return this._changeListID; }

  set changeListID(id: string) {
    this._changeListID = id;
    this._render();
  }

  /** The Code Review System (e.g. "gerrit") if changeListID is set. */
  get crs(): string { return this._crs; }

  set crs(c: string) {
    this._crs = c;
    this._render();
  }

  /**
   * The total number of digests that were seen in this group of traces, which can be more than
   * digests.length, due to the fact that the backend limits the length of digests when it sends it
   * to us.
   */
  get totalDigests(): number { return this._totalDigests; }

  set totalDigests(td: number) {
    this._totalDigests = td;
    this._render();
  }

  private digestDetailHref(index: number): string {
    return detailHref(this.grouping, this._digests[index].digest, this.changeListID, this.crs);
  }

  private digestDiffHref(index: number): string {
    return diffPageHref(
      this._grouping,
      this._digests[0].digest,
      this._digests[index].digest,
      this.changeListID,
      this.crs,
    );
  }
}

define('dots-legend-sk', DotsLegendSk);
