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
import 'elements-sk/select-sk';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { $, $$ } from 'common-sk/modules/dom';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

// TODO(jcgregorio) select-sk needs something like attr-for-selected and
// fallback-selection like iron-selector.
const template = (ele) => html`
  <select-sk @selection-changed=${ele._selectionChanged}>
    <div value=kmeans ?selected=${ele.algo === 'kmeans'} title="Use k-means clustering on the trace shapes and look for a step on the cluster centroid.">K-Means</div>
    <div value=stepfit ?selected=${ele.algo === 'stepfit'} title="Look for a step in each individual trace.">Individual</div>
  </select-sk>
  `;

define('algo-select-sk', class extends ElementSk {
  constructor() {
    super(template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._upgradeProperty('algo');
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
    this.dispatchEvent(new CustomEvent('algo-change', { detail, bubbles: true }));
  }

  /** @prop algo {string} The algorithm. */
  get algo() { return this.getAttribute('algo'); }

  set algo(val) {
    this.setAttribute('algo', val);
  }

  attributeChangedCallback() {
    this._render();
  }
});
