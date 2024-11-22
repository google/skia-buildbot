/**
 * @module modules/report-page-sk
 * @description <h2><code>report-page-sk</code></h2>
 *
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

export class ReportPageSk extends ElementSk {
  constructor() {
    super(ReportPageSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  private static template = () => html``;
}

define('report-page-sk', ReportPageSk);
