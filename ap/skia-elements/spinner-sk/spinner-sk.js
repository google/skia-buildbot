import { upgradeProperty } from '../dom'

// The <spinner-sk> custom element declaration.
//
//  Attributes:
//    active - Boolean attribute, if present, spinner is active.
//
//  Properties:
//    active - Mirrors the 'active' attribute.
//
//  Events:
//    None
//
//  Methods:
//    None
//
//  TODO(jcgregorio) What is ARIA for a spinner?
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
