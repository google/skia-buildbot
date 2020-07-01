/**
 * @module modules/calendar-input-sk
 * @description <h2><code>calendar-input-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import {html} from 'lit-html';
import {ElementSk} from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/icon/date-range-icon-sk';
import dialogPolyfill from 'dialog-polyfill';
import '../calendar-sk';

export class CalendarInputSk extends ElementSk {
  private static template = (ele: CalendarInputSk) => html`<input
      type="text"
      pattern="[0-9]{4}-[0-9]{2}-[0-9]{2}"
      placeholder="yyyy-mm-dd"
    />
    <date-range-icon-sk></date-range-icon-sk>
    <dialog>
      <calendar-sk></calendar-sk>
    </dialog> `;

  constructor() {
    super(CalendarInputSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    dialogPolyfill.registerDialog(this.querySelector('dialog')!);
  }
}

window.customElements.define('calendar-input-sk', CalendarInputSk);
