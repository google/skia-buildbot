/**
 * @module modules/calendar-input-sk
 * @description <h2><code>calendar-input-sk</code></h2>
 *
 * Displays a text input for the date along with a button that pops up a dialog
 * allowing the user to select the date from a calendar-sk.
 *
 * @evt input - A CustomEvent<Date> with the new date.
 *
 */
import { html, LitElement } from 'lit';
import { state, query, customElement } from 'lit/decorators.js';
import { live } from 'lit/directives/live.js';
import '../../../elements-sk/modules/icons/date-range-icon-sk';
import '../calendar-sk';
import { CalendarSk } from '../calendar-sk/calendar-sk';

@customElement('calendar-input-sk')
export class CalendarInputSk extends LitElement {
  private static nextUniqueId = 0;

  private readonly uniqueId = `${CalendarInputSk.nextUniqueId++}`;

  @query('dialog')
  private dialog!: HTMLDialogElement;

  @query('calendar-sk')
  private calendar!: CalendarSk;

  @query('input')
  private input!: HTMLInputElement;

  @state()
  private _displayDate: Date = new Date();

  private keyboardForwarder = (e: KeyboardEvent) => this.calendar.keyboardHandler(e);

  createRenderRoot() {
    return this;
  }

  protected render() {
    return html`
      <label for="date-${this.uniqueId}">
        <input
          id="date-${this.uniqueId}"
          name="date-${this.uniqueId}"
          @input=${this.inputChangeHandler}
          type="text"
          pattern="[0-9]{4}-[0-9]{1,2}-[0-9]{1,2}"
          title="Date in YYYY-MM-DD format."
          placeholder="yyyy-mm-dd"
          .value="${live(
            `${this._displayDate.getFullYear()}-${
              this._displayDate.getMonth() + 1
            }-${this._displayDate.getDate()}`
          )}" />
        <span class="invalid" aria-live="polite" title="Date is invalid."> &cross; </span>
      </label>
      <button
        id="cal-button-${this.uniqueId}"
        class="action"
        @click=${this.openHandler}
        title="Open calendar dialog to choose the date.">
        <date-range-icon-sk></date-range-icon-sk>
      </button>
      <dialog @cancel=${this.dialogCancelHandler}>
        <calendar-sk
          @change=${this.calendarChangeHandler}
          .displayDate=${this.displayDate}></calendar-sk>
        <button @click=${this.dialogCancelHandler}>Cancel</button>
      </dialog>
    `;
  }

  private inputChangeHandler(e: InputEvent) {
    // We don't change event from the input element to be confused with the
    // 'change' event that CalendarInputSk will produce.
    e.stopPropagation();
    e.preventDefault();

    const inputElement = e.target! as HTMLInputElement;
    if (inputElement.validity.patternMismatch) {
      return;
    }

    // Since we have checked validity above we can parse the values with
    // confidence here.
    const dateString = inputElement.value;
    const parts = dateString.split('-');
    try {
      this.displayDate = new Date(+parts[0], +parts[1] - 1, +parts[2]);
    } catch (_error) {
      return;
    }
    this.sendEvent();
  }

  private openHandler() {
    this.dialog.showModal();
    this.dialog.addEventListener('keydown', this.keyboardForwarder);
  }

  private sendEvent() {
    this.dispatchEvent(
      new CustomEvent<Date>('input', {
        detail: this.displayDate,
        bubbles: true,
      })
    );
  }

  private calendarChangeHandler(e: CustomEvent<Date>) {
    this.displayDate = e.detail;
    this.sendEvent();
    this.dialog.close();
    this.dialog.removeEventListener('keydown', this.keyboardForwarder);
    this.input.focus();
  }

  private dialogCancelHandler() {
    this.dialog.close();
    this.dialog.removeEventListener('keydown', this.keyboardForwarder);
  }

  /** The default date, if not set defaults to today. */
  get displayDate(): Date {
    return this._displayDate;
  }

  set displayDate(val: Date) {
    this._displayDate = val;
  }

  /** @prop locale - The locale, used just for testing. */
  get locale(): string | string[] | undefined {
    return this.calendar!.locale;
  }

  set locale(val: string | string[] | undefined) {
    this.calendar!.locale = val;
  }
}
