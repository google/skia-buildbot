/**
 * @module skottie-text-sampler-sk
 * @description <h2><code>skottie-text-sampler-sk</code></h2>
 *
 * <p>
 *   A component with a list of text samples in multiple languages
 *   to try on text animations.
 * </p>
 *
 *
 */
import { define } from '../../../elements-sk/modules/define';
import { html, TemplateResult } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../skottie-button-sk';
import { LottieAnimation, LottieAsset, LottieLayer } from '../types';
import { isCompAsset } from '../helpers/animation';

type SampleText = {
  id: string;
  language: string;
  text: string;
};

const samples: SampleText[] = [
  {
    id: '1',
    language: 'English',
    text: 'Hope you had a great day, dude',
  },
  {
    id: '2',
    language: 'Devanagari',
    text: 'आशा है कि आपका दिन अच्छा रहा, दोस्त',
  },
  {
    id: '3',
    language: 'Chinese simplified',
    text: '希望你今天过得愉快，伙计',
  },
  {
    id: '4',
    language: 'Korean',
    text: '좋은 하루 보내길 바래, 친구',
  },
  {
    id: '5',
    language: 'Latin characters',
    text: 'ñãõáéíóúàèìòùöüãõçøâçêëîïôùûüÿ',
  },
];

const LAYER_TYPE_TEXT = 5;

export interface SkottieTextSampleEventDetail {
  animation: LottieAnimation;
}

export class SkottieTextSamplerSk extends ElementSk {
  private _animation: LottieAnimation | null = null;

  private static template = (ele: SkottieTextSamplerSk) => html`
    <div class="wrapper">${ele.renderSamples()}</div>
  `;

  constructor() {
    super(SkottieTextSamplerSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  private renderSample(
    ele: SkottieTextSamplerSk,
    sample: SampleText
  ): TemplateResult {
    return html`
      <skottie-button-sk
        @select=${() => {
          ele.updateText(sample.text);
        }}
        type="outline"
        .content=${sample.language}
        .classes=${['sample__button']}>
      </skottie-button-sk>
    `;
  }

  private renderSamples(): TemplateResult[] {
    return samples.map((sample: SampleText) => {
      return this.renderSample(this, sample);
    });
  }

  private updateTextInLayers(layers: LottieLayer[], text: string): void {
    layers.forEach((layer) => {
      if (layer.ty === LAYER_TYPE_TEXT) {
        if (layer.t) {
          layer.t.d.k[0].s.t = text;
        }
      }
    });
  }

  private updateTextInAssets(assets: LottieAsset[], text: string): void {
    assets.forEach((asset) => {
      if (isCompAsset(asset)) {
        this.updateTextInLayers(asset.layers, text);
      }
    });
  }

  private updateText(text: string): void {
    // This event handler replaces the animation in place
    // instead of creating a copy of the lottie animation.
    // If there is a reason why it should create a copy, this can be updated.
    if (text && this._animation) {
      this.updateTextInLayers(this._animation.layers, text);
      this.updateTextInAssets(this._animation.assets, text);
      this.dispatchEvent(
        new CustomEvent<SkottieTextSampleEventDetail>('animation-updated', {
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

define('skottie-text-sampler-sk', SkottieTextSamplerSk);
