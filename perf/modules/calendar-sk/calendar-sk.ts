/**
 * @module modules/calendar-sk
 * @description <h2><code>calendar-sk</code></h2>
 *
 * Displays a calendar, one month at a time, and allows selecting a single day.
 * Offers the ability to navigate by both month and year.
 *
 * @evt change - A CustomEvent with the selected Date in the detail.
 *
 */
import { html, TemplateResult } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/styles/buttons';
import 'elements-sk/icon/navigate-before-icon-sk';
import 'elements-sk/icon/navigate-next-icon-sk';

/*
  The built in Date object for JS is inconsistent, years and days of the month
  are 1-indexed, but months, for some reason, are 0-indexed. So to remind us of
  that insanity in all the code below years are named 'year', days of the month
  are named 'date' and months are named 'monthIndex'.
*/

export interface CalendarSkChangeEventDetail {
  date: Date;
}

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
  private static buttonForDate = (
    ele: CalendarSk,
    d: number,
    daysInMonth: number,
    selected: boolean
  ) => {
    if (d < 1 || d > daysInMonth) {
      return html``;
    }
    return html`<button
      @click=${ele.dateClick}
      data-date=${d}
      tabindex=${selected ? 0 : -1}
      aria-selected=${selected}
    >
      ${d}
    </button>`;
  };

  private static row = (ele: CalendarSk, weekIndex: number) => {
    const year = ele.displayYear();
    const monthIndex = ele.displayMonthIndex();
    const today = calendarDateFromDate(new Date());

    // If June starts on a Tuesday then IndexOfTheFirstDayOfTheMonth = 2,
    // which means if we are filling in the first row we want to leave the first
    // two days (Sunday and Monday) blank.
    const IndexOfTheFirstDayOfTheMonth = firstDayOfMonth(year, monthIndex);
    const daysInMonth = getDaysInMonth(year, monthIndex);
    const selectedDate = ele._displayDate.getDate();
    const currentDate = {
      year,
      monthIndex,
      date: -1,
    };
    return html`<tr>
      ${sevenDaysInAWeek.map((_, i) => {
        const d = 7 * weekIndex + i + 1 - IndexOfTheFirstDayOfTheMonth;
        currentDate.date = d;
        const selected = selectedDate === d;
        return html`<td
          class="
            ${equal(currentDate, today) ? 'today' : ''}
            ${selected ? 'selected' : ''}
          "
        >
          ${CalendarSk.buttonForDate(ele, d, daysInMonth, selected)}
        </td>`;
      })}</tr
    >`;
  };

  private static template = (ele: CalendarSk) => html`
    <table>
      <tr>
        <th>
          <button @click=${ele.decYear} aria-label="Previous year">
            <navigate-before-icon-sk></navigate-before-icon-sk>
          </button>
        </th>
        <th colspan="5">
          <h2 aria-live="polite">
            ${new Intl.DateTimeFormat(ele._locale, { year: 'numeric' }).format(
              ele._displayDate
            )}
          </h2>
        </th>
        <th>
          <button @click=${ele.incYear} aria-label="Next year">
            <navigate-next-icon-sk></navigate-next-icon-sk>
          </button>
        </th>
      </tr>
      <tr>
        <th>
          <button @click=${ele.decMonth} aria-label="Previous month">
            <navigate-before-icon-sk></navigate-before-icon-sk>
          </button>
        </th>
        <th colspan="5">
          <h2 aria-live="polite">
            ${new Intl.DateTimeFormat(ele._locale, { month: 'long' }).format(
              ele._displayDate
            )}
          </h2>
        </th>
        <th>
          <button @click=${ele.incMonth} aria-label="Next month">
            <navigate-next-icon-sk></navigate-next-icon-sk>
          </button>
        </th>
      </tr>
      ${ele._weekDayHeader} ${sixWeeks.map((_, i) => CalendarSk.row(ele, i))}
    </table>
  `;

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
        (_, i) =>
          html`<td>
            <span
              aria-label="${longFormatter.format(new Date(2020, 2, i + 1))}"
            >
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
      new CustomEvent<CalendarSkChangeEventDetail>('change', {
        detail: {
          date: d,
        },
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
    const daysInMonth = getDaysInMonth(year + 1, monthIndex);
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
    const daysInMonth = getDaysInMonth(year - 1, monthIndex);
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

    const daysInMonth = getDaysInMonth(year, monthIndex);
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

    const daysInMonth = getDaysInMonth(year, monthIndex);
    if (date > daysInMonth) {
      date = daysInMonth;
    }

    this._displayDate = new Date(year, monthIndex, date);
    this._render();
  }

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
