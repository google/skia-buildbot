// This is a generated file!

import { define } from '../../define';

const iconSkTemplate = document.createElement('template');
iconSkTemplate.innerHTML = '<svg class="icon-sk-svg" xmlns="http://www.w3.org/2000/svg" width=24 height=24 viewBox="0 0 24 24"><path d="M6.99 11L3 15l3.99 4v-3H14v-2H6.99v-3zM21 9l-3.99-4v3H10v2h7.01v3L21 9z"/></svg>';

define('swap-horiz-icon-sk', class extends HTMLElement {
  connectedCallback() {
    const icon = iconSkTemplate.content.cloneNode(true);
    while (this.firstChild) {
      this.removeChild(this.firstChild);
    }
    this.appendChild(icon);
  }
});
