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

import 'elements-sk/icon/group-work-icon-sk';

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

<div>traces box</div>
<div>traces legend</div>
<div>Params</div>
</div>`;

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
    // TODO(kjlubick)
    this._render();
  }
});
