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
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/icon/date-range-icon-sk';
import '../calendar-sk';
import { CalendarSk } from '../calendar-sk/calendar-sk';

export class CalendarInputSk extends ElementSk {
  private dialog: HTMLDialogElement | null = null;

  private calendar: CalendarSk | null = null;

  private input: HTMLInputElement | null = null;

  private _displayDate: Date = new Date();

  // These two functions store the callbacks from a Promise, which allows the
  // openHandler() function to be a nice linear function.
  private resolve: ((value?: any)=> void) | null = null;

  private reject: ((reason?: any)=> void) | null = null;

  constructor() {
    super(CalendarInputSk.template);
  }

  private static template = (ele: CalendarInputSk) => html`
    <label>
      <input
        @input=${ele.inputChangeHandler}
        type="text"
        pattern="[0-9]{4}-[0-9]{1,2}-[0-9]{1,2}"
        title="Date in YYYY-MM-DD format."
        placeholder="yyyy-mm-dd"
        .value="${ele._displayDate.getFullYear()}-${ele._displayDate.getMonth()
        + 1}-${ele._displayDate.getDate()}"
      />
      <span class="invalid" aria-live="polite" title="Date is invalid.">
        &cross;
      </span>
    </label>
    <button
      class="calendar"
      @click=${ele.openHandler}
      title="Open calendar dialog to choose the date."
    >
      <date-range-icon-sk></date-range-icon-sk>
    </button>
    <dialog @cancel=${ele.dialogCancelHandler}>
      <calendar-sk
        @change=${ele.calendarChangeHandler}
        .displayDate=${ele.displayDate}
      ></calendar-sk>
      <button @click=${ele.dialogCancelHandler}>Cancel</button>
    </dialog>
  `;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();

    this.dialog = this.querySelector('dialog')!;
    this.calendar = this.querySelector<CalendarSk>('calendar-sk');
    this.input = this.querySelector('input');
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
    } catch (error) {
      return;
    }
    this.sendEvent();
  }

  private async openHandler() {
    const keyboardHandler = (e: KeyboardEvent) => this.calendar!.keyboardHandler(e);
    try {
      this.dialog!.showModal();
      this.dialog!.addEventListener('keydown', keyboardHandler);
      const date = await new Promise<Date>((resolve, reject) => {
        this.resolve = resolve;
        this.reject = reject;
      });
      this.displayDate = date;
      this.sendEvent();
      this.input!.focus();
    } catch (_) {
      // Cancel button was pressed.
    } finally {
      this.dialog!.removeEventListener('keydown', keyboardHandler);
    }
  }

  private sendEvent() {
    this.dispatchEvent(
      new CustomEvent<Date>('input', {
        detail: this.displayDate,
        bubbles: true,
      }),
    );
  }

  private calendarChangeHandler(e: CustomEvent<Date>) {
    this.dialog!.close();
    this.resolve!(e.detail);
  }

  private dialogCancelHandler() {
    this.dialog!.close();
    this.reject!();
  }

  /** The default date, if not set defaults to today. */
  get displayDate(): Date {
    return this._displayDate;
  }

  set displayDate(val: Date) {
    this._displayDate = val;
    this._render();
  }

  /** @prop locale - The locale, used just for testing. */
  get locale(): string | string[] | undefined {
    return this.calendar!.locale;
  }

  set locale(val: string | string[] | undefined) {
    this.calendar!.locale = val;
  }
}

define('calendar-input-sk', CalendarInputSk);
