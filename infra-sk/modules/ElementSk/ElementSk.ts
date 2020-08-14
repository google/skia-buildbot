/**
 * @module infra-sk/modules/ElementSk
 * @description <h2><code>ElementSk</code></h2>
 *
 */
import { render, TemplateResult } from 'lit-html'
import { upgradeProperty } from 'elements-sk/upgradeProperty'

/**
 * A base class that records the connected status of the element in this._connected and provides a
 * _render() function that doesn't do anything if this this._connected is false.
 *
 * @example
 *
 * class MyElement extends ElementSk {
 *   greeting = "Hello";
 *
 *   constructor() {
 *     super((ele: MyElement) => html`<p>${ele.greeting} World!</p>`);
 *   }
 *
 *   connectedCallback() {
 *     super.connectedCallback();
 *     this._render();
 *   }
 * }
 */
export class ElementSk extends HTMLElement {
  protected _template: ((el: any) => TemplateResult) | null = null;
  protected _connected: boolean = false;

  /**
   * @param template A function that, when applied to this component, will returns the component's
   *     lit-html template.
   */
  constructor(templateFn?: (el: any) => TemplateResult) {
    super();
    this._template = templateFn || null;
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
   * @param name Property name.
   */
  protected _upgradeProperty(name: string) {
    upgradeProperty(this, name);
  }

  /**
   * Renders the lit-html template found at this._template if not-null, but only if
   * connectedCallback has been called.
   */
  protected _render() {
    if (this._connected && !!this._template) {
      render(this._template(this), this, {eventContext: this});
    }
  }
};
