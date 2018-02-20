function upgradeProperty(ele, prop) {
  if (ele.hasOwnProperty(prop)) {
    let value = ele[prop];
    delete ele[prop];
    ele[prop] = value;
  }
}

window.customElements.define('example-element', class extends HTMLElement {
  constructor() {
    super();
  }

  connectedCallback() {
    upgradeProperty(this, 'active');
  }

  get active() { return this.hasAttribute('active'); }
  set active(val) {
    if (val) {
      this.setAttribute('active', '');
    } else {
      this.removeAttribute('active');
    }
  }
});
