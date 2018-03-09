/** @module skia-elements/upgradeProperty */

/**
 * Capture the value from the unupgraded instance and delete the property so
 * it does not shadow the custom element's own property setter.
 *
 * See this [Google Developers article]{@link https://developers.google.com/web/fundamentals/web-components/best-practices#lazy-properties } for more details.
 *
 * @param {Element} ele -The element.
 * @param {string} prop - The name of the property to upgrade.
 */
export function upgradeProperty(ele, prop) {
  if (ele.hasOwnProperty(prop)) {
    let value = ele[prop];
    delete ele[prop];
    ele[prop] = value;
  }
}
