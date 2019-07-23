/**
 * @module modules/query-summary-sk
 * @description <h2><code>query-summary-sk</code></h2>
 *
 * Displays a summary of a selection made using query-sk.
 *
 * @attr {string} url - If supplied, the displayed summary will be a link to the given URL.
 *
 * @attr {string} selection - A query-sk selection formatted as query parameters to be displayed.
 *
 */
import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import { html, render } from 'lit-html'
import { toParamSet } from 'common-sk/modules/query'

const template = (ele) => {
  if (ele.url) {
    return html`<a href=${ele.url}><pre>${ele._display()}</pre></a>`;
  } else {
    return html`<pre>${ele._display()}</pre>`;
  }
}

window.customElements.define('query-summary-sk', class extends ElementSk {
  constructor() {
    super(template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  _display() {
    if (!this.selection) {
      return '[No filters applied]';
    }
    const params = toParamSet(this.selection);
    const keys = Object.keys(params);
    keys.sort();
    const ret = [];
    keys.forEach((key) => {
      params[key].forEach((value) => {
        ret.push(`${key}=${value}`);
      });
    });
    return ret.join('\n');
  }

  static get observedAttributes() {
    return ['url', 'selection'];
  }

  /** @prop url {string} Mirrors the 'url' attribute. */
  get url() { return this.getAttribute('url'); }
  set url(val) { this.setAttribute('url', val); }

  /** @prop selection {string} Mirrors the 'selection' attribute. */
  get selection() { return this.getAttribute('selection'); }
  set selection(val) { this.setAttribute('selection', val); }

  attributeChangedCallback(name, oldValue, newValue) {
    this._render();
  }

});
