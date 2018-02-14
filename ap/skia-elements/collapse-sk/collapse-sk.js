import { upgradeProperty } from '../dom'

// The <collapse-sk> custom element declaration.
//
//  Attributes:
//    closed - A boolean attribute that, if present, causes the element to
//             collapse, i.e., transition to display: none.
//
//  Properties:
//    closed - Mirrors the 'closed' attribute.
//
//  Events:
//    None
//
//  Methods:
//    None
//
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
