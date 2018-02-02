import { upgradeProperty } from './upgrade-property.js'

export class CheckboxElement extends HTMLElement {
  static get _role() { return 'checkbox'; }

  static get observedAttributes() {
    return ['checked', 'disabled'];
  }

  connectedCallback() {
    upgradeProperty(this, 'checked');
    upgradeProperty(this, 'disabled');
    upgradeProperty(this, 'hidden');
    if (!this.hasAttribute('role')) {
      this.setAttribute('role', this.constructor._role);
    }
    if (!this.hasAttribute('tabindex')) {
      this.setAttribute('tabindex', '0');
    }
    this.setAttribute('aria-checked', this.checked);
    this.setAttribute('aria-disabled', this.disabled);
    this.addEventListener('click', this);
    this.addEventListener('keydown', this);
  }

  disconnectedCallback() {
    this.removeEventListener('click', this);
    this.removeEventListener('keydown', this);
  }

  get checked() { return this.hasAttribute('checked'); }
  set checked(val) {
    if (val) {
      this.setAttribute('checked', '');
      this.setAttribute('aria-checked', 'true');
    } else {
      this.removeAttribute('checked');
      this.setAttribute('aria-checked', 'false');
    }
  }

  get disabled() { return this.hasAttribute('disabled'); }
  set disabled(val) {
    if (val) {
      this.setAttribute('disabled', '');
      this.setAttribute('aria-disabled', 'true');
    } else {
      this.removeAttribute('disabled');
      this.setAttribute('aria-disabled', 'false');
    }
  }

  get hidden() { return this.hasAttribute('hidden'); }
  set hidden(val) {
    if (val) {
      this.setAttribute('hidden', '');
    } else {
      this.removeAttribute('hidden');
    }
  }

  handleEvent(e) {
    if (e.type === 'click' && !this.disabled) {
      this.checked = !this.checked;
      this.dispatchEvent(new CustomEvent('change-sk', { 'bubbles': true }));
    } else if (e.type === 'keydown') {
      if (event.altKey) {
        return;
      }
      if (event.keyCode === 32 /* Space */) {
        e.preventDefault();
        if (!this.disabled) {
          this.checked = !this.checked;
          this.dispatchEvent(new CustomEvent('change-sk', { 'bubbles': true }));
        }
      }
    }
  }

  attributeChangedCallback(name, oldValue, newValue) {
    let isTrue = !!newValue;
    switch (name) {
      case 'checked':
        this.setAttribute('aria-checked', isTrue);
        break;
      case 'disabled':
        this.setAttribute('aria-disabled', isTrue);
        break;
    }
  }
}

window.customElements.define('checkbox-sk', CheckboxElement);

