/**
 * @module modules/calendar-sk
 * @description <h2><code>calendar-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { html, TemplateResult } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

const getDaysInMonth = (year: number, monthIndex: number) => {
  // Jump forward one month, and back one day to get the last day of the month.
  return new Date(year, monthIndex + 1, 0).getDate();
};

const sevenDaysInAWeek = new Array(7).fill(1);

// To display a month we need to display up to 6 weeks.
const sixWeeks = new Array(6).fill(1);

const row = (year: number, monthIndex: number, weekIndex: number) => {
  // If June starts on a Tuesday then IndexOfTheFirstDayOfTheMonth = 2,
  // which means if we are filling in the first row we want to leave the first
  // two days (Sunday and Monday) blank.
  const IndexOfTheFirstDayOfTheMonth = new Date(year, monthIndex).getDay();
  const daysInMonth = getDaysInMonth(year, monthIndex);
  return html`<tr>
    ${sevenDaysInAWeek.map((_, i) => {
      const d = 7 * weekIndex + i - IndexOfTheFirstDayOfTheMonth + 1;
      return html`<td>${d >= 1 && d <= daysInMonth ? d : ''}</td>`;
    })}</tr
  >`;
};

export class CalendarSk extends ElementSk {
  private static template = (ele: CalendarSk) => html`
    <table>
      <tr>
        <th class="clickable" @click=${ele._decYear}>&lsaquo;</th>
        <th colspan="5"
          >${new Intl.DateTimeFormat(undefined, { year: 'numeric' }).format(
            ele._displayDate
          )}</th
        >
        <th class="clickable" @click=${ele._incYear}>&rsaquo;</th>
      </tr>
      <tr>
        <th class="clickable" @click=${ele._decMonth}>&lsaquo;</th>
        <th colspan="5"
          >${new Intl.DateTimeFormat(undefined, { month: 'long' }).format(
            ele._displayDate
          )}</th
        >
        <th class="clickable" @click=${ele._incMonth}>&rsaquo;</th>
      </tr>
      ${ele._weekDayHeader}
      ${sixWeeks.map((_, i) => row(ele._year(), ele._monthIndex(), i))}
    </table>
  `;

  private _displayDate: Date = new Date();
  private _date: Date = new Date();
  private _weekDayHeader: TemplateResult;

  constructor() {
    super(CalendarSk.template);

    // March 1, 2020 falls on a Sunday, use that to generate the week day headers.
    const formatter = new Intl.DateTimeFormat(undefined, { weekday: 'narrow' });
    this._weekDayHeader = html`<tr class="weekdayHeader"
      >${sevenDaysInAWeek.map(
        (_, i) => html`<td>${formatter.format(new Date(2020, 2, i + 1))}</td>`
      )}</tr
    >`;
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  private _year(): number {
    return this._displayDate.getFullYear();
  }

  private _monthIndex(): number {
    return this._displayDate.getMonth();
  }

  private _incYear() {
    this._displayDate = new Date(this._year() + 1, this._monthIndex());
    this._render();
  }

  private _decYear() {
    this._displayDate = new Date(this._year() - 1, this._monthIndex());
    this._render();
  }

  private _incMonth() {
    let year = this._year();
    let monthIndex = this._monthIndex();
    monthIndex += 1;
    if (monthIndex > 11) {
      monthIndex = 0;
      year += 1;
    }
    this._displayDate = new Date(year, monthIndex);
    this._render();
  }

  private _decMonth() {
    let year = this._year();
    let monthIndex = this._monthIndex();
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
  }
}

window.customElements.define('calendar-sk', CalendarSk);
