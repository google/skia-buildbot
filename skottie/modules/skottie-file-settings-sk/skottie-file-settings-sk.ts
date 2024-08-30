/**
 * @module skottie-file-settings-sk
 * @description <h2><code>skottie-file-settings-sk</code></h2>
 *
 * <p>
 *   A component to edit skottie's width, height and fps inline
 * </p>
 *
 *
 * @evt settings-change - This event is generated when the user presses Apply.
 *         The updated fps, width, and height is available in the event detail.
 *
 */
import { html, TemplateResult } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../../../elements-sk/modules/icons/lock-icon-sk';
import '../../../elements-sk/modules/icons/lock-open-icon-sk';
import '../skottie-button-sk';
import { $$ } from '../../../infra-sk/modules/dom';

export interface SkottieFileSettingsEventDetail {
  width: number;
  height: number;
  fps: number;
}
const DEFAULT_SIZE = 128;

export class SkottieFileSettingsSk extends ElementSk {
  private _isRatioLocked: boolean = false;

  private _width: number = DEFAULT_SIZE;

  private _height: number = DEFAULT_SIZE;

  private _fps: number = 0;

  private _ratio: number = DEFAULT_SIZE / DEFAULT_SIZE;

  private static template = (ele: SkottieFileSettingsSk) => html`
    <div class="wrapper">
      <div id="wrapper-form">
        <div class="text-box text-box__left">
          <div class="text-box--label">W</div>
          <input
            type="number"
            class="text-box--input"
            id="file-settings-width"
            @change=${ele.onWidthChange}
            value=${ele._width}
            required />
        </div>
        <div class="text-box text-box__right">
          <div class="text-box--label">H</div>
          <input
            type="number"
            class="text-box--input"
            id="file-settings-height"
            @change=${ele.onHeightChange}
            value=${ele._height}
            required />
        </div>
        <skottie-button-sk
          class="aspect-ratio"
          type="outline"
          @select=${ele.toggleAspectRatio}
          .content=${ele.renderAspectRatioButton()}>
        </skottie-button-sk>
        <div class="text-box">
          <div class="text-box--label">FPS</div>
          <input
            type="number"
            class="text-box--input"
            id="file-settings-fps"
            @change=${ele.onFrameChange}
            value=${ele._fps}
            required />
        </div>
        <div class="text-box--info">
          <info-icon-sk></info-icon-sk>
          <span class="text-box--info--tooltip">
            Frame rate "0" means "as smooth as possible" and -1 is to use the
            default from the animation
          </span>
        </div>
      </div>
      <div class="toolbar">
        <skottie-button-sk
          type="filled"
          @select=${ele.applySettings}
          .content=${'Apply'}>
        </skottie-button-sk>
      </div>
    </div>
  `;

  constructor() {
    super(SkottieFileSettingsSk.template);
    this.toggleAspectRatio = this.toggleAspectRatio.bind(this);
    this.applySettings = this.applySettings.bind(this);
  }

  renderAspectRatioButton(): TemplateResult {
    const iconText = this._isRatioLocked ? 'link' : 'link_off';
    return html` <span class="icon-sk">${iconText}</span> `;
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._ratio = this.width / this.height;
    this._render();
    this.addEventListener('input', this.inputEvent);
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    this.removeEventListener('input', this.inputEvent);
  }

  get height(): number {
    return this._height;
  }

  set height(val: number) {
    this._height = val;
    this._render();
  }

  get fps(): number {
    return this._fps;
  }

  set fps(val: number) {
    this._fps = +val;
    this._render();
  }

  get width(): number {
    return this._width;
  }

  set width(val: number) {
    this._width = +val;
    this._render();
  }

  _render() {
    super._render();
    const widthElement = $$<HTMLInputElement>('#file-settings-width', this);
    if (widthElement) {
      widthElement.value = this.width.toString();
    }
    const heightElement = $$<HTMLInputElement>('#file-settings-height', this);
    if (heightElement) {
      heightElement.value = this.height.toString();
    }
  }

  private toggleAspectRatio(): void {
    this._isRatioLocked = !this._isRatioLocked;
    this._render();
  }

  private applySettings(): void {
    this.dispatchEvent(
      new CustomEvent<SkottieFileSettingsEventDetail>('settings-change', {
        detail: {
          width: this._width,
          height: this._height,
          fps: this._fps,
        },
        bubbles: true,
      })
    );
  }

  private updateState(): void {
    this._width = +$$<HTMLInputElement>('#file-settings-width', this)!.value;
    this._height = +$$<HTMLInputElement>('#file-settings-height', this)!.value;
    this._fps = +$$<HTMLInputElement>('#file-settings-fps', this)!.value;
  }

  private onFrameChange(e: Event): void {
    const target = e.target as HTMLInputElement;
    this.fps = +target.value;
  }

  private onWidthChange(e: Event): void {
    if (this._isRatioLocked) {
      this._height = Math.floor(this._width / this._ratio);
      this._render();
    }
  }

  private onHeightChange(e: Event): void {
    if (this._isRatioLocked) {
      this._width = Math.floor(this._height * this._ratio);
      this._render();
    }
  }

  private inputEvent(): void {
    this.updateState();
    this._render();
  }
}

define('skottie-file-settings-sk', SkottieFileSettingsSk);
