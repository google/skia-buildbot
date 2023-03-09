// This is a generated file!

import { define } from '../../define';

const iconSkTemplate = document.createElement('template');
iconSkTemplate.innerHTML = '<svg class="icon-sk-svg" xmlns="http://www.w3.org/2000/svg" width=24 height=24 viewBox="0 0 24 24"><path d="M3 13h8V3H3v10zm0 8h8v-6H3v6zm10 0h8V11h-8v10zm0-18v6h8V3h-8z"/></svg>';

define('dashboard-icon-sk', class extends HTMLElement {
  connectedCallback() {
    const icon = iconSkTemplate.content.cloneNode(true);
    while (this.firstChild) {
      this.removeChild(this.firstChild);
    }
    this.appendChild(icon);
  }
});
