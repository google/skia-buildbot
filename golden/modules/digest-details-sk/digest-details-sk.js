/**
 * @module module/digest-details-sk
 * @description <h2><code>digest-details-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { $$ } from '../../../common-sk/modules/dom';
import { shorten, imgHref, diffImgHref } from '../common';

import 'elements-sk/icon/group-work-icon-sk';
import '../dots-sk';
import '../dots-legend-sk';
import '../triage-sk';
import '../triage-history-sk';
import '../image-compare-sk';
import '../../../perf/modules/paramset-sk';

const template = (ele) => html`
<div class="flex_vertical container">
<div class=flex_horizontal>
  <span class=test_name>Test: ${ele._grouping}</span>
  <span class=expand></span>
  <a href="cluster#TODO" target="_blank" rel="noopener">
    <group-work-icon-sk title="Cluster view of this digest and all others for this test.">
    </group-work-icon-sk>
  </a>
</div>

<div class=flex_vertical>
  <div class=flex_horizontal>
    <span class=test_name>Left: ${ele._digest}</span>
    <span class=expand></span>
    <span class=test_name>Right: ${ele._right && ele._right.digest}</span>
  </div>
  <div class=flex_horizontal>
    <div>${detailsAndTriage(ele)}</div>
    <div>${imageComparison(ele)}</div>
    <div>
      <!-- TODO(kjlubick) add a mouseover for what this button does and giving more explanation
          if it is disabled -->
      <button @click=${ele._toggleRightRef} ?disabled=${!ele._canToggle()}>Toggle Reference</button>
      
      <div ?hidden=${ele._rightRef !== 'neg'} class=warning>
        Closest image is negative!
      </div>
      
      <!-- TODO(kjlubick) Comments would go here -->
    </div>
  </div>
</div>

<div class=trace_info>
  <dots-sk .value=${ele._traces} @hover=${ele._hoverOverTrace}
        @mouseleave=${ele._clearTraceHighlights}></dots-sk>
  <dots-legend-sk .digests=${ele._traces.digests} .issue=${ele._issue} .test=${ele._grouping}>
      .totalDigests=${ele._traces.total_digests || 0}</dots-legend-sk>
</div>
${paramset(ele)}
</div>`;


const detailsAndTriage = (ele) => {
  if (!ele._right) {
    return '';
  }

  return html`
<div class="flex_vertical metrics">
  <div>Diff metric: ${ele._right.diffs.combined.toFixed(3)}</div>
  <div>Diff %: ${(ele._right.pixelDiffPercent * 100).toFixed(2)}</div>
  <div>Pixels: ${ele._right.numDiffPixels}</div>
  <div>Max RGBA: ${ele._right.maxRGBADiffs.join(',')}</div>
  <triage-sk @change=${ele._triageChangeHandler} .value=${ele._status}></triage-sk>
  <triage-history-sk .history=${ele._triageHistory}></triage-history-sk>
</div>`;
};

const imageComparison = (ele) => {
  const leftImageHref = imgHref(ele._digest);

  if (!ele._right) {
    return `<image-compare-sk .left=${leftImageHref}></image-compare-sk>`;
  }
  const diffImageHref = diffImgHref(ele._digest, ele._right.digest);
  const rightImageHref = imgHref(ele._right.digest);
  return html`
<image-compare-sk .left=${leftImageHref} .diff=${diffImageHref} .right=${rightImageHref}>
</image-compare-sk>`;
};

const paramset = (ele) => {
  const input = {
    titles: [shorten(ele._digest)],
    paramsets: [ele._params],
  };

  if (ele._right) {
    input.titles.push(shorten(ele._right.digest));
    input.paramsets.push(ele._right.paramset);
  }
  return html`<paramset-sk .paramsets=${input} .highlight=${ele._highlightedParams}>
</paramset-sk>`;
};

const validRefs = ['pos', 'neg'];

define('digest-details-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._grouping = '';
    this._digest = '';
    this._status = 'untriaged';
    this._triageHistory = [];
    this._params = {};
    this._traces = {};
    this._closestRef = '';
    this._refDiffs = {};
    this._issue = ''; // TODO(kjlubick) Should we get this from query params or is it passed in?
    this._triageHistory = [];

    // This tracks which ref we are showing on the right. It will default to the closest one, but
    // can be changed with the toggle.
    this._rightRef = '';
    this._right = null;

    this._highlightedParams = {};
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  /**
   * @prop details {object} an object with many parts. It has a setter for compatibility with
   *   Polymer implementation.
   */
  set details(obj) {
    this._grouping = obj.test || '';
    this._digest = obj.digest || '';
    this._traces = obj.traces || {};
    this._params = obj.paramset || {};
    this._refDiffs = obj.refDiffs || {};
    this._rightRef = obj.closestRef || '';
    this._status = obj.status || '';
    this._triageHistory = obj.triage_history || [];
    this._right = this._refDiffs[this._rightRef] || null;
    // TODO(kjlubick) Finish the rest.
    this._render();
  }

  _canToggle() {
    let totalRefs = 0;
    for (const ref of validRefs) {
      if (this._refDiffs[ref]) {
        totalRefs++;
      }
    }
    return totalRefs > 1;
  }

  _clearTraceHighlights() {
    this._highlightedParams = {};
    this._render();
  }

  _hoverOverTrace(e) {
    const id = e.detail;
    this._highlightedParams = {};
    const traces = this._traces.traces;

    // Find the matching trace in details.traces.
    for (let i = 0, len = traces.length; i < len; i++) {
      if (traces[i].label === id) {
        this._highlightedParams = traces[i].params;
        break;
      }
    }
    this._render();
  }

  _render() {
    super._render();
    // TODO(kjlubick,lovisolo) would it make sense to have dots-sk scroll itself when its data
    //   is updated?
    const traces = $$('dots-sk', this);
    if (traces) {
      // We have to wait until after the dots-sk is rendered to set this, otherwise the scrollWidth
      // won't be correct.
      traces.scroll(traces.scrollWidth, 0);
    }
  }

  _toggleRightRef() {
    if (!this._canToggle()) {
      return;
    }
    let idx = validRefs.indexOf(this._rightRef);
    let newRight = '';
    while (!this._refDiffs[newRight]) {
      idx = (idx + 1) % validRefs.length;
      newRight = validRefs[idx];
    }
    this._rightRef = newRight;
    this._right = this._refDiffs[newRight];
    this._render();
  }

  _triageChangeHandler(e) {
    console.log(e);
    // TODO(kjlubick). Don't forget to update triageHistory
  }
});
