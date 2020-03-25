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

import 'elements-sk/icon/group-work-icon-sk';
import '../dots-sk';
import '../dots-legend-sk';
import '../../../perf/modules/paramset-sk';

const template = (ele) => html`
<div class=flex_vertical>
<div class=flex_horizontal>
  <span class=test_name>Test: ${ele._test}</span>
  <span class=expand></span>
  <a href="cluster#TODO" target="_blank" rel="noopener">
    <group-work-icon-sk title="Cluster view of this digest and all others for this test.">
    </group-work-icon-sk>
  </a>
</div>

<div class=flex_vertical>
    <div>left and right</div>
    <div class=flex_horizontal>
        <div>details box and triage</div>
        <div>images and zoom</div>
        <div>toggle and comments</div>
    </div>
</div>

<div class=trace_info>
  <dots-sk .value=${ele._traces} @hover=${ele._hoverOverTrace}
        @mouseleave=${ele._clearTraceHighlights}></dots-sk>
  <dots-legend-sk .digests=${ele._traces.digests} .issue=${ele._issue} .test=${ele.test}>
      .totalDigests=${ele._traces.total_digests || 0}</dots-legend-sk>
</div>
${paramset(ele)}
</div>`;

const paramset = (ele) => {
  const input = {
    titles: [shorten(ele._digest)],
    paramsets: [ele._params],
  };

  if (ele._rightRef) {
    const right = ele._refDiffs[ele._rightRef];
    input.titles.push(shorten(right.digest));
    input.paramsets.push(right.paramset);
  }
  return html`<paramset-sk .paramsets=${input} .highlight=${ele._highlightedParams}>
</paramset-sk>`;
};

function shorten(str, maxLength = 15) {
  if (str.length <= maxLength) {
    return str;
  }
  return `${str.substr(0, maxLength - 3)}...`;
}

define('digest-details-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._test = '';
    this._digest = '';
    this._status = 'untriaged';
    this._triageHistory = [];
    this._params = {};
    this._traces = {};
    this._closestRef = '';
    this._refDiffs = {};
    this._issue = ''; // TODO(kjlubick) Should we get this from query params or is it passed in?

    // This tracks which ref we are showing on the right. It will default to the closest one, but
    // can be changed with the toggle.
    this._rightRef = '';

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
    this._test = obj.test || '';
    this._digest = obj.digest || '';
    this._traces = obj.traces || {};
    this._params = obj.paramset || {};
    this._refDiffs = obj.refDiffs || {};
    this._closestRef = obj.closestRef || '';
    if (this._closestRef) {
      this._rightRef = this._closestRef;
    } else {
      this._rightRef = '';
    }
    // TODO(kjlubick) Finish the rest.
    this._render();
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
});
