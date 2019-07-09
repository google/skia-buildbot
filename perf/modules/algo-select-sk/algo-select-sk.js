/**
 * @module modules/algo-select-sk
 * @description <h2><code>algo-select-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { html, render } from 'lit-html'
import { upgradeProperty } from 'elements-sk/upgradeProperty';

const template = (ele) => html`
  <select-sk @selection-changed=${_selectionChanged} selected="{{algo}}" fallback-selection="kmeans">
    <div value=kmeans title="Use k-means clustering on the trace shapes.">K-Means</div>
    <div value=stepfit title="Only look for traces that step up or down at the selected commit.">StepFit</div>
    <div value=tail title="Only look for traces with a jumping tail.">Tail</div>
  </select-sk>
  `;

window.customElements.define('algo-select-sk', class extends HTMLElement {
  constructor() {
    super();
  }

  connectedCallback() {
    upgradeProperty(this, 'algo');
    this._render();
  }

  static get observedAttributes() {
    return ['algo'];
  }

  _selectionChanged(e) {
    // Turn e.detail.selection, which is an index, into a new algo value.
  }

  /** @prop algo {string} The algorithm. */
  get algo() { return this.getAttribute('algo'); }
  set algo(val) { this.setAttribute('algo', val); }

  attributeChangedCallback(name, oldValue, newValue) {
    this._render();
  }

  _render() {
    render(template(this), this, {eventContext: this});
  }

});
