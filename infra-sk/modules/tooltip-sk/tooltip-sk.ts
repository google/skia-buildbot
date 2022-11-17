/**
 * @module modules/tooltip-sk
 * @description <h2><code>tooltip-sk</code></h2>
 *
 *  Known limitations: You can only have one tooltip for an element. While multiple tooltips
 *  will work, the aria-describedby attribute will only be correct for one of the tooltips.
 *
 *  The displayed tooltip can be styled by targeting `tooltip-sk .content`.
 *
 * @attr target - The id of the element that this tooltip is for.
 *
 * @attr value - The text to display.
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../ElementSk';

export const targetAriaAttribute = 'aria-describedby';

export const hiddenClassName = 'hidden';

export class TooltipSk extends ElementSk {
  private hide = () => this.classList.add(hiddenClassName);

  private show = () => this.classList.remove(hiddenClassName);

  /** The element this tooltip is for. */
  private targetElement: HTMLElement | null = null;

  private static template = (ele: TooltipSk) => html`<div class=content>${ele.value}</div>`;

  constructor() {
    super(TooltipSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.hide();

    if (!this.hasAttribute('role')) {
      this.setAttribute('role', 'tooltip');
    }

    if (!this.hasAttribute('tabindex')) {
      this.setAttribute('tabindex', '-1');
    }

    // This element sets the 'aria-describedby' attribute on the target to point
    // back to this element. We require an id for this to work, so assign a
    // random id if one hasn't been set.
    if (!this.id) {
      this.id = `x${Math.random()}`;
    }

    this.connectToTarget();
  }

  disconnectedCallback(): void {
    this.disconnectTarget();
  }

  private connectToTarget() {
    this.targetElement = document.querySelector(`#${this.target}`);
    if (!this.targetElement) {
      return;
    }
    this.targetElement.setAttribute(targetAriaAttribute, this.id);
    this.targetElement.addEventListener('focus', this.show);
    this.targetElement.addEventListener('blur', this.hide);
    this.targetElement.addEventListener('mouseenter', this.show);
    this.targetElement.addEventListener('mouseleave', this.hide);
  }

  private disconnectTarget() {
    if (!this.targetElement) {
      return;
    }

    this.targetElement.removeAttribute(targetAriaAttribute);
    this.targetElement.removeEventListener('focus', this.show);
    this.targetElement.removeEventListener('blur', this.hide);
    this.targetElement.removeEventListener('mouseenter', this.show);
    this.targetElement.removeEventListener('mouseleave', this.hide);
    this.targetElement = null;
  }

  static get observedAttributes(): string[] {
    return ['target', 'value'];
  }

  /** @prop target {string} The target this tooltip is for. */
  get target(): string { return this.getAttribute('target') || ''; }

  set target(val: string) { this.setAttribute('target', val); }

  /** @prop value {string} The value to display in the tooltip. */
  get value(): string { return this.getAttribute('value') || ''; }

  set value(val: string) { this.setAttribute('value', val); }

  attributeChangedCallback(name: string): void {
    switch (name) {
      case 'target':
        this.connectToTarget();
        break;
      case 'value':
        this._render();
        break;
      default:
        break;
    }
  }
}

define('tooltip-sk', TooltipSk);
