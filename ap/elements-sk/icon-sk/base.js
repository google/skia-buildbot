const iconSkTemplate = document.createElement('template');
iconSkTemplate.innerHTML = `<svg class="icon-sk-svg" viewBox="0 0 24 24" preserveAspectRatio="xMidYMid meet" focusable="false">
  <g><path d=""></path></g>
</svg>`;

export class IconSk extends HTMLElement {
  connectedCallback() {
    let icon = iconSkTemplate.content.cloneNode(true);
    icon.querySelector('path').setAttribute('d', this.constructor._path);
    this.appendChild(icon);
  }
}