/**
 * @module modules/day-range-sk
 * @description <h2><code>day-range-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { html, render } from 'lit-html'
import 'elix/src/DateComboBox.js'
import { upgradeProperty } from 'elements-sk/upgradeProperty';

const template = (ele) => html`
  <elix-date-combo-box .date=${new Date(ele.begin * 1000)}></elix-date-combo-box>
  <elix-date-combo-box .date=${new Date(ele.end * 1000)}></elix-date-combo-box>
`;

window.customElements.define('day-range-sk', class extends HTMLElement {
  constructor() {
    super();
  }

  connectedCallback() {
    upgradeProperty(this, 'begin');
    upgradeProperty(this, 'end');
    const now = Date.now();
    if (!this.begin) {
      this.begin = now/1000 - 24*60*60;
    }
    if (!this.end) {
      this.end= now/1000;
    }
    this._render();
  }

  disconnectedCallback() {
  }

  static get observedAttributes() {
    return ['begin', 'end'];
  }

  /** @prop begin {string} Unix time in seconds since the epoch. */
  get begin() { return +this.getAttribute('begin'); }
  set begin(val) { this.setAttribute('begin', ''+val); }

  /** @prop end {string} Unit time in seconds since the epoch. */
  get end() { return this.getAttribute('end'); }
  set end(val) { this.setAttribute('end', val); }

  attributeChangedCallback(name, oldValue, newValue) {
    this._render();
  }

  _render() {
    render(template(this), this, {eventContext: this});
  }

});
