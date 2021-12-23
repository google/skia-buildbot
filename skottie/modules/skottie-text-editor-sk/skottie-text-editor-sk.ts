/**
 * @module skottie-text-editor-sk
 * @description <h2><code>skottie-text-editor-sk</code></h2>
 *
 * <p>
 *   A skottie text editor
 * </p>
 *
 *
 * @evt apply - This event is generated when the user presses Apply.
 *         The updated json is available in the event detail.
 *
 * @attr animation the animation json.
 *         At the moment it only reads it at load time.
 *
 * @attr mode - the view mode.
 *         Supported values are default and presentation
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ifDefined } from 'lit-html/directives/if-defined';
import { ExtraLayerData, TextData } from './text-replace';
import {
  LottieAnimation, LottieAsset, LottieLayer, ViewMode,
} from '../types';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

export interface TextEditApplyEventDetail {
  texts: TextData[];
}

const LAYER_TEXT_TYPE = 5;
const COMP_ROOT_NAME = 'Root';
const LINE_FEED = 10;
const FORM_FEED = 13;

export class SkottieTextEditorSk extends ElementSk {
  private static template = (ele: SkottieTextEditorSk) => html`
  <div>
    <header class="editor-header">
      <div class="editor-header-title">Text Editor</div>
      <div class="editor-header-separator"></div>
      ${ele.ungroupButton()}
      <button class="editor-header-save-button" @click=${ele.save}>Save</button>
    </header>
    <section>
      <ul class="text-container">
         ${ele.texts.map((item: TextData) => ele.textElement(item))}
      </ul>
    <section>
  </div>
`;

  private ungroupButton = () => {
    if (this.mode === 'presentation') {
      return null;
    }
    return html`
  <button
    class="editor-header-save-button"
    @click=${this.toggleTextsCollapse}>
    ${this.areTextsCollapsed
      ? 'Ungroup Texts'
      : 'Group Texts'}
    </button>`;
  };

  private textElement = (item: TextData) => html`
  <li class="text-element">
    <div class="text-element-wrapper">
      ${this.textElementTitle(item.name)}
      <div class="text-element-item">
        <div class="text-element-label">
          Layer text:
        </div>
        <textarea class="text-element-input"
          @change=${(ev: Event) => this.onChange(ev, item)}
          @input=${(ev: Event) => this.onChange(ev, item)}
          maxlength=${ifDefined(item.maxChars)}
          .value=${item.text}
        ></textarea>
      </div>
      <div>${this.originTemplate(item)}</div>
    </div>
  </li>
`;

  private textElementTitle = (name: string) => {
    if (this.mode === 'presentation') {
      return null;
    }
    return html`
  <div class="text-element-item">
    <div class="text-element-label">
      Layer name:
    </div>
    <div>
      ${name}
    </div>
  </div>
  `;
  };

  private originTemplate = (group: TextData) => {
    if (this.mode === 'presentation') {
      return null;
    }
    return html`
    <div class="text-element-item">
      <div class="text-element-label">
        Origin${group.items.length > 1 ? 's' : ''}:
      </div>
        <ul>
          ${group.items.map(SkottieTextEditorSk.originTemplateElement)}
        </ul>
    </div>
  `;
  };

  private static originTemplateElement = (item: ExtraLayerData) => html`
  <li class="text-element-origin">
    <b>${item.precompName}</b> > Layer ${item.layer.ind}
  </li>
`;

  private _animation: LottieAnimation | null = null;

  private areTextsCollapsed: boolean = true;

  private originalAnimation: LottieAnimation | null = null;

  private mode: ViewMode = 'default';

  private texts: TextData[] = [];

  constructor() {
    super(SkottieTextEditorSk.template);
  }

  findPrecompName(animation: LottieAnimation, precompId: string): string {
    const animationLayers = animation.layers;
    let comp = animationLayers.find((layer: LottieLayer) => layer.refId === precompId);
    if (comp) {
      return comp.nm;
    }
    const animationAssets = animation.assets;
    animationAssets.forEach((asset: LottieAsset) => {
      if (asset.layers) {
        asset.layers.forEach((layer: LottieLayer) => {
          if (layer.refId === precompId) {
            comp = layer;
          }
        });
      }
    });
    if (comp) {
      return (comp as LottieLayer).nm;
    }
    return 'not found';
  }

  private buildTexts(animation: LottieAnimation): void {
    const textsData = animation.layers // we iterate all layer at the root layer
      .filter((layer: LottieLayer) => layer.ty === LAYER_TEXT_TYPE) // we filter all layers of type text
      .map((layer: LottieLayer) => ({ layer: layer, parentId: '', precompName: COMP_ROOT_NAME })) // we map them to some extra data
      .concat(
        animation.assets // we iterate over the assets of the animation looking for precomps
          .filter((asset: LottieAsset) => asset.layers) // we filter assets that of type precomp (by querying if they have a layers property)
          .reduce((accumulator: ExtraLayerData[], precomp: LottieAsset) => { // we flatten into a single array layers from multiple precomps
            accumulator = accumulator.concat(precomp.layers
              .filter((layer: LottieLayer) => layer.ty === LAYER_TEXT_TYPE) // we filter all layers of type text
              .map((layer: LottieLayer) => ({ // we map them to some extra data
                layer: layer,
                parentId: precomp.id,
                precompName: this.findPrecompName(animation, precomp.id),
              } as ExtraLayerData)));
            return accumulator;
          }, [] as ExtraLayerData[]),
      ) // this creates a dictionary with all available texts
      .reduce((accumulator: Record<string, TextData>, item: ExtraLayerData, index: number) => {
        const key: string = this.areTextsCollapsed
          ? item.layer.nm // if texts are collapsed the key will be the layer name (nm)
          : String(index + 1); // if they are not collapse we use the index as key to be unique
        if (!accumulator[key]) {
          accumulator[key] = {
            id: item.layer.nm,
            name: item.layer.nm,
            items: [],
            // this property is the text string of a text layer.
            // It's read as: Text Element > Text document > First Keyframe > Start Value > Text
            text: item.layer.t?.d.k[0].s.t || 'unnamed layer',
            maxChars: item.layer.t?.d.k[0].s.mc, // Max characters text document attribute
            precompName: item.precompName,
          };
        }

        accumulator[key].items.push(item);
        return accumulator;
      }, {} as Record<string, TextData>);
    // we map the dictionary back to an array to get the final texts to render
    this.texts = Object.keys(textsData)
      .map((key: string) => textsData[key]);
  }

  private save() {
    this.dispatchEvent(new CustomEvent<TextEditApplyEventDetail>('apply', {
      detail: {
        texts: this.texts,
      },
    }));
  }

  private toggleTextsCollapse(): void {
    this.areTextsCollapsed = !this.areTextsCollapsed;
    this.buildTexts(this._animation!);
    this._render();
  }

  private static sanitizeText(text: string): string {
    let sanitizedText = '';
    for (let i = 0; i < text.length; i += 1) {
      if (text.charCodeAt(i) === LINE_FEED) {
        sanitizedText += String.fromCharCode(FORM_FEED);
      } else {
        sanitizedText += text.charAt(i);
      }
    }
    return sanitizedText;
  }

  private onChange(e: Event, textData: TextData): void {
    const target = (e.target as HTMLTextAreaElement);
    const text = SkottieTextEditorSk.sanitizeText(target.value);
    textData.text = text;
    textData.items.forEach((item: ExtraLayerData) => {
      // this property is the text string of a text layer.
      // It's read as: Text Element > Text document > First Keyframe > Start Value > Text
      if (item.layer.t) {
        item.layer.t.d.k[0].s.t = text;
      }
    });
  }

  private updateAnimation(animation: LottieAnimation): void {
    if (animation && this.originalAnimation !== animation) {
      const clonedAnimation = JSON.parse(JSON.stringify(animation)) as LottieAnimation;
      this.buildTexts(clonedAnimation);
      this._animation = clonedAnimation;
      this.originalAnimation = animation;
      this._render();
    }
  }

  set animation(val: LottieAnimation) {
    this.updateAnimation(val);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this.updateAnimation(this.animation);
    this._render();
  }
}

define('skottie-text-editor-sk', SkottieTextEditorSk);
