/**
 * @module skia-elements/nav-button-sk
 * @description <h2><code>nav-button-sk<code></h2>
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
import '../icon-sk';
import '../buttons';
import { upgradeProperty } from '../upgradeProperty'

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
