import { upgradeProperty } from '../upgradeProperty'

// This is the implementation for both the checkbox-sk and radio-sk custom
// element.
//
// The checkbox-sk and radio-sk elements each contains a native 'input'
// element in light DOM so that they can participate in a form element.
//
//  Attributes:
//    label - A string, with no markup, that is to be used as the label for
//            the checkbox. If you wish to have a label with markup then set
//            'label' to the empty string and create your own <label></label>
//            element in the DOM with the 'for' attribute set to match the
//            name of the checkbox-sk or radio-sk.
//
//    Each element also supports the following attributes exactly as the
//    native checkbox element:
//     - checked
//     - disabled
//     - name
//
//  Properties:
//    All the above attributes are mirrored to the property of the same name.
//
//  Events:
//    All the normal events of a native checkbox or radio button.
//
//  Methods:
//    None
//
export class CheckOrRadio extends HTMLElement {
  get _role() { return 'checkbox'; }

  static get observedAttributes() {
    return ['checked', 'disabled', 'name', 'label'];
  }

  connectedCallback() {
    this.innerHTML = `<label><input type=${this._role}></input><span class=box></span><span class=label></span></label>`;
    this._label = this.querySelector('.label');
    this._input = this.querySelector('input');
    upgradeProperty(this, 'checked');
    upgradeProperty(this, 'disabled');
    upgradeProperty(this, 'name');
    upgradeProperty(this, 'label');
    this._input.checked = this.checked;
    this._input.disabled = this.disabled;
    this._input.setAttribute('name', this.getAttribute('name'));
    this._label.textContent = this.getAttribute('label');
    // TODO(jcgregorio) Do we capture and alter the 'input' and 'change' events generated
    // by the input element so that the evt.target points to 'this'?
  }

  get checked() { return this.hasAttribute('checked'); }
  set checked(val) {
    let isTrue = !!val;
    this._input.checked = isTrue;
    if (val) {
      this.setAttribute('checked', '');
    } else {
      this.removeAttribute('checked');
    }
  }

  get disabled() { return this.hasAttribute('disabled'); }
  set disabled(val) {
    let isTrue = !!val;
    this._input.disabled = isTrue;
    if (isTrue) {
      this.setAttribute('disabled', '');
    } else {
      this.removeAttribute('disabled');
    }
  }

  get name() { return this._input.getAttribute('name'); }
  set name(val) {
    this.setAttribute('name', val);
    this._input.setAttribute('name', val);
  }

  get label() { return this._input.getAttribute('label'); }
  set label(val) {
    this.setAttribute('label', val);
    this._input.setAttribute('label', val);
  }

  attributeChangedCallback(name, oldValue, newValue) {
    console.log(name, oldValue, newValue);
    if (!this._input) {
      return
    }
    // Strictly check for null since an empty string doesn't mean false
    // for a boolean attribute.
    let isTrue = newValue != null;
    switch (name) {
      case 'checked':
        this._input.checked = isTrue;
        break;
      case 'disabled':
        this._input.disabled = isTrue;
        break;
      case 'name':
        this._input.name = newValue;
        break;
      case 'label':
        this._label.textContent = newValue;
        break;
    }
  }
}

window.customElements.define('checkbox-sk', CheckOrRadio);
