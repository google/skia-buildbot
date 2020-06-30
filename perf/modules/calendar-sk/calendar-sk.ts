/**
 * @module modules/calendar-sk
 * @description <h2><code>calendar-sk</code></h2>
 *
 * Displays an accessible calendar, one month at a time, and allows selecting a
 * single day. Offers the ability to navigate by both month and year. Also is
 * themeable and offers keyboard navigation.
 *
 * Why not use input type="date"? It doesn't work on Safari, and the pop-up
 * calendar isn't styleable.
 *
 * Why not use the Elix web component? It is not themeable (at least not
 * easily), and it is also inaccessible.
 *
 * Accessibility advice was derived from this page:
 *   https://www.w3.org/TR/wai-aria-practices/examples/dialog-modal/datepicker-dialog.html
 *
 * @evt change - A CustomEvent with the selected Date in the detail.
 *
 * This element provides a keyboardHandler callback that should be attached and
 * detached to/from the appropriate containing element when it is used, for
 * example, a containing 'dialog' element.
 */
import {html, TemplateResult} from 'lit-html';
import {ElementSk} from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/styles/buttons';
import 'elements-sk/icon/navigate-before-icon-sk';
import 'elements-sk/icon/navigate-next-icon-sk';

/*
 * Most of the Date[1] object's methods return zero-indexed values, with exceptions such as
 * Date.prototype.getDate() and Date.prototype.getYear().
 *
 * To make things easier to follow, we suffix zero-indexed values returned by a
 * Date object with "Index", e.g.:
 *
 *   const dayIndex = date.getDay();     // (0-6).
 *   const monthIndex = date.getMonth(); // (0-11).
 *
 * [1] https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Date
 */

const getNumberOfDaysInMonth = (year: number, monthIndex: number) => {
  // Jump forward one month, and back one day to get the last day of the month.
  // Since days are 1-indexed, a value of 0 represents the last day of the
  // previous month.
  return new Date(year, monthIndex + 1, 0).getDate();
};

// Returns a day of the week [0-6]
const firstDayIndexOfMonth = (year: number, monthIndex: number): number => {
  return new Date(year, monthIndex).getDay();
};

// Used in templates.
const sevenDaysInAWeek = [0, 1, 2, 3, 4, 5, 6];

// To display a month we need to display up to 6 weeks. Used in templates.
const sixWeeks = [0, 1, 2, 3, 4, 5];

// The dates that CalendarSk manipulates, always in local time.
class CalendarDate {
  year: number;
  monthIndex: number;
  date: number;

  constructor(d: Date) {
    this.year = d.getFullYear();
    this.monthIndex = d.getMonth();
    this.date = d.getDate();
  }

  equal(d: CalendarDate) {
    return (
      this.year === d.year &&
      this.monthIndex === d.monthIndex &&
      this.date === d.date
    );
  }
}

export class CalendarSk extends ElementSk {
  private static template = (ele: CalendarSk) => html`
    <table>
      <tr>
        <th>
          <button
            @click=${ele.decYear}
            aria-label="Previous year"
            title="Previous year"
            id="previous-year"
          >
            <navigate-before-icon-sk></navigate-before-icon-sk>
          </button>
        </th>
        <th colspan="5">
          <h2 aria-live="polite" id="calendar-year">
            ${new Intl.DateTimeFormat(ele._locale, {year: 'numeric'}).format(
              ele._displayDate
            )}
          </h2>
        </th>
        <th>
          <button
            @click=${ele.incYear}
            aria-label="Next year"
            title="Next year"
            id="next-year"
          >
            <navigate-next-icon-sk></navigate-next-icon-sk>
          </button>
        </th>
      </tr>
      <tr>
        <th>
          <button
            @click=${ele.decMonth}
            aria-label="Previous month"
            title="Previous month"
            id="previous-month"
          >
            <navigate-before-icon-sk></navigate-before-icon-sk>
          </button>
        </th>
        <th colspan="5">
          <h2 aria-live="polite" id="calendar-month">
            ${new Intl.DateTimeFormat(ele._locale, {month: 'long'}).format(
              ele._displayDate
            )}
          </h2>
        </th>
        <th>
          <button
            @click=${ele.incMonth}
            aria-label="Next month"
            title="Next month"
            id="next-month"
          >
            <navigate-next-icon-sk></navigate-next-icon-sk>
          </button>
        </th>
      </tr>
      ${ele._weekDayHeader}
      ${sixWeeks.map((i) => CalendarSk.rowTemplate(ele, i))}
    </table>
  `;

  private static buttonForDateTemplate = (
    ele: CalendarSk,
    date: number,
    daysInMonth: number,
    selected: boolean
  ) => {
    if (date < 1 || date > daysInMonth) {
      return html``;
    }
    return html`<button
      @click=${ele.dateClick}
      data-date=${date}
      tabindex=${selected ? 0 : -1}
      aria-selected=${selected}
    >
      ${date}
    </button>`;
  };

  private static rowTemplate = (ele: CalendarSk, weekIndex: number) => {
    const year = ele.displayYear();
    const monthIndex = ele.displayMonthIndex();
    const today = new CalendarDate(new Date());

    // If June starts on a Tuesday then IndexOfTheFirstDayOfTheMonth = 2,
    // which means if we are filling in the first row we want to leave the first
    // two days (Sunday and Monday) blank.
    const firstDayOfTheMonthIndex = firstDayIndexOfMonth(year, monthIndex);
    const daysInMonth = getNumberOfDaysInMonth(year, monthIndex);
    const selectedDate = ele._displayDate.getDate();
    const currentDate = new CalendarDate(ele._displayDate);
    return html`<tr>
      ${sevenDaysInAWeek.map((i) => {
        const date = 7 * weekIndex + i + 1 - firstDayOfTheMonthIndex;
        currentDate.date = date;
        const selected = selectedDate === date;
        return html`<td
          class="
            ${currentDate.equal(today) ? 'today' : ''}
            ${selected ? 'selected' : ''}
          "
        >
          ${CalendarSk.buttonForDateTemplate(ele, date, daysInMonth, selected)}
        </td>`;
      })}
    </tr>`;
  };

  private _displayDate: Date = new Date();
  private _weekDayHeader: TemplateResult = html``;
  private _locale: string | string[] | undefined = undefined;

  constructor() {
    super(CalendarSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this.buildWeekDayHeader();
    this._render();
  }

  /**
   * Attach this handler to the 'keydown' event on the appropriate elements
   * parent, such as document or a 'dialog' element.
   *
   * Allows finer grained control of keyboard events on a page with more
   * than one keyboard listener.
   */
  keyboardHandler(e: KeyboardEvent) {
    let keyHandled = true;
    switch (e.code) {
      case 'PageUp':
        this.decMonth();
        break;

      case 'PageDown':
        this.incMonth();
        break;

      case 'ArrowRight':
        this.incDay();
        break;

      case 'ArrowLeft':
        this.decDay();
        break;

      case 'ArrowUp':
        this.decWeek();
        break;

      case 'ArrowDown':
        this.incWeek();
        break;

      default:
        keyHandled = false;
        break;
    }
    if (keyHandled) {
      e.stopPropagation();
      e.preventDefault();
      this.querySelector<HTMLButtonElement>(
        'button[aria-selected="true"]'
      )!.focus();
    }
  }

  buildWeekDayHeader() {
    // March 1, 2020 falls on a Sunday, use that to generate the week day headers.
    const narrowFormatter = new Intl.DateTimeFormat(this._locale, {
      weekday: 'narrow',
    });
    const longFormatter = new Intl.DateTimeFormat(this._locale, {
      weekday: 'long',
    });
    this._weekDayHeader = html`<tr class="weekdayHeader">
      ${sevenDaysInAWeek.map(
        (i) =>
          html`<td>
            <span abbr="${longFormatter.format(new Date(2020, 2, i + 1))}">
              ${narrowFormatter.format(new Date(2020, 2, i + 1))}
            </span>
          </td>`
      )}
    </tr>`;
  }

  private dateClick(e: MouseEvent) {
    const d = new Date(this._displayDate);
    d.setDate(+(e.target as HTMLButtonElement).dataset.date!);
    this.dispatchEvent(
      new CustomEvent<Date>('change', {
        detail: d,
        bubbles: true,
      })
    );
    this._displayDate = d;
    this._render();
  }

  private displayYear(): number {
    return this._displayDate.getFullYear();
  }

  private displayMonthIndex(): number {
    return this._displayDate.getMonth();
  }

  private incYear() {
    const year = this.displayYear();
    const monthIndex = this.displayMonthIndex();
    let date = this._displayDate.getDate();
    const daysInMonth = getNumberOfDaysInMonth(year + 1, monthIndex);
    if (date > daysInMonth) {
      date = daysInMonth;
    }
    this._displayDate = new Date(year + 1, monthIndex, date);
    this._render();
  }

  private decYear() {
    const year = this.displayYear();
    const monthIndex = this.displayMonthIndex();
    let date = this._displayDate.getDate();
    const daysInMonth = getNumberOfDaysInMonth(year - 1, monthIndex);
    if (date > daysInMonth) {
      date = daysInMonth;
    }
    this._displayDate = new Date(year - 1, monthIndex, date);
    this._render();
  }

  private incMonth() {
    let year = this.displayYear();
    let monthIndex = this.displayMonthIndex();
    let date = this._displayDate.getDate();

    monthIndex += 1;
    if (monthIndex > 11) {
      monthIndex = 0;
      year += 1;
    }

    const daysInMonth = getNumberOfDaysInMonth(year, monthIndex);
    if (date > daysInMonth) {
      date = daysInMonth;
    }

    this._displayDate = new Date(year, monthIndex, date);
    this._render();
  }

  private decMonth() {
    let year = this.displayYear();
    let monthIndex = this.displayMonthIndex();
    let date = this._displayDate.getDate();

    monthIndex -= 1;
    if (monthIndex < 0) {
      monthIndex = 11;
      year -= 1;
    }

    const daysInMonth = getNumberOfDaysInMonth(year, monthIndex);
    if (date > daysInMonth) {
      date = daysInMonth;
    }

    this._displayDate = new Date(year, monthIndex, date);
    this._render();
  }

  private incDay() {
    const year = this.displayYear();
    const monthIndex = this.displayMonthIndex();
    const date = this._displayDate.getDate();
    this._displayDate = new Date(year, monthIndex, date + 1);
    this._render();
  }

  private decDay() {
    const year = this.displayYear();
    const monthIndex = this.displayMonthIndex();
    const date = this._displayDate.getDate();
    this._displayDate = new Date(year, monthIndex, date - 1);
    this._render();
  }

  private incWeek() {
    const year = this.displayYear();
    const monthIndex = this.displayMonthIndex();
    const date = this._displayDate.getDate();
    this._displayDate = new Date(year, monthIndex, date + 7);
    this._render();
  }

  private decWeek() {
    const year = this.displayYear();
    const monthIndex = this.displayMonthIndex();
    const date = this._displayDate.getDate();
    this._displayDate = new Date(year, monthIndex, date - 7);
    this._render();
  }

  /** The date to display on the calendar. */
  get displayDate() {
    return this._displayDate;
  }

  set displayDate(v) {
    this._displayDate = v;
    this._render();
  }

  /**
   * Leave as undefined to use the browser settings. Only really used for
   * testing.
   */
  public get locale() {
    return this._locale;
  }

  public set locale(v) {
    this._locale = v;
    this.buildWeekDayHeader();
    this._render();
  }
}

window.customElements.define('calendar-sk', CalendarSk);
