import { $$ } from '../dom'

// The following custom elements are just 24x24 svg icons.
//
//   <icon-menu-sk>
//   <icon-link-sk>
//   <icon-check-sk>
//   <icon-warning-sk>
//   <icon-create-sk>
//   <icon-alarm-sk>
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
const iconSkTemplate = document.createElement('template');
iconSkTemplate.innerHTML = `<svg class="icon-sk-svg" viewBox="0 0 24 24" preserveAspectRatio="xMidYMid meet" focusable="false">
  <g><path d=""></path></g>
</svg>`;

class IconSk extends HTMLElement {
  connectedCallback() {
    let icon = iconSkTemplate.content.cloneNode(true);
    $$('path', icon).setAttribute('d', this.constructor._path);
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

window.customElements.define('icon-check-sk', class extends IconSk {
  static get _path() { return "M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z"; }
});

window.customElements.define('icon-warning-sk', class extends IconSk {
  static get _path() { return "M1 21h22L12 2 1 21zm12-3h-2v-2h2v2zm0-4h-2v-4h2v4z"; }
});

window.customElements.define('icon-create-sk', class extends IconSk {
  static get _path() { return "M3 17.25V21h3.75L17.81 9.94l-3.75-3.75L3 17.25zM20.71 7.04c.39-.39.39-1.02 0-1.41l-2.34-2.34c-.39-.39-1.02-.39-1.41 0l-1.83 1.83 3.75 3.75 1.83-1.83z"; }
});

window.customElements.define('icon-alarm-sk', class extends IconSk {
  static get _path() { return "M22 5.72l-4.6-3.86-1.29 1.53 4.6 3.86L22 5.72zM7.88 3.39L6.6 1.86 2 5.71l1.29 1.53 4.59-3.85zM12.5 8H11v6l4.75 2.85.75-1.23-4-2.37V8zM12 4c-4.97 0-9 4.03-9 9s4.02 9 9 9c4.97 0 9-4.03 9-9s-4.03-9-9-9zm0 16c-3.87 0-7-3.13-7-7s3.13-7 7-7 7 3.13 7 7-3.13 7-7 7z"; }
});
