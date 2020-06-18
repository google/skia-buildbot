/**
 * @module modules/day-range-sk
 * @description <h2><code>day-range-sk</code></h2>
 *
 * @evt day-range-change - Fired when the selection has stopped changing. Contains
 *      the begin and end timestamps in the details:
 *
 *      {
 *        begin: 1470084997,
 *        end: 1474184677
 *      }
 *
 * @attr {Number} begin - The beginning of the time range in seconds since the epoch.
 * @attr {Number} end - The end of the time range in seconds since the epoch.
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

export interface DayRangeSkChangeDetail {
  // Seconds from the Unix epoch.
  begin: number;

  // Seconds from the Unix epoch.
  end: number;
}

// Converts the timestamp in seconds from the epoch into
// an acceptable format for an input of type=date.
function inputDateFormatFromNumber(timestamp: number) {
  const d = new Date(timestamp * 1000);

  const month = `${d.getUTCMonth() + 1}`.padStart(2, '0');
  const day = `${d.getUTCDate()}`.padStart(2, '0');
  const year = d.getUTCFullYear();
  return `${year}-${month}-${day}`;
}

export class DayRangeSk extends ElementSk {
  static template = (ele: DayRangeSk) => html`
    <label class="begin">
      Begin
      <input
        type=date
        @change=${ele._beginChanged}
        .value=${inputDateFormatFromNumber(ele.begin)}
      ></input>
    </label>
    <label class="end">
      End
      <input
        type=date
        @change=${ele._endChanged}
        .value=${inputDateFormatFromNumber(ele.end)}
      ></input>
    </label>
  `;

  constructor() {
    super(DayRangeSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._upgradeProperty('begin');
    this._upgradeProperty('end');
    const now = Date.now();
    if (!this.begin) {
      this.begin = now / 1000 - 24 * 60 * 60;
    }
    if (!this.end) {
      this.end = now / 1000;
    }
    this._render();
  }

  _sendEvent() {
    const detail = {
      begin: this.begin,
      end: this.end,
    };
    this.dispatchEvent(
      new CustomEvent<DayRangeSkChangeDetail>('day-range-change', {
        detail,
        bubbles: true,
      })
    );
  }

  _beginChanged(e: Event) {
    this.begin = (e.target! as HTMLInputElement).valueAsNumber / 1000;
    this._sendEvent();
  }

  _endChanged(e: Event) {
    this.end = (e.target! as HTMLInputElement).valueAsNumber / 1000;
    this._sendEvent();
  }

  static get observedAttributes() {
    return ['begin', 'end'];
  }

  /** @prop begin - Mirrors the 'begin' attribute. */
  get begin() {
    return +(this.getAttribute('begin') || '0');
  }

  set begin(val) {
    this.setAttribute('begin', `${val}`);
  }

  /** @prop end - Mirrors the 'end' attribute. */
  get end() {
    return +(this.getAttribute('end') || '0');
  }

  set end(val) {
    this.setAttribute('end', `${val}`);
  }

  attributeChangedCallback() {
    this._render();
  }
}

define('day-range-sk', DayRangeSk);
