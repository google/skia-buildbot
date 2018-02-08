import { upgradeProperty } from '../dom'

window.customElements.define('spinner-sk', class extends HTMLElement {
  connectedCallback() {
    upgradeProperty(this, 'active');
  }

  get active() { return this.hasAttribute('active'); }
  set active(val) {
    if (val) {
      this.setAttribute('active', '');
    } else {
      this.removeAttribute('active');
    }
  }
});
