// Note that we are importing just the class, not the whole element + CSS definition.
import { CheckboxElement } from '../checkbox-sk/checkbox-sk.js';

class RadioElement extends CheckboxElement {
  static get _role() { return 'radio'; }
}

window.customElements.define('radio-sk', RadioElement);
