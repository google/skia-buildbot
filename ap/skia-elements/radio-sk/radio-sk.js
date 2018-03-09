/** @module skia-elements/radio-sk
 *
 * @description See the description for {@linkcode module:skia-elements/checkbox-sk}.
 *
 */

// Note that we are importing just the class, not the whole element + CSS definition.
import { CheckOrRadio } from '../checkbox-sk/checkbox-sk.js';

class RadioElement extends CheckOrRadio {
  get _role() { return 'radio'; }
}

// The radio-sk element contains a native 'input' element in light DOM
// so that the radio button can participate in a form element,
// and also participate in a native 'radiogroup' element.
//
// See documentation for checkbox-sk.
window.customElements.define('radio-sk', RadioElement);
