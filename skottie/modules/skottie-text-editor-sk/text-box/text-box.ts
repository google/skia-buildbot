/**
 * @module skottie-text-editor-box-sk
 * @description <h2><code>skottie-text-editor-box-sk</code></h2>
 *
 * <p>
 *   A skottie text editor box
 * </p>
 *
 *
 * @evt change - This event is generated when the text changes.
 *
 * @attr textData the text data.
 *         At the moment it only reads it at load time.
 *
 * @attr mode - the view mode.
 *         Supported values are default and presentation
 *
 */
import { html, TemplateResult } from 'lit-html';
import { define } from '../../../../elements-sk/modules/define';
import { ElementSk } from '../../../../infra-sk/modules/ElementSk';
import { ViewMode } from '../../types';
import { ExtraLayerData, TextData } from '../text-replace';
import sanitizeText from '../text-sanizite';
import { ifDefined } from 'lit-html/directives/if-defined';
import { setTimeout } from 'timers';

export class SkottieTextEditorBoxSk extends ElementSk {
  private static template = (ele: SkottieTextEditorBoxSk) => html`
    <li class="wrapper">
      <details class="expando" ?open=${ele._isOpen} @toggle=${ele.toggle}>
        <summary>
          <span>Text layer: ${ele._textData?.text || ''}</span>
          <expand-less-icon-sk></expand-less-icon-sk>
          <expand-more-icon-sk></expand-more-icon-sk>
        </summary>

        <div class="text-element">
          ${ele.textElementTitle()}
          <textarea
            class="text-element-input"
            @change=${ele.onChange}
            @input=${ele.onChange}
            @blur=${ele.onBlur}
            maxlength=${ifDefined(ele._textData?.maxChars)}
            .value=${ele._textData?.text || ''}></textarea>
          <div>${ele.originTemplate()}</div>
        </div>
      </details>
    </li>
  `;

  private _textData: TextData | null = null;

  private mode: ViewMode = 'default';

  private _isOpen: boolean = false;

  private _timeoutId: number = -1;

  constructor() {
    super(SkottieTextEditorBoxSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  private onChange(ev: Event): void {
    if (this._textData) {
      const target = ev.target as HTMLTextAreaElement;
      const text = sanitizeText(target.value);
      this._textData.text = text;
      this._textData.items.forEach((item: ExtraLayerData) => {
        // this property is the text string of a text layer.
        // It's read as: Text Element > Text document > First Keyframe > Start Value > Text
        if (item.layer.t) {
          item.layer.t.d.k[0].s.t = text;
        }
      });
      this.scheduleChangeEvent();
    }
  }

  private onBlur(): void {
    this.scheduleChangeEvent(0);
  }

  // instead of dispatching a change event on every keystroke,
  // we buffer changes by default for 1 second
  // to avoid performance issues when rebuilding the animation
  private scheduleChangeEvent(timeout: number = 1000): void {
    window.clearTimeout(this._timeoutId);
    this._timeoutId = window.setTimeout(() => {
      this.dispatchEvent(
        new CustomEvent('change', {
          detail: {},
        })
      );
    }, timeout);
  }

  private toggle(): void {
    this._isOpen = !this._isOpen;
    this._render();
  }

  private textElementTitle(): TemplateResult | null {
    if (this.mode === 'presentation' || !this._textData) {
      return null;
    }
    const { name } = this._textData;
    return html`
      <div class="text-element-item">
        <div class="text-element-label">Layer name:</div>
        <div>${name}</div>
      </div>
    `;
  }

  private originTemplate(): TemplateResult | null {
    if (this.mode === 'presentation' || !this._textData) {
      return null;
    }
    const { items } = this._textData;
    return html`
      <div class="text-element-item">
        <div class="text-element-label">
          Origin${items.length > 1 ? 's' : ''}:
        </div>
        <ul>
          ${items.map(SkottieTextEditorBoxSk.originTemplateElement)}
        </ul>
      </div>
    `;
  }

  private static originTemplateElement(
    item: ExtraLayerData
  ): TemplateResult | null {
    return html`
      <li class="text-element-origin">
        <b>${item.precompName}</b> > Layer ${item.layer.ind}
      </li>
    `;
  }

  set textData(val: TextData) {
    this._textData = val;
    this._render();
  }
}

define('skottie-text-editor-box-sk', SkottieTextEditorBoxSk);
