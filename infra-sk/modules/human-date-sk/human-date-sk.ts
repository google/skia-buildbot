/**
 * @module modules/human-date-sk
 * @description <h2><code>human-date-sk</code></h2>
 *
 * An element that displays a date in a human-readable format, optionally as a diff from now.
 *
 * @property date - The date to make human-readable.
 * @property diff - Rather than displaying the date, display the difference between the
 * current date and the given date.
 * @property seconds - Indicates that the given date is expressed in seconds, not milliseconds.
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { diffDate } from 'common-sk/modules/human';
import { upgradeProperty } from 'elements-sk/upgradeProperty';
import { ElementSk } from '../ElementSk';

export class HumanDateSk extends ElementSk {
  private static template = (el: HumanDateSk) => html`<span title="${el.humanDate(false)}">${el.humanDate(el.diff)}</span>`;

  private _date: string | number = 0;

  private _diff: boolean = false;

  private _seconds: boolean = false;

  constructor() {
    super(HumanDateSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    upgradeProperty(this, 'date');
    upgradeProperty(this, 'diff');
    upgradeProperty(this, 'seconds');
    this._render();
  }

  humanDate(diff: boolean) {
    let millis: number = 0;
    // Take the input and convert it to milliseconds.
    if (typeof this._date === 'number') {
      millis = this._date;
      if (this.seconds) {
        millis *= 1000;
      }
    } else {
      millis = new Date(this._date).getTime();
    }

    if (diff) {
      return diffDate(millis) + " ago";
    }
    const d = new Date(millis);
    return `${d.toLocaleDateString()}, ${d.toLocaleTimeString()}`;
  }

  set date(value: string | number) {
    this._date = value;
    this._render();
  }

  get date(): string | number {
    return this._date;
  }

  set diff(value: boolean) {
    this._diff = value;
    this._render();
  }

  get diff(): boolean {
    return this._diff;
  }

  set seconds(value: boolean) {
    this._seconds = value;
    this._render();
  }

  get seconds(): boolean {
    return this._seconds;
  }
}

define('human-date-sk', HumanDateSk);
