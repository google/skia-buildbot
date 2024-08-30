/**
 * @module skottie-button-sk
 * @description <h2><code>skottie-button-sk</code></h2>
 *
 * <p>
 *   A skottie button.
 * </p>
 *
 *
 * @attr content - A string, with no markup, that is to be used as the text for
 *            the text.
 *
 * @prop content This mirrors the text attribute.
 *
 * @attr type - A string of type ButtonType that styles the button.
 *
 * @prop type This mirrors the type attribute.
 *
 * @attr classes - An array of classes to add to the button element.
 *
 * @prop classes This mirrors the classes attribute.
 *
 * @attr disabled - A boolean to set the button as disabled.
 *
 * @prop disabled This mirrors the disabled attribute.
 *
 */
import { html, TemplateResult } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

type ButtonType = 'filled' | 'outline' | 'plain';

type ContentType = TemplateResult | string;

const classesMap: Record<ButtonType, string> = {
  filled: 'base__filled',
  outline: 'base__outline',
  plain: 'base__plain',
};

export class SkottieButtonSk extends ElementSk {
  private _type: ButtonType = 'filled';

  private _isDisabled: boolean = true;

  private _content: ContentType = '';

  protected _classes: string[] = [];

  private _id: string = '';

  private static template = (ele: SkottieButtonSk) => html`
    <button
      class=${ele.buildButtonClass()}
      id=${ele._id}
      ?disabled=${ele._isDisabled}
      @click=${ele.onClick}>
      ${ele._content}
    </button>
  `;

  constructor() {
    super(SkottieButtonSk.template);
  }

  set content(val: ContentType) {
    this._content = val;
    this._render();
  }

  buildButtonClass(): string {
    const classes = ['base', classesMap[this._type]].concat(this._classes);

    return classes.join(' ');
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._type = (this.getAttribute('type') as ButtonType) || 'filled';
    this._isDisabled = this.hasAttribute('disabled');
    if (this.getAttribute('id')) {
      this._id = this.getAttribute('id') || '';
      this.setAttribute('id', '');
    }
    this._render();
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
  }

  onClick(ev: Event): void {
    ev.preventDefault();
    this.dispatchEvent(
      new CustomEvent('select', {
        bubbles: true,
      })
    );
  }

  set type(value: ButtonType) {
    this._type = value;
    this._render();
  }

  set classes(val: string[]) {
    this._classes = val;
    this._render();
  }
}

define('skottie-button-sk', SkottieButtonSk);
