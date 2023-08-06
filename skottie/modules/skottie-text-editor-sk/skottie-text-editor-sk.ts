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
import { html } from 'lit-html';
import { define } from '../../../elements-sk/modules/define';
import { ExtraLayerData, replaceTexts, TextData } from './text-replace';
import { LottieAnimation, LottieAsset, LottieLayer, ViewMode } from '../types';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { isCompAsset } from '../helpers/animation';
import './text-box/text-box';

export interface TextEditApplyEventDetail {
  animation: LottieAnimation;
}

const LAYER_TEXT_TYPE = 5;
const COMP_ROOT_NAME = 'Root';

export class SkottieTextEditorSk extends ElementSk {
  private static template = (ele: SkottieTextEditorSk) => html`
    <div>
      <ul class="text-container">
        ${ele.texts.map((item: TextData) => ele.textElement(item))}
      </ul>
    </div>
  `;

  private textElement = (item: TextData) => html`
    <skottie-text-editor-box-sk
      .textData=${item}
      .mode=${this.mode}
      @change=${this.save}>
    </skottie-text-editor-box-sk>
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
    let comp = animationLayers.find(
      (layer: LottieLayer) => layer.refId === precompId
    );
    if (comp) {
      return comp.nm;
    }
    const animationAssets = animation.assets;
    animationAssets.forEach((asset: LottieAsset) => {
      if (isCompAsset(asset)) {
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
      .map((layer: LottieLayer) => ({
        layer: layer,
        parentId: '',
        precompName: COMP_ROOT_NAME,
      })) // we map them to some extra data
      .concat(
        animation.assets // we iterate over the assets of the animation looking for precomps
          // we filter assets that of type precomp (by querying if they have a layers property)
          .filter((asset: LottieAsset) => isCompAsset(asset))
          .reduce((accumulator: ExtraLayerData[], precomp: LottieAsset) => {
            // we flatten into a single array layers from multiple precomps
            accumulator = accumulator.concat(
              ((isCompAsset(precomp) && precomp.layers) || [])
                .filter((layer: LottieLayer) => layer.ty === LAYER_TEXT_TYPE) // we filter all layers of type text
                .map(
                  (layer: LottieLayer) =>
                    ({
                      // we map them to some extra data
                      layer: layer,
                      parentId: precomp.id,
                      precompName: this.findPrecompName(animation, precomp.id),
                    } as ExtraLayerData)
                )
            );
            return accumulator;
          }, [] as ExtraLayerData[])
      ) // this creates a dictionary with all available texts
      .reduce(
        (
          accumulator: Record<string, TextData>,
          item: ExtraLayerData,
          index: number
        ) => {
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
        },
        {} as Record<string, TextData>
      );
    // we map the dictionary back to an array to get the final texts to render
    this.texts = Object.keys(textsData).map((key: string) => textsData[key]);
  }

  private save() {
    const animation = replaceTexts(this.texts, this._animation!);
    this.dispatchEvent(
      new CustomEvent<TextEditApplyEventDetail>('apply', {
        detail: {
          animation: animation,
        },
      })
    );
  }

  private updateAnimation(animation: LottieAnimation): void {
    if (animation && this.originalAnimation !== animation) {
      const clonedAnimation = JSON.parse(
        JSON.stringify(animation)
      ) as LottieAnimation;
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
