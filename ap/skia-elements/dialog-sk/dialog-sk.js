window.customElements.define('dialog-sk', class extends HTMLElement {
  show() {
    this.setAttribute('shown', '');
  }

  hide() {
    this.removeAttribute('shown');
  }
});
