/**
 * @module modules/debug-view-sk
 * @description <h2><code>debug-view-sk</code></h2>
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

export class DebugViewSk extends ElementSk {
  private static template = (ele: DebugViewSk) =>
    html`<canvas id="img" class="fit" width=400 height=400></canvas>`;

  constructor() {
    super(DebugViewSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }
};

define('debug-view-sk', DebugViewSk);
