/**
 * @module modules/domain-picker-sk
 * @description <h2><code>domain-picker-sk</code></h2>
 *
 * Allows picking either a date range for commits, or for
 * picking a number of commits to show before a selected
 * date.
 *
 * @attr {string} force_request_type - A value of 'dense' or 'range' will
 *   force the corresponding request_type to be always set.
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { RequestType } from '../json';

import 'elements-sk/radio-sk';
import 'elements-sk/styles/buttons';
import '../calendar-input-sk';

// Types of domain ranges we can choose.
// TODO(jcgregorio) Make the underlying dataframe.RequestType a string.
const RANGE = 0; // Specify a begin and end time.
const DENSE = 1; // Specify an end time and the number of commits with data.

type ForceRequestType = 'range' | 'dense' | '';

const toDate = (seconds: number) => new Date(seconds * 1000);

const toForceRequestType = (s: string | null): ForceRequestType => {
  if (s === 'range') {
    return 'range';
  } if (s === 'dense') {
    return 'dense';
  }
  return '';
};

/** The state of the DomainPickerSk control. */
export interface DomainPickerState {
  /**  Beginning of time range in Unix timestamp seconds. */
  begin: number;
  /**  End of time range in Unix timestamp seconds. */
  end: number;
  /**
   * If RequestType is REQUEST_COMPACT (1), then this value is the number of
   * commits to show before End, and the value of Begin is ignored.
   */
  num_commits: number;
  request_type: RequestType;
}

export class DomainPickerSk extends ElementSk {
  private _state: DomainPickerState;

  constructor() {
    super(DomainPickerSk.template);
    const now = Date.now();
    this._state = {
      begin: Math.floor(now / 1000 - 24 * 60 * 60),
      end: Math.floor(now / 1000),
      num_commits: 50,
      request_type: RANGE,
    };
  }

  private static template = (ele: DomainPickerSk) => html`
    ${DomainPickerSk._showRadio(ele)}
    <div class="ranges">
      ${DomainPickerSk._requestType(ele)}
      <label>
        <span class="prefix">End:</span>
        <calendar-input-sk
          @change=${ele.endChange}
          .displayDate=${toDate(ele._state.end)}
        ></calendar-input-sk>
      </label>
    </div>
  `;

  private static _showRadio = (ele: DomainPickerSk) => {
    if (!ele.force_request_type) {
      return html`
        <radio-sk
          @change=${ele.typeRange}
          ?checked=${ele._state.request_type === RANGE}
          label="Date Range"
          name="daterange"
        ></radio-sk>
        <radio-sk
          @change=${ele.typeDense}
          ?checked=${ele._state.request_type === DENSE}
          label="Dense"
          name="daterange"
        ></radio-sk>
      `;
    }
    return html``;
  };

  private static _requestType = (ele: DomainPickerSk) => {
    if (ele._state.request_type === RANGE) {
      return html`
        <p>Display all points in the date range.</p>
        <label>
          <span class="prefix">Begin:</span>
          <calendar-input-sk
            @change=${ele.beginChange}
            .displayDate=${toDate(ele._state.begin)}
          ></calendar-input-sk>
        </label>
      `;
    }
    return html`
      <p>Display only the points that have data before the date.</p>
      <label>
        <span class="prefix">Points</span>
        <input
          @change=${ele.numChanged}
          type="number"
          .value="${ele._state.num_commits}"
          min="1"
          max="5000"
          list="defaultNumbers"
          title="The number of points."
        />
      </label>
      <datalist id="defaultNumbers">
        <option value="50"></option>
        <option value="100"></option>
        <option value="250"></option>
        <option value="500"></option>
      </datalist>
    `;
  };


  connectedCallback(): void {
    super.connectedCallback();
    this._upgradeProperty('state');
    this._upgradeProperty('force_request_type');
    this.render();
  }

  attributeChangedCallback(): void {
    this.render();
  }

  render(): void {
    if (this.force_request_type === 'dense') {
      this._state.request_type = DENSE;
    } else if (this.force_request_type === 'range') {
      this._state.request_type = RANGE;
    }
    super._render();
  }

  private typeRange() {
    this._state.request_type = RANGE;
    this.render();
  }

  private typeDense() {
    this._state.request_type = DENSE;
    this.render();
  }

  private beginChange(e: CustomEvent<Date>) {
    this._state.begin = Math.floor(e.detail.valueOf() / 1000);
    this.render();
  }

  private endChange(e: CustomEvent<Date>) {
    this._state.end = Math.floor(e.detail.valueOf() / 1000);
    this.render();
  }

  private numChanged(e: MouseEvent) {
    this._state.num_commits = +(e.target! as HTMLInputElement).value;
    this.render();
  }

  static get observedAttributes(): string[] {
    return ['force_request_type'];
  }

  get state(): DomainPickerState {
    return this._state;
  }

  set state(val: DomainPickerState) {
    if (!val) {
      return;
    }
    this._state = {
      begin: val.begin,
      end: val.end,
      num_commits: val.num_commits,
      request_type: val.request_type,
    };
    this.render();
  }

  /**  A value of DENSE or RANGE will force the corresponding request_type to be always set.
   */
  get force_request_type(): ForceRequestType {
    return toForceRequestType(this.getAttribute('force_request_type'));
  }

  set force_request_type(val: ForceRequestType) {
    this.setAttribute('force_request_type', toForceRequestType(val));
  }
}

define('domain-picker-sk', DomainPickerSk);
