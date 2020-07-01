/**
 * @module modules/calendar-input-sk
 * @description <h2><code>calendar-input-sk</code></h2>
 *
 * Displays a text input for the date along with a button that pops up a dialog
 * allowing to select the date from a calendar-sk.
 *
 * @evt input - A CustomEvent<Date> with the new date.
 *
 */
import {html} from 'lit-html';
import {ElementSk} from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/icon/date-range-icon-sk';
import dialogPolyfill from 'dialog-polyfill';
import '../calendar-sk';
import {CalendarSk} from '../calendar-sk/calendar-sk';

export class CalendarInputSk extends ElementSk {
  private static template = (ele: CalendarInputSk) => html`
    <label>
      <input
        @change=${ele.inputChange}
        type="text"
        pattern="[0-9]{4}-[0-9]{1,2}-[0-9]{1,2}"
        title="Date in YYYY-MM-DD format."
        placeholder="yyyy-mm-dd"
        .value="${ele._displayDate.getFullYear()}-${ele._displayDate.getMonth() +
        1}-${ele._displayDate.getDate()}"
      />
      <span class="invalid" aria-live="polite" title="Date is invalid.">
        &cross;
      </span>
    </label>
    <button @click=${ele.open} title="Open calendar dialog to choose the date.">
      <date-range-icon-sk></date-range-icon-sk>
    </button>
    <dialog @cancel=${ele.cancel}>
      <calendar-sk
        @change=${ele.dateChanged}
        .displayDate=${ele.displayDate}
      ></calendar-sk>
      <button @click=${ele.cancel}>Cancel</button>
    </dialog>
  `;

  private dialog: HTMLDialogElement | null = null;
  private calendar: CalendarSk | null = null;
  private _displayDate: Date = new Date();
  private resolve: ((value?: any) => void) | null = null;
  private reject: ((reason?: any) => void) | null = null;

  constructor() {
    super(CalendarInputSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this.dialog = this.querySelector('dialog')!;
    this.calendar = this.querySelector<CalendarSk>('calendar-sk');
    dialogPolyfill.registerDialog(this.dialog);
  }

  inputChange(e: InputEvent) {
    e.stopPropagation();
    e.preventDefault();

    const inputElement = e.target! as HTMLInputElement;
    if (inputElement.validity.patternMismatch) {
      return;
    }
    const dateString = inputElement.value;
    const parts = dateString.split('-');
    try {
      this.displayDate = new Date(+parts[0], +parts[1] - 1, +parts[2]);
    } catch (error) {
      return;
    }
    this.dispatchEvent(
      new CustomEvent<Date>('input', {detail: this.displayDate, bubbles: true})
    );
  }

  async open() {
    const keyboardHandler = (e: KeyboardEvent) =>
      this.calendar!.keyboardHandler(e);
    try {
      this.dialog!.showModal();
      this.dialog!.addEventListener('keydown', keyboardHandler);
      const date = await new Promise<Date>((resolve, reject) => {
        this.resolve = resolve;
        this.reject = reject;
      });
      this.dispatchEvent(
        new CustomEvent<Date>('input', {detail: date, bubbles: true})
      );
      this.displayDate = date;
      this.querySelector<HTMLInputElement>('input')!.focus();
    } catch (_) {
      // Cancel button was pressed.
    } finally {
      this.dialog!.removeEventListener('keydown', keyboardHandler);
    }
  }

  private dateChanged(e: CustomEvent<Date>) {
    this.dialog!.close();
    this.resolve!(e.detail);
  }

  private cancel() {
    this.dialog!.close();
    this.reject!();
  }

  /** The default date, if not set defaults to today. */
  get displayDate() {
    return this._displayDate;
  }
  set displayDate(val) {
    this._displayDate = val;
    this._render();
  }

  /** @prop locale {string} The locate, used almost exclusively for testing. */
  get locale() {
    return this.calendar!.locale;
  }

  set locale(val) {
    this.calendar!.locale = val;
  }
}

window.customElements.define('calendar-input-sk', CalendarInputSk);
