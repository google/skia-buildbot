const iconSkTemplate = document.createElement('template');

export class IconSk extends HTMLElement {
  constructor() {
    super()
    iconSkTemplate.innerHTML = `<svg class="icon-sk-svg" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24">${this.constructor._svg}</svg>`;
  }

  connectedCallback() {
    let icon = iconSkTemplate.content.cloneNode(true);
    this.appendChild(icon);
  }
}
