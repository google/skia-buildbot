import '../icon-sk';
import '../buttons';
import { upgradeProperty } from '../dom'

// The <nav-button-sk> custom element declaration.
//
// Allows for the creation of a pop-up menu. The actual menu is contained
// in a sibling <nav-links-sk> element. For example:
//
//    <nav-button-sk></nav-button-sk>
//    <nav-links-sk shown>
//      <a href="">Main</a>
//      <a href="">Triage</a>
//      <a href="">Alerts</a>
//    </nav-links-sk>
//
//  <nav-button-sk> is just a convenience that contains the hamburger menu
//  and toggles the shown property of a sibling 'nav-links-sk'. Other types
//  of popup menus can be created using buttons and icons directly. For
//  example:
//
//    <button onclick="this.nextElementSibling.shown = true;">
//      <icon-create-sk></icon-create-sk>
//    </button>
//    <nav-links-sk>
//      <a href="">New A</a>
//      <a href="">New B</a>
//      <a href="">New C</a>
//    </nav-links-sk>
//
//
//  The children of 'nav-links-sk' does not have to be links, it could
//  be other elements, such as buttons.
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
    this.innerHTML = `<button><icon-menu-sk></icon-menu-sk></button>`;
  }

  disconnectedCallback() {
    this.removeEventListener('click', this);
  }

  handleEvent(e) {
    if (this.nextElementSibling.tagName === "NAV-LINKS-SK") {
      this.nextElementSibling.shown = !this.nextElementSibling.shown;
      if (this.nextElementSibling.shown) {
        this.nextElementSibling.firstElementChild.focus();
      }
    }
  }
});

// The <nav-links-sk> custom element declaration.
//
// See the documentation above for nav-button-sk.
//
// The nav-links-sk will closed if the user presses ESC, or if focus moves off
// of <nav-links-sk> or any of its children.
//
//  Attributes:
//    shown - A boolean attribute controlling if the list of links
//             is displayed or not.
//
//  Properties:
//    shown - Mirrors the 'shown' attribute.
//
//  Methods:
//    None
//
//  Events:
//    closed - This event is generated when nav-links-sk is closed.
//
window.customElements.define('nav-links-sk', class extends HTMLElement {
  static get observedAttributes() {
    return ['shown'];
  }

  connectedCallback() {
    upgradeProperty(this, 'shown');
  }

  get shown() { return this.hasAttribute('shown'); }
  set shown(val) {
    if (val) {
      this.setAttribute('shown', '');
    } else {
      this.removeAttribute('shown');
    }
  }

  attributeChangedCallback(name, oldValue, newValue) {
    if (newValue !== null) {
      window.addEventListener('keydown', this);
      window.addEventListener('focusin', this);
    } else {
      window.removeEventListener('keydown', this);
      window.removeEventListener('focusin', this);
      this.dispatchEvent(new CustomEvent('closed', { bubbles: true }));
    }
  }

  handleEvent(e) {
    if (e.type === 'keydown') {
        if (e.key === "Escape") {
          e.preventDefault();
          this.shown = false;
        }
    } else {
      // If focus is not on 'this' or its children then close.
      let ele = e.target;
      while (ele !== this && ele !== null) {
        ele = ele.parentElement;
      }
      if (!ele) {
        this.shown = false;
      }
    }
  }
});

