import { $$ } from './core.js'

const iconSkTemplate = document.createElement('template');
iconSkTemplate.innerHTML = `<svg class="icon-sk-svg" viewBox="0 0 24 24" preserveAspectRatio="xMidYMid meet" focusable="false">
  <g><path d=""></path></g>
</svg>`;

class IconSk extends HTMLElement {
  connectedCallback() {
    let icon = iconSkTemplate.content.cloneNode(true);
    $$('path', icon)[0].setAttribute('d', this.constructor._path);
    this.appendChild(icon);
  }
}

// TODO Break out each icon into its own file so they can be selectively included.
// TODO Generate all icons from the Polymer set.
window.customElements.define('icon-menu-sk', class extends IconSk {
  static get _path() { return "M3 18h18v-2H3v2zm0-5h18v-2H3v2zm0-7v2h18V6H3z"; }
});

window.customElements.define('icon-link-sk', class extends IconSk {
  static get _path() { return "M3.9 12c0-1.71 1.39-3.1 3.1-3.1h4V7H7c-2.76 0-5 2.24-5 5s2.24 5 5 5h4v-1.9H7c-1.71 0-3.1-1.39-3.1-3.1zM8 13h8v-2H8v2zm9-6h-4v1.9h4c1.71 0 3.1 1.39 3.1 3.1s-1.39 3.1-3.1 3.1h-4V17h4c2.76 0 5-2.24 5-5s-2.24-5-5-5z"; }
});
