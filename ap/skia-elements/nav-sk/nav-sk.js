import '../icon-sk';
import '../buttons';
import { upgradeProperty } from '../dom'

const navButtonSk = document.createElement('template');
navButtonSk.innerHTML = `<button><icon-menu-sk></icon-menu-sk></button>`;

// TODO(jcgregorio) Add support for 'ESC' key and clicking outside
// the element to close the nav-links-sk.
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
