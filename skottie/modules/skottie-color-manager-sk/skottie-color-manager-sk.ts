/**
 * @module skottie-color-manager-sk
 * @description <h2><code>skottie-color-manager-sk</code></h2>
 *
 * <p>
 *   A component to enable changing colors of a lottie animation.
 *   It works by wrapping the animation in a precomp and applying
 *   a color filter to the outer composition.
 *   It uses a json template that works as a wrapper that will contain
 *   the original animation.
 * </p>
 *
 * @evt animation-updated - This event is triggered every time the manager is updated,
 *      either from applying the filter or when its properties change.
 *
 */
import { html, TemplateResult } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { LottieAnimation, LottieCompAsset, LottieTintEffect } from '../types';
import { colorToHex, hexToColor } from '../helpers/color';
import '../skottie-color-input-sk';
import { SkottieColorEventDetail } from '../skottie-color-input-sk/skottie-color-input-sk';

const TEMPLATE_ID = 'template_0';
const TINT_EFFECT_TYPE = 20;

// Tint effect template
const tintEffectTemplate: LottieTintEffect = {
  ty: TINT_EFFECT_TYPE, // Tint effect
  nm: 'Tint',
  np: 6,
  mn: 'ADBE Tint',
  en: 1,
  ef: [
    {
      ty: 2,
      nm: 'Map Black To',
      mn: 'ADBE Tint-0001',
      v: {
        a: 0,
        k: [0, 0, 0, 0],
      },
    },
    {
      ty: 2,
      nm: 'Map White To',
      mn: 'ADBE Tint-0002',
      v: {
        a: 0,
        k: [1, 1, 1, 0],
      },
    },
    {
      ty: 0,
      nm: 'Amount to Tint',
      mn: 'ADBE Tint-0003',
      v: {
        a: 0,
        k: 100,
      },
    },
  ],
};

// Animation template
const animationTemplate: LottieAnimation = {
  w: 0, // this should be changed to match the animation width
  h: 0, // this should be changed to match the animation height
  ip: 0, // this should be changed to match the animation in point
  op: 0, // this should be changed to match the animation out point
  fr: 0, // this should be changed to match the animation frame rate
  v: '5.0.0', // this should be changed to match the animation version
  ddd: 0,
  assets: [
    {
      id: TEMPLATE_ID,
      nm: 'template_precomp',
      fr: 0,
      layers: [], // Here we'll insert the animation layers
    },
  ],
  layers: [
    {
      ty: 0, // precomp
      ind: 1,
      nm: 'template',
      ddd: 0,
      refId: TEMPLATE_ID,
      ef: [tintEffectTemplate],
      sr: 1, // no stretch
      bm: 0, // normal blend mode
      ks: {
        // Default values for transform properties
        o: {
          a: 0,
          k: 100,
        },
        r: {
          a: 0,
          k: 0,
        },
        p: {
          a: 0,
          k: [0, 0, 0],
          l: 2,
        },
        a: {
          a: 0,
          k: [0, 0, 0],
          l: 2,
        },
        s: {
          a: 0,
          k: [100, 100, 100],
          l: 2,
        },
      },
      ao: 0, // no auto orient
      w: 0, // this should be changed to match the animation width
      h: 0, // this should be changed to match the animation height
      ip: 0, // this should be changed to match the animation in point
      op: 0, // this should be changed to match the animation out point
      st: 0, // this should be changed to match the animation start point
    },
  ],
};

export interface SkottieTemplateEventDetail {
  animation: LottieAnimation;
}

export class SkottieColorManagerSk extends ElementSk {
  private _animation: LottieAnimation | null = null;

  private static template = (ele: SkottieColorManagerSk) => html`
    <div class="wrapper">${ele.renderView()}</div>
  `;

  constructor() {
    super(SkottieColorManagerSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
  }

  private hasFontManager(): boolean {
    // We are using some conditions to check if this animation
    // has already been wrapped with a font manager.
    // There could be some false positives but it's unlikely.
    // 1. it has a single layer in the layers list
    // 2. the layer is of type precomp
    // 3. the layer has a single effect applied to it
    // 4. the effect is a tint effect
    if (this._animation) {
      if (
        this._animation.layers.length === 1 &&
        this._animation.layers[0].refId === TEMPLATE_ID &&
        this._animation.layers[0].ef &&
        this._animation.layers[0].ef.length &&
        this._animation.layers[0].ef[0].ty === TINT_EFFECT_TYPE
      ) {
        return true;
      }
    }
    return false;
  }

  private renderManagedAnimation(): TemplateResult {
    const effect = this._animation?.layers?.[0].ef?.[0] as LottieTintEffect;
    const blackColor = colorToHex(effect.ef[0].v.k);
    const whiteColor = colorToHex(effect.ef[1].v.k);
    const blendValue = effect.ef[2].v.k;
    return html`
      <div class="manager">
        <div class="color-form--color">
          <span>Map Black to</span>
          <skottie-color-input-sk
            .color=${blackColor}
            @color-change=${this.onMapBlackChange}></skottie-color-input-sk>
        </div>
        <div class="color-form--color">
          <span>Map White to</span>
          <skottie-color-input-sk
            .color=${whiteColor}
            @color-change=${this.onMapWhiteChange}></skottie-color-input-sk>
        </div>
        <label class="color-form--original-color">
          <input
            type="checkbox"
            @change=${this.onToggleOriginal}
            ?checked=${blendValue === 0} />
          <span class="box"></span>
          <span class="label">Show original color scheme</span>
        </label>
      </div>
    `;
  }

  private renderUnmanagedAnimation(): TemplateResult {
    return html`
      <div class="no-manager">
        <div class="info-box">
          <span class="icon-sk info-box--icon">info</span>
          <span class="info-box--description">
            The color manager will modify the Json file to allow color editing
            and it will not be possible to revert it
          </span>
        </div>
        <skottie-button-sk
          id="view-perf-chart"
          @select=${this.applyColorManager}
          .content=${'Apply color manager'}
          .classes=${['apply-btn']}>
        </skottie-button-sk>
      </div>
    `;
  }

  private applyColorManager(): void {
    if (!this._animation) {
      return;
    }
    // Making a copy of the template to avoid writing on the original one
    const template: LottieAnimation = JSON.parse(
      JSON.stringify(animationTemplate)
    );
    const animation = this._animation;
    template.v = animation.v;
    template.w = animation.w;
    template.h = animation.h;
    template.ip = animation.ip;
    template.op = animation.op;
    template.fr = animation.fr;
    template.fonts = animation.fonts;
    const assets = template.assets;
    const compAsset = assets[0] as LottieCompAsset;
    compAsset.layers = this._animation.layers;
    this._animation.assets.forEach((asset) => {
      assets.push(asset);
    });
    template.props = this._animation.props;
    template.markers = this._animation.markers;

    const rootLayer = template.layers[0];
    rootLayer.w = animation.w;
    rootLayer.h = animation.h;
    rootLayer.ip = animation.ip;
    rootLayer.op = animation.op;

    JSON.stringify(template);
    this.animation = template;
    this.dispatch();
  }

  private renderNoAnomation(): TemplateResult {
    return html`<span>No active animation</span>`;
  }

  private renderView(): TemplateResult {
    if (!this._animation) {
      return this.renderNoAnomation();
    }
    if (this.hasFontManager()) {
      return this.renderManagedAnimation();
    }
    return this.renderUnmanagedAnimation();
  }

  private onMapBlackChange(ev: CustomEvent<SkottieColorEventDetail>): void {
    if (!this._animation) {
      return;
    }
    const color = hexToColor(ev.detail.color);
    const effect = this._animation.layers?.[0].ef?.[0] as LottieTintEffect;
    // Index 0 of effect is black map
    effect.ef[0].v.k = color;
    this.dispatch();
    this._render();
  }

  private onMapWhiteChange(ev: CustomEvent<SkottieColorEventDetail>): void {
    if (!this._animation) {
      return;
    }
    const color = hexToColor(ev.detail.color);
    const effect = this._animation.layers?.[0].ef?.[0] as LottieTintEffect;
    // Index 1 of effect is white map
    effect.ef[1].v.k = color;
    this.dispatch();
    this._render();
  }

  private onToggleOriginal(ev: Event): void {
    if (!this._animation) {
      return;
    }
    const checkboxElement = ev.target as HTMLInputElement;
    const merge = checkboxElement.checked === true ? 0 : 100;
    const effect = this._animation.layers?.[0].ef?.[0] as LottieTintEffect;
    // Index 2 of effect is blending with original value
    // 100 is fully blend, 0 is no blend
    effect.ef[2].v.k = merge;
    this.dispatch();
    this._render();
  }

  dispatch(): void {
    if (this._animation) {
      this.dispatchEvent(
        new CustomEvent<SkottieTemplateEventDetail>('animation-updated', {
          detail: {
            animation: this._animation,
          },
          bubbles: true,
        })
      );
    }
  }

  set animation(value: LottieAnimation) {
    this._animation = value;
    this._render();
  }
}

define('skottie-color-manager-sk', SkottieColorManagerSk);
