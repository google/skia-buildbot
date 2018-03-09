/** @module skia-elements/upgradeProperty */

/**
 * Capture the value from the unupgraded instance and delete the property so
 * it does not shadow the custom element's own property setter.
 *
 * See this [Google Developers article]{@link https://developers.google.com/web/fundamentals/web-components/best-practices#lazy-properties } for more details.
 *
 * @param {Element} ele -The element.
 * @param {string} prop - The name of the property to upgrade.
 *
 * @example
 *
 * // Upgrade the 'duration' property if it was already set.
 * window.customElements.define('my-element', class extends HTMLElement {
 *   connectedCallback() {
 *     upgradeProperty(this, 'duration');
 *   }
 *
 *   get duration() { return +this.getAttribute('duration'); }
 *   set duration(val) { this.setAttribute('duration', val); }
 * });
 *
 */
export function upgradeProperty(ele, prop) {
  if (ele.hasOwnProperty(prop)) {
    let value = ele[prop];
    delete ele[prop];
    ele[prop] = value;
  }
}
