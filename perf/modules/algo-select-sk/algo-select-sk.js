/**
 * @module modules/algo-select-sk
 * @description <h2><code>algo-select-sk</code></h2>
 *
 * Displays and allows changing the clustering algorithm.
 *
 * @evt algo-change - Sent when the algo has changed. The value is stored
 *    in e.detail.algo.
 *
 * @attr {string} algo - The algorithm name.
 */
import 'elements-sk/select-sk'
import { html, render } from 'lit-html'
import { $, $$ } from 'common-sk/modules/dom'
import { upgradeProperty } from 'elements-sk/upgradeProperty'

const _fromName = (ele) => {
  const target = ele.algo;
  const divs = $('div', ele);
  for (let i = divs.length - 1; i >= 0; i--) {
    if (divs[i].getAttribute('value') === target) {
      return i;
    }
  }
  return 0;
}

// TODO(jcgregorio) select-sk needs something like attr-for-selected and
// fallback-selection like iron-selector.
const template = (ele) => html`
  <select-sk @selection-changed=${ele._selectionChanged} .selection=${_fromName(ele)}>
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
    this._select = $$('select-sk', this);
  }

  static get observedAttributes() {
    return ['algo'];
  }

  _selectionChanged(e) {
    let index = e.detail.selection;
    if (index < 0) {
      index = 0;
    }
    this.algo = $('div', this)[index].getAttribute('value');
    const detail = {
      algo: this.algo,
    };
    this.dispatchEvent(new CustomEvent('algo-change', { detail: detail, bubbles: true }));
  }

  /** @prop algo {string} The algorithm. */
  get algo() { return this.getAttribute('algo'); }
  set algo(val) {
    this.setAttribute('algo', val);
    this._render();
  }

  attributeChangedCallback(name, oldValue, newValue) {
    this._render();
  }

  _render() {
    render(template(this), this, {eventContext: this});
  }

});
