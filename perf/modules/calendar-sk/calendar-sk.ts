/**
 * @module modules/calendar-sk
 * @description <h2><code>calendar-sk</code></h2>
 *
 * Displays a calendar, one month at a time, and allows selecting a single day.
 * Offers the ability to navigate by both month and year.
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { html, TemplateResult } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/styles/buttons';
import 'elements-sk/icon/navigate-before-icon-sk';
import 'elements-sk/icon/navigate-next-icon-sk';

/*
  The built in Date object for JS is inconsistent, years and days of the month
  are normal, but months, for some reason, are 0-indexed. So to remind us of
  that insanity in all the code below years are named 'year', days of the month
  are named 'date' and months are named 'monthIndex'.
*/

const getDaysInMonth = (year: number, monthIndex: number) => {
  // Jump forward one month, and back one day to get the last day of the month.
  // Since days are 1-indexed, a value of 0 represents the last day of the
  // previous month.
  return new Date(year, monthIndex + 1, 0).getDate();
};

// If June starts on a Tuesday then IndexOfTheFirstDayOfTheMonth = 2,
// which means if we are filling in the first row we want to leave the first
// two days (Sunday and Monday) blank.
const firstDayOfMonth = (year: number, monthIndex: number): number => {
  return new Date(year, monthIndex).getDay();
};

const sevenDaysInAWeek = new Array(7).fill(1);

// To display a month we need to display up to 6 weeks.
const sixWeeks = new Array(6).fill(1);

// The dates that CalendarSk manipulates, always in local time.
interface CalendarDate {
  year: number;
  monthIndex: number;
  date: number;
}

const calendarDateFromDate = (d: Date): CalendarDate => {
  return {
    year: d.getFullYear(),
    monthIndex: d.getMonth(),
    date: d.getDate(),
  };
};

const equal = (a: CalendarDate, b: CalendarDate): boolean =>
  a.year === b.year && a.monthIndex === b.monthIndex && a.date === b.date;

export class CalendarSk extends ElementSk {
  private static row = (ele: CalendarSk, weekIndex: number) => {
    const year = ele.displayYear();
    const monthIndex = ele.displayMonthIndex();
    const today = calendarDateFromDate(new Date());
    const selected = calendarDateFromDate(ele._date);

    // If June starts on a Tuesday then IndexOfTheFirstDayOfTheMonth = 2,
    // which means if we are filling in the first row we want to leave the first
    // two days (Sunday and Monday) blank.
    const IndexOfTheFirstDayOfTheMonth = firstDayOfMonth(year, monthIndex);
    const daysInMonth = getDaysInMonth(year, monthIndex);
    const currentDate = {
      year,
      monthIndex,
      date: -1,
    };
    return html`<tr>
      ${sevenDaysInAWeek.map((_, i) => {
        const d = 7 * weekIndex + i - IndexOfTheFirstDayOfTheMonth + 1;
        currentDate.date = d;
        return html`<td
          class="
            ${equal(currentDate, today) ? 'today' : ''}
            ${equal(currentDate, selected) ? 'selected' : ''}
          "
        >
          ${d >= 1 && d <= daysInMonth ? d : ''}
        </td>`;
      })}</tr
    >`;
  };

  private static template = (ele: CalendarSk) => html`
    <table>
      <tr>
        <th>
          <button @click=${ele.decYear}>
            <navigate-before-icon-sk></navigate-before-icon-sk>
          </button>
        </th>
        <th colspan="5">
          ${new Intl.DateTimeFormat(undefined, { year: 'numeric' }).format(
            ele._displayDate
          )}
        </th>
        <th>
          <button @click=${ele.incYear}>
            <navigate-next-icon-sk></navigate-next-icon-sk>
          </button>
        </th>
      </tr>
      <tr>
        <th>
          <button @click=${ele.decMonth}>
            <navigate-before-icon-sk></navigate-before-icon-sk>
          </button>
        </th>
        <th colspan="5">
          ${new Intl.DateTimeFormat(undefined, { month: 'long' }).format(
            ele._displayDate
          )}
        </th>
        <th>
          <button @click=${ele.incMonth}>
            <navigate-next-icon-sk></navigate-next-icon-sk>
          </button>
        </th>
      </tr>
      ${ele._weekDayHeader} ${sixWeeks.map((_, i) => CalendarSk.row(ele, i))}
    </table>
  `;

  private _displayDate: Date = new Date();
  private _date: Date = new Date();
  private _weekDayHeader: TemplateResult;

  constructor() {
    super(CalendarSk.template);

    // March 1, 2020 falls on a Sunday, use that to generate the week day headers.
    const formatter = new Intl.DateTimeFormat(undefined, {
      weekday: 'narrow',
    });
    this._weekDayHeader = html`<tr class="weekdayHeader">
      ${sevenDaysInAWeek.map(
        (_, i) => html`<td>${formatter.format(new Date(2020, 2, i + 1))}</td>`
      )}
    </tr>`;
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  private displayYear(): number {
    return this._displayDate.getFullYear();
  }

  private displayMonthIndex(): number {
    return this._displayDate.getMonth();
  }

  private incYear() {
    this._displayDate = new Date(
      this.displayYear() + 1,
      this.displayMonthIndex()
    );
    this._render();
  }

  private decYear() {
    this._displayDate = new Date(
      this.displayYear() - 1,
      this.displayMonthIndex()
    );
    this._render();
  }

  private incMonth() {
    let year = this.displayYear();
    let monthIndex = this.displayMonthIndex();
    monthIndex += 1;
    if (monthIndex > 11) {
      monthIndex = 0;
      year += 1;
    }
    this._displayDate = new Date(year, monthIndex);
    this._render();
  }

  private decMonth() {
    let year = this.displayYear();
    let monthIndex = this.displayMonthIndex();
    monthIndex -= 1;
    if (monthIndex < 0) {
      monthIndex = 11;
      year -= 1;
    }
    this._displayDate = new Date(year, monthIndex);
    this._render();
  }

  /** The selected date. */
  get date() {
    return this._date;
  }

  set date(val) {
    this._date = this._displayDate = val;
    this._render();
  }

  get displayDate() {
    return this._displayDate;
  }

  set displayDate(v) {
    this._displayDate = v;
    this._render();
  }
}

window.customElements.define('calendar-sk', CalendarSk);
