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
 * @example
 */
import { define } from 'elements-sk/define'
import { html, render } from 'lit-html'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import 'elix/src/DateComboBox.js'

const template = (ele) => html`
  <label>Begin <elix-date-combo-box @date-changed=${ele._beginChanged} .date=${new Date(ele.begin * 1000)}></elix-date-combo-box></label>
  <label>End <elix-date-combo-box @date-changed=${ele._endChanged} .date=${new Date(ele.end * 1000)}></elix-date-combo-box></label>
`;

define('day-range-sk', class extends ElementSk {
  constructor() {
    super(template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._upgradeProperty('begin');
    this._upgradeProperty('end');
    const now = Date.now();
    if (!this.begin) {
      this.begin = now/1000 - 24*60*60;
    }
    if (!this.end) {
      this.end = now/1000;
    }
    this._render();
  }

  _sendEvent() {
    const detail = {
      begin: this.begin,
      end: this.end,
    };
    this.dispatchEvent(new CustomEvent('day-range-change', {detail: detail, bubbles: true}));
  }

  _beginChanged(e) {
    this.begin = e.detail.date / 1000;
    this._sendEvent();
  }

  _endChanged(e) {
    this.end = e.detail.date / 1000;
    this._sendEvent();
  }

  static get observedAttributes() {
    return ['begin', 'end'];
  }

  /** @prop begin {string} Mirrors the 'begin' attribute. */
  get begin() { return +this.getAttribute('begin'); }
  set begin(val) { this.setAttribute('begin', ''+val); }

  /** @prop end {string} Mirros the 'end' attribute. */
  get end() { return +this.getAttribute('end'); }
  set end(val) { this.setAttribute('end', val); }

  attributeChangedCallback(name, oldValue, newValue) {
    this._render();
  }

});
