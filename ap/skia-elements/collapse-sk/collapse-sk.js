import { upgradeProperty } from '../dom'

window.customElements.define('collapse-sk', class extends HTMLElement {
  connectedCallback() {
    upgradeProperty(this, 'closed');
  }

  get closed() { return this.hasAttribute('closed'); }
  set closed(val) {
    if (val) {
      this.setAttribute('closed', '');
    } else {
      this.removeAttribute('closed');
    }
  }
});
