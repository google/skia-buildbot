/**
 * @module infra-sk/modules/ElementSk
 * @description <h2><code>ElementSk</code></h2>
 *
 */
import { render } from 'lit-html'
import { upgradeProperty } from 'elements-sk/upgradeProperty'

/**
 * A base class that records the connected status of the element in
 * this._connected and provides a _render() function that doesn't do anything
 * this this._connected is false.
 *
 * @property {Boolean} _connected - True if the connectedCallback has been
 *   called.
 *
 * @example
 *
 * class MyElement extends ElementSk {
 *   constructor() {
 *     super();
 *     this._template = (ele) => html`<p>Hello World!</p>`;
 *   }
 *
 *   connectedCallback() {
 *     super.connectedCallback();
 *     this._render();
 *   }
 * }
 *
 */
export class ElementSk extends HTMLElement {
  /**
   * @param template A lit-html template to be used in _render().
   */
  constructor(template = null) {
    super();
    this._template = template;
    this._connected = false;
  }

  connectedCallback() {
    this._connected = true;
  }

  disconnectedCallback() {
    this._connected = false;
  }

  /**
   * Capture the value from the unupgraded instance and delete the property so
   * it does not shadow the custom element's own property setter.
   *
   * See this [Google Developers article]{@link
   *    https://developers.google.com/web/fundamentals/web-components/best-practices#lazy-properties
   *    } for more details.
   *
   * @param name {string} Property name.
   * @protected
   */
  _upgradeProperty(name) {
    upgradeProperty(this, name);
  }

  /**
   * Renders the lit-html template found at this._template if not-null, but
   * only if connectedCallback has been called.
   *
   * @protected
   */
  _render() {
    if (this._connected && !!this._template) {
      render(this._template(this), this, {eventContext: this});
    }
  }
};
