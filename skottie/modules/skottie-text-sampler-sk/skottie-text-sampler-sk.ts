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
import { html, TemplateResult } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../skottie-button-sk';
import { FontType } from '../skottie-font-selector-sk/skottie-font-selector-sk';

type SampleText = {
  id: string;
  language: string;
  text: string;
  font: FontType;
};

const samples: SampleText[] = [
  {
    id: '1',
    language: 'English',
    text: 'Hope you had a great day, dude',
    font: {
      fName: 'Roboto-Regular',
      fStyle: 'Regular',
      fFamily: 'Roboto',
    },
  },
  {
    id: '2',
    language: 'Devanagari',
    text: 'आशा है कि आपका दिन अच्छा रहा, दोस्त',
    font: {
      fName: 'NotoSerifDevanagari-Regular',
      fStyle: 'Regular',
      fFamily: 'NotoSerifDevanagari',
    },
  },
  {
    id: '3',
    language: 'Chinese simplified',
    text: '希望你今天过得愉快，伙计',
    font: {
      fName: 'NotoSansSC-Regular',
      fStyle: 'Regular',
      fFamily: 'NotoSansSC',
    },
  },
  {
    id: '4',
    language: 'Korean',
    text: '좋은 하루 보내길 바래, 친구',
    font: {
      fName: 'NotoSansKR-Regular',
      fStyle: 'Regular',
      fFamily: 'NotoSansKR',
    },
  },
  {
    id: '5',
    language: 'Latin characters',
    text: 'ñãõáéíóúàèìòùöüãõçøâçêëîïôùûüÿ',
    font: {
      fName: 'Roboto-Regular',
      fStyle: 'Regular',
      fFamily: 'Roboto',
    },
  },
];

export interface SkTextSampleEventDetail {
  text: string;
  font: FontType;
}

export class SkottieTextSamplerSk extends ElementSk {
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
          ele.updateSample(sample);
        }}
        type="outline"
        .content=${sample.language}
        .classes=${['sample__button']}>
      </skottie-button-sk>
    `;
  }

  private renderSamples(): TemplateResult[] {
    return samples.map((sample: SampleText) => this.renderSample(this, sample));
  }

  private updateSample(sample: SampleText): void {
    const { text, font } = sample;
    // This event handler replaces the animation in place
    // instead of creating a copy of the lottie animation.
    // If there is a reason why it should create a copy, this can be updated.
    if (text) {
      this.dispatchEvent(
        new CustomEvent<SkTextSampleEventDetail>('select-text', {
          detail: {
            text: text,
            font: font,
          },
        })
      );
    }
  }
}

define('skottie-text-sampler-sk', SkottieTextSamplerSk);
