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
 * @evt font-change - This event is generated when the font changes.
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
import '../../skottie-font-selector-sk';
import '../../skottie-text-sampler-sk';
import '../../skottie-button-sk';
import {
  FontType,
  SkottieFontEventDetail,
} from '../../skottie-font-selector-sk/skottie-font-selector-sk';
import { SkTextSampleEventDetail } from '../../skottie-text-sampler-sk/skottie-text-sampler-sk';
import { SkottiePlayerSk } from '../../skottie-player-sk/skottie-player-sk';
import textManagerFactory from '../../helpers/textManagerFactory';

export interface SkottieFontChangeEventDetail {
  font: FontType;
  fontName: string;
}

interface BoundingRect {
  width: number;
  height: number;
  top: number;
  left: number;
}

const filteredKeys = ['ArrowUp', 'ArrowDown', 'Shift', 'Escape', 'Meta'];

export class SkottieTextEditorBoxSk extends ElementSk {
  private static template = (ele: SkottieTextEditorBoxSk) => html`
    <li class="wrapper">
      <details class="expando" ?open=${ele._isOpen} @toggle=${ele.toggle}>
        <summary>
          <span class="summary-label">
            Text layer: ${ele._textData?.text || ''}
          </span>
          <expand-less-icon-sk></expand-less-icon-sk>
          <expand-more-icon-sk></expand-more-icon-sk>
        </summary>

        <div class="text-element">
          ${ele.textElementTitle()} ${ele.buildEditors()} ${ele.inlineEditor()}
        </div>
      </details>
    </li>
  `;

  private _textData: TextData | null = null;

  private mode: ViewMode = 'default';

  private _isOpen: boolean = false;

  private _isWysiwygActive: boolean = false;

  private _timeoutId: number = -1;

  private _skottiePlayer: SkottiePlayerSk | null = null;

  private _canActivateOnDoubleClick: boolean = false;

  private _boundingRect: BoundingRect = {
    width: 0,
    height: 0,
    top: 0,
    left: 0,
  };

  constructor() {
    super(SkottieTextEditorBoxSk.template);
    this.handleKeydown = this.handleKeydown.bind(this);
    this.handlePaste = this.handlePaste.bind(this);
    this.handleCanvasMouseDown = this.handleCanvasMouseDown.bind(this);
    this.handleCanvasMouseMove = this.handleCanvasMouseMove.bind(this);
    this.handleCanvasMouseUp = this.handleCanvasMouseUp.bind(this);
    this.handleCanvasDoubleClick = this.handleCanvasDoubleClick.bind(this);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  private onChange(ev: Event): void {
    const target = ev.target as HTMLTextAreaElement;
    const text = sanitizeText(target.value);
    this.updateText(text);
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
      // Not dispatching change event if wysiwyg is active to avoid losing focus
      if (this._isWysiwygActive) {
        return;
      }
      this.dispatchEvent(
        new CustomEvent('text-data-change', {
          detail: {},
        })
      );
    }, timeout);
  }

  private toggle(): void {
    this._isOpen = !this._isOpen;
    this._render();
  }

  insertText(key: string): void {
    const animation = this._skottiePlayer?.skottieAnimation();
    if (animation && this._textData) {
      animation.dispatchEditorKey(key);
      const textProps = animation.getTextProps();
      textProps.forEach((textProp) => {
        if (textProp.key === this._textData?.name) {
          const text = sanitizeText(textProp.value.text);
          this.updateText(text);
        }
      });
    }
  }

  handleKeydown(ev: KeyboardEvent): void {
    if (ev.key === 'Escape') {
      this.closeWysiwyg();
      return;
    }
    if (filteredKeys.includes(ev.key) || ev.metaKey) {
      return;
    }
    this.insertText(ev.key);
  }

  handlePaste(ev: ClipboardEvent): void {
    if (ev.type === 'paste') {
      const clipboardData = ev.clipboardData;
      const data = clipboardData?.getData('text/plain') || '';
      const textManager = textManagerFactory(data);
      for (const char of textManager) {
        this.insertText(char);
      }
    }
  }

  calculateXCoord(ev: MouseEvent): number {
    if (this._skottiePlayer) {
      const scale =
        this._boundingRect.width /
        (this._skottiePlayer.canvasWidth() * window.devicePixelRatio);
      return (ev.clientX - this._boundingRect.left) / scale;
    }
    return 0;
  }

  calculateYCoord(ev: MouseEvent): number {
    if (this._skottiePlayer) {
      const scale =
        this._boundingRect.height /
        (this._skottiePlayer.canvasHeight() * window.devicePixelRatio);
      return (ev.clientY - this._boundingRect.top) / scale;
    }
    return 0;
  }

  handleCanvasMouseDown(ev: MouseEvent): void {
    const canvasEle = this._skottiePlayer?.canvas();
    const animation = this._skottiePlayer?.skottieAnimation();
    const kit = this._skottiePlayer?.canvasKit();
    if (canvasEle && animation && kit) {
      const canvasBoundingRect = canvasEle.getBoundingClientRect();
      this._boundingRect.top = canvasBoundingRect.top;
      this._boundingRect.left = canvasBoundingRect.left;
      this._boundingRect.width = canvasBoundingRect.width;
      this._boundingRect.height = canvasBoundingRect.height;
      animation.dispatchEditorPointer(
        this.calculateXCoord(ev),
        this.calculateYCoord(ev),
        kit.InputState.Down,
        kit.ModifierKey.None
      );
    }
  }

  handleCanvasMouseMove(ev: MouseEvent): void {
    const animation = this._skottiePlayer?.skottieAnimation();
    const kit = this._skottiePlayer?.canvasKit();
    if (animation && kit) {
      animation.dispatchEditorPointer(
        this.calculateXCoord(ev),
        this.calculateYCoord(ev),
        kit.InputState.Move,
        kit.ModifierKey.None
      );
    }
  }

  handleCanvasDoubleClick(): void {
    if (this._canActivateOnDoubleClick && !this._isWysiwygActive) {
      this.openWysiwyg();
    }
  }

  handleCanvasMouseUp(ev: MouseEvent): void {
    const animation = this._skottiePlayer?.skottieAnimation();
    const kit = this._skottiePlayer?.canvasKit();
    if (animation && kit) {
      animation.dispatchEditorPointer(
        this.calculateXCoord(ev),
        this.calculateYCoord(ev),
        kit.InputState.Up,
        kit.ModifierKey.None
      );
    }
  }

  clearListeners(): void {
    const animation = this._skottiePlayer?.skottieAnimation();
    const canvasEle = this._skottiePlayer?.canvas();
    if (animation && canvasEle) {
      animation.enableEditor(false);
      document.removeEventListener('keydown', this.handleKeydown);
      canvasEle.removeEventListener('mousedown', this.handleCanvasMouseDown);
      canvasEle.removeEventListener('mousemove', this.handleCanvasMouseMove);
      canvasEle.removeEventListener('mouseup', this.handleCanvasMouseUp);
    }
  }

  closeWysiwyg(): void {
    this._isWysiwygActive = false;
    this.clearListeners();
    this.scheduleChangeEvent(0);
    this._render();
  }

  openWysiwyg(): void {
    this._isWysiwygActive = true;
    this.clearListeners();
    const animation = this._skottiePlayer?.skottieAnimation();
    const canvasEle = this._skottiePlayer?.canvas();
    if (animation && canvasEle) {
      animation.attachEditor(this._textData?.name || '', 0);
      animation.enableEditor(true);
      document.addEventListener('keydown', this.handleKeydown);
      document.addEventListener('paste', this.handlePaste);
      canvasEle.addEventListener('mousedown', this.handleCanvasMouseDown);
      canvasEle.addEventListener('mousemove', this.handleCanvasMouseMove);
      canvasEle.addEventListener('mouseup', this.handleCanvasMouseUp);
      if (document.activeElement instanceof HTMLElement) {
        document.activeElement.blur();
      }
      canvasEle.focus();
    }
    this._render();
  }

  private toggleWysiwyg(): void {
    if (this._skottiePlayer && this._skottiePlayer.skottieAnimation()) {
      if (!this._isWysiwygActive) {
        this.openWysiwyg();
      } else {
        this.closeWysiwyg();
      }
    }
  }

  private buildEditors(): TemplateResult | null {
    if (!this._isWysiwygActive) {
      return html`<textarea
          class="text-element-input"
          @change=${this.onChange}
          @input=${this.onChange}
          @blur=${this.onBlur}
          maxlength=${ifDefined(this._textData?.maxChars)}
          .value=${this._textData?.text || ''}></textarea>
        <div>${this.originTemplate()}</div>
        <div>${this.fontSelector()}</div>
        <div>${this.fontSettings()}</div>
        <div>${this.fontTextSampler()}</div> `;
    }
    return null;
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
      <div class="text-element-section">
        <div class="text-element-section--title">
          Origin${items.length > 1 ? 's' : ''}:
        </div>
        <ul>
          ${items.map(SkottieTextEditorBoxSk.originTemplateElement)}
        </ul>
      </div>
    `;
  }

  private fontSelector(): TemplateResult | null {
    if (this.mode === 'presentation' || !this._textData) {
      return null;
    }
    const { items, fontName } = this._textData;
    return html`
      <section class="text-element-section">
        <div class="text-element-section--title">Font manager</div>
        <skottie-font-selector-sk
          .fontName=${fontName}
          @select-font=${this.onFontSelected}>
        </skottie-font-selector-sk>
      </section>
    `;
  }

  private fontSettings(): TemplateResult | null {
    if (this.mode === 'presentation' || !this._textData) {
      return null;
    }
    return html`
      <section class="text-element-section">
        <div class="text-element-section--title">Font settings</div>
        <div class="text-box text-box__left">
          <div class="text-box--label">
            <span class="icon-sk text-box--label--icon">
              format_line_spacing
            </span>
          </div>
          <input
            type="number"
            class="text-box--input"
            @change=${this.onLineHeightChange}
            value=${this._textData.lineHeight}
            required />
        </div>
        <div class="text-box text-box__left">
          <div class="text-box--label">
            <span class="icon-sk text-box--label--icon">
              format_letter_spacing_wide
            </span>
          </div>
          <input
            type="number"
            class="text-box--input"
            @change=${this.onTrackingChange}
            value=${this._textData.tracking}
            required />
        </div>
      </section>
    `;
  }

  private fontTextSampler(): TemplateResult | null {
    if (this.mode === 'presentation' || !this._textData) {
      return null;
    }
    const { fontName } = this._textData;
    return html`
      <section class="text-element-section">
        <div class="text-element-section--title">Text samples</div>
        <skottie-text-sampler-sk
          .fontName=${fontName}
          @select-text=${this.onTextSelected}>
        </skottie-text-sampler-sk>
      </section>
    `;
  }

  private inlineEditor(): TemplateResult | null {
    if (this.mode === 'presentation' || !this._textData) {
      return null;
    }
    const { fontName } = this._textData;
    return html`
      <section class="text-element-section">
        <div class="text-element-section--title">Inline editing</div>
        <skottie-button-sk
          id="rewind"
          .content=${html`${this._isWysiwygActive
            ? 'Close inline editing'
            : 'Open inline editing'}`}
          .classes=${['playback-content__button']}
          @select=${this.toggleWysiwyg}></skottie-button-sk>
      </section>
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

  protected _render(): void {
    super._render();
    const canvasEle = this._skottiePlayer?.canvas();
    if (canvasEle) {
      canvasEle.addEventListener('dblclick', this.handleCanvasDoubleClick);
    }
  }

  disconnectedCallback(): void {
    const canvasEle = this._skottiePlayer?.canvas();
    if (canvasEle) {
      canvasEle.removeEventListener('dblclick', this.handleCanvasDoubleClick);
    }
    this.clearListeners();
    super.disconnectedCallback();
  }

  private onFontSelected(ev: CustomEvent<SkottieFontEventDetail>): void {
    if (this._textData) {
      this.dispatchEvent(
        new CustomEvent<SkottieFontChangeEventDetail>('font-change', {
          detail: {
            font: ev.detail.font,
            fontName: this._textData?.fontName || '',
          },
        })
      );
      this._textData.fontName = ev.detail.font.fName;
    }
  }

  private onTextSelected(ev: CustomEvent<SkTextSampleEventDetail>): void {
    this.updateText(ev.detail.text);
  }

  private onTrackingChange(ev: Event) {
    const target = ev.target as HTMLInputElement;
    if (this._textData) {
      this._textData.tracking = target.valueAsNumber;
      this._textData.items.forEach((item: ExtraLayerData) => {
        // this property is the text tracking of a text layer.
        // It's read as: Text Element > Text document > First Keyframe > Start Value > Tracking
        if (item.layer.t) {
          item.layer.t.d.k[0].s.tr = target.valueAsNumber;
        }
      });
      this.scheduleChangeEvent(0);
    }
  }

  private onLineHeightChange(ev: Event) {
    const target = ev.target as HTMLInputElement;
    if (this._textData) {
      this._textData.lineHeight = target.valueAsNumber;
      this._textData.items.forEach((item: ExtraLayerData) => {
        // this property is the text line height of a text layer.
        // It's read as: Text Element > Text document > First Keyframe > Start Value > Line height
        if (item.layer.t) {
          item.layer.t.d.k[0].s.lh = target.valueAsNumber;
        }
      });
      this.scheduleChangeEvent(0);
    }
  }

  private updateText(value: string): void {
    if (this._textData) {
      this._textData.text = value;
      this._textData.items.forEach((item: ExtraLayerData) => {
        // this property is the text string of a text layer.
        // It's read as: Text Element > Text document > First Keyframe > Start Value > Text
        if (item.layer.t) {
          item.layer.t.d.k[0].s.t = value;
        }
      });
      this.scheduleChangeEvent(0);
    }
  }

  set textData(val: TextData) {
    this._textData = val;
    this._render();
  }

  set skottiePlayer(val: SkottiePlayerSk) {
    this._skottiePlayer = val;
  }

  set canActivateOnDoubleClick(val: boolean) {
    this._canActivateOnDoubleClick = val;
  }
}

define('skottie-text-editor-box-sk', SkottieTextEditorBoxSk);
