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
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

const getDaysInMonth = (year: number, monthIndex: number) => {
  return new Date(year, monthIndex + 1, 0).getDate();
};

const oneWeek = new Array(7);

const row = (year: number, monthIndex: number, week: number) => {
  // Start at the first day of the month.
  const date = new Date(year, monthIndex);
  console.log(date);
  const theFirstDayOfTheMonthFallsOn = date.getDay();
  const delta = -theFirstDayOfTheMonthFallsOn;
  const daysInMonth = getDaysInMonth(year, monthIndex);
  return html`<tr>
    ${new Array(7).fill(1).map((_, i) => {
      const d = 7 * week + i + delta + 1;
      if (d >= 1 && d <= daysInMonth) {
        return html`<td>${d}</td>`;
      } else {
        return html`<td></td>`;
      }
    })}</tr
  >`;
};

export class CalendarSk extends ElementSk {
  private static template = (ele: CalendarSk) => html`
    <table>
      ${new Array(6)
        .fill(1)
        .map((_, i) => row(ele._year(), ele._monthIndex(), i))}
    </table>
  `;

  private _date: Date = new Date(2020, 5, 26);

  constructor() {
    super(CalendarSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  private _year(): number {
    return this._date.getFullYear();
  }

  private _monthIndex(): number {
    return this._date.getMonth();
  }

  /** The selected date. */
  get date() {
    return this._date;
  }

  set date(val) {
    this._date = val;
  }
}

window.customElements.define('calendar-sk', CalendarSk);
