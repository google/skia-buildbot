/** @module skia-elements/nav-sk */
import '../icon-sk';
import '../buttons';
import { upgradeProperty } from '../upgradeProperty'

/**
 * <code>nav-button-sk<code>
 *
 * <p>
 *   Allows for the creation of a pop-up menu. The actual menu is contained
 *   in a sibling <nav-links-sk> element.
 * </p>
 *
 * <p>
 *   <code>nav-button-sk</code> is just a convenience that contains the
 *   hamburger menu and toggles the shown property of a sibling
 *   'nav-links-sk'. Other types of popup menus can be created using buttons
 *   and icons directly.  The children of 'nav-links-sk' do not have to be
 *   links, they could be other elements, such as buttons.
 * </p>
 *
 *
 * @example
 *
 * // Using nav-button-sk:
 * <nav-button-sk></nav-button-sk>
 * <nav-links-sk shown>
 *   <a href="">Main</a>
 *   <a href="">Triage</a>
 *   <a href="">Alerts</a>
 * </nav-links-sk>
 *
 * // Using a button instead of nav-button-sk:
 * <button onclick="this.nextElementSibling.shown = true;">
 *   <icon-create-sk></icon-create-sk>
 * </button>
 * <nav-links-sk>
 *   <a href="">New A</a>
 *   <a href="">New B</a>
 *   <a href="">New C</a>
 * </nav-links-sk>
 *
 */
class NavButtonSk extends HTMLElement {
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
}

window.customElements.define('nav-button-sk', NavButtonSk);

/**
 * <code>nav-links-sk</code>
 *
 * <p>
 *   See the documentation for {@link module:skia-elements/nav-sk~NavButtonSk}.
 * </p>
 *
 * <p>
 *   The nav-links-sk will be closed if the user presses ESC, or if focus
 *   moves off of nav-links-sk or any of its children.
 * </p>
 *
 * @attr shown - A boolean attribute controlling if the list of links is
 *   displayed or not.
 *
 */
class NavLinksSk extends HTMLElement {
  static get observedAttributes() {
    return ['shown'];
  }

  connectedCallback() {
    upgradeProperty(this, 'shown');
  }

  /** @prop shown {boolean} Mirrors the shown attribute. */
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
}

window.customElements.define('nav-links-sk', NavLinksSk);
