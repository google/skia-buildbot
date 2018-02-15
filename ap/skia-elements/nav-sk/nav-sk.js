import '../icon-sk';
import '../buttons';
import { upgradeProperty } from '../dom'

const navButtonSk = document.createElement('template');
navButtonSk.innerHTML = `<button><icon-menu-sk></icon-menu-sk></button>`;

// TODO(jcgregorio) Add support for 'ESC' key and clicking outside
// the element to close the nav-links-sk.

// The <nav-button-sk> custom element declaration.
//
// Allows for the creation of a pop-up menu. The actual menu is contained
// in a sibling <nav-links-sk> element. For example:
//
//    <nav-button-sk></nav-button-sk>
//    <nav-links-sk closed>
//      <a href="">Main</a>
//      <a href="">Triage</a>
//      <a href="">Alerts</a>
//    </nav-links-sk closed>
//
//  Attributes:
//    None
//
//  Properties:
//    None
//
//  Events:
//    None
//
//  Methods:
//    None
//
window.customElements.define('nav-button-sk', class extends HTMLElement {
  connectedCallback() {
    this.addEventListener('click', this);
    let icon = navButtonSk.content.cloneNode(true);
    this.appendChild(icon);
  }

  disconnectedCallback() {
    this.removeEventListener('click', this);
  }

  handleEvent(e) {
    if (this.nextElementSibling.tagName === "NAV-LINKS-SK") {
      this.nextElementSibling.closed = !this.nextElementSibling.closed;
    }
  }
});

// The <nav-links-sk> custom element declaration.
//
// See the documentation above for nav-button-sk.
//
//  Attributes:
//    closed - A boolean attribute controlling if the list of links
//             is displayed or not.
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
window.customElements.define('nav-links-sk', class extends HTMLElement {
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
