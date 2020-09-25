/**
 * @module modules/debugger-page-sk
 * @description <h2><code>debugger-page-sk</code></h2>
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

export class DebuggerPageSk extends ElementSk {
  private static template = (ele: DebuggerPageSk) =>
    html`<h3>Hello world</h3>`;

  constructor() {
    super(DebuggerPageSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }
};

define('debugger-page-sk', DebuggerPageSk);
