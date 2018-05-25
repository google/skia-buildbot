/**
 * @module elements-sk/radio-sk
 * @description <h2><code>radio-sk</code></h2>
 *
 * <p>
 *   The radio-sk and element contains a native 'input'
 *   element in light DOM so that it can participate in a form element.
 * </p>
 *
 * <p>
 *    Each element also supports the following attributes exactly as the
 *    native radio button element:
 *    <ul>
 *      <li>checked</li>
 *      <li>disabled</li>
 *      <li>name</li>
 *     </ul>
 * </p>
 *
 * <p>
 *    All the normal events of a native radio button are supported.
 * </p>
 *
 * @attr label - A string, with no markup, that is to be used as the label for
 *            the radio button. If you wish to have a label with markup then set
 *            'label' to the empty string and create your own
 *            <code>label</code> element in the DOM with the 'for' attribute
 *            set to match the name of the radio-sk.
 *
 * @prop {boolean} checked This mirrors the checked attribute.
 * @prop {boolean} disabled This mirrors the disabled attribute.
 * @prop {string} disabled This mirrors the name attribute.
 * @prop {string} disabled This mirrors the label attribute.
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
