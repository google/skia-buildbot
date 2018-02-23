import { jsonOrThrow } from 'common/jsonOrThrow'

import { html, render } from 'lit-html/lib/lit-extended'

// How often to update the data.
const UPDATE_INTERVAL_MS = 60000;

const template = (ele) => html`
<h2>Hello custom element</h2>`;

window.customElements.define('power-index-sk', class extends HTMLElement {

  connectedCallback() {
    this._render();
    window.setTimeout(this.update.bind(this));
  }

  update() {
    fetch('/down_bots')
      .then(jsonOrThrow)
      .then((json) => {
        console.log('Got json', json)
        window.setTimeout(this.update.bind(this), UPDATE_INTERVAL_MS);
      })
      .catch((e) => {
        console.log('got err', e);
        window.setTimeout(this.update.bind(this), UPDATE_INTERVAL_MS);
      });
  }

  _render() {
    render(template(this), this);
  }

});
