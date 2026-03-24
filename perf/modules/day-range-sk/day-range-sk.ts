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
import { html, LitElement } from 'lit';
import { property, customElement } from 'lit/decorators.js';
import '../calendar-input-sk';

export interface DayRangeSkChangeDetail {
  // Seconds from the Unix epoch.
  begin: number;

  // Seconds from the Unix epoch.
  end: number;
}

// Converts the timestamp in seconds from the epoch into a Date.
function dateFromTimestamp(timestamp: number) {
  return new Date(timestamp * 1000);
}

@customElement('day-range-sk')
export class DayRangeSk extends LitElement {
  @property({ type: Number })
  begin = Math.floor(Date.now() / 1000) - 24 * 60 * 60;

  @property({ type: Number })
  end = Math.floor(Date.now() / 1000);

  createRenderRoot() {
    return this;
  }

  protected render() {
    return html`
      <label class="begin">
        Begin
        <calendar-input-sk
          @input=${this._beginChanged}
          .displayDate=${dateFromTimestamp(this.begin)}></calendar-input-sk>
      </label>
      <label class="end">
        End
        <calendar-input-sk
          @input=${this._endChanged}
          .displayDate=${dateFromTimestamp(this.end)}></calendar-input-sk>
      </label>
    `;
  }

  private _sendEvent() {
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

  private _beginChanged(e: CustomEvent<Date>) {
    this.begin = Math.floor(e.detail.valueOf() / 1000);
    this._sendEvent();
  }

  private _endChanged(e: CustomEvent<Date>) {
    this.end = Math.floor(e.detail.valueOf() / 1000);
    this._sendEvent();
  }
}
