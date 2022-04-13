/**
 * @module skottie-shader-editor-sk
 * @description <h2><code>skottie-shader-editor-sk</code></h2>
 *
 * <p>
 *   A skottie shader editor for custom shader layer effects
 * </p>
 *
 *
 * @evt apply - Generated when the user presses Save.
 *              The updated json is available in the event detail.
 *
 * @attr animation- The lottie json.
 *                  Read at load time, not able to change effects during runtime.
 *
 * @attr mode - View mode.
 *              Supported values are default and presentation
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ShaderData } from './shader-replace';
import {
  LottieAnimation, LottieAsset, LottieLayer, ViewMode,
} from '../types';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

export interface ShaderEditApplyEventDetail {
  shaders: ShaderData[];
}

//TODO(jmbetancourt): find a way to parse through layers and find sksl effects
//constants go here


export class ShaderEditorSk extends ElementSk {
  private static template = (ele: ShaderEditorSk) => html`
  <div>
    <header class="editor-header">
      <div class="editor-header-title">Shader Editor</div>
      <div class="editor-header-separator"></div>
      <button class="editor-header-save-button" @click=${ele.save}>Save</button>
    </header>
    <section>
      <ul class="shader-container">
         ${ele.shaders.map((item: ShaderData) => ele.shaderElement(item))}
      </ul>
    <section>
  </div>
`;

  private shaderElement = (item: ShaderData) => html`
  <li class="shader-element">
    <div class="shader-element-wrapper">
      ${this.shaderElementTitle(item.name)}
      <div class="shader-element-item">
        <div class="shader-element-label">
          Shader:
        </div>
        <textarea class="shader-element-input"
          @change=${(ev: Event) => this.onChange(ev, item)}
          @input=${(ev: Event) => this.onChange(ev, item)}
          .value=${item.shader}
        ></textarea>
      </div>
    </div>
  </li>
`;
  //TODO(jmbetancourt): replace textarea with CodeMirror
  private shaderElementTitle = (name: string) => {
    if (this.mode === 'presentation') {
      return null;
    }
    return html`
  <div class="shader-element-item">
    <div class="shader-element-label">
      Layer name:
    </div>
    <div>
      ${name}
    </div>
  </div>
  `;
  };

  private _animation: LottieAnimation | null = null;

  private originalAnimation: LottieAnimation | null = null;

  private mode: ViewMode = 'default';

  private shaders: ShaderData[] = [];

  constructor() {
    super(ShaderEditorSk.template);
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

  private buildShaders(animation: LottieAnimation): void {
    // TODO(jmbetancourt): search through layers and create the shader elements
    // This is a sample "shader" that can be force the shader layout by flipping
    // the conditional to true while debugging
    if (false) {
      let mockShader : ShaderData;
      mockShader = {
        id: 'demo id',
        name: 'demo name',
        shader: 'mainShaderFunction() {}',
        precompName: 'demo precomp',
        items: []
      }
      this.shaders = [mockShader];
    }
  }

  private save() {
    this.dispatchEvent(new CustomEvent<ShaderEditApplyEventDetail>('apply', {
      detail: {
        shaders: this.shaders,
      },
    }));
  }

  private onChange(e: Event, shaderData: ShaderData): void {
    const target = (e.target as HTMLTextAreaElement);
    const shaderText = target.value;
    shaderData.shader = shaderText;
    // TODO(jmbetancourt): on textbox change, place it in the right place in the Lottie file
  }

  private updateAnimation(animation: LottieAnimation): void {
    if (animation && this.originalAnimation !== animation) {
      const clonedAnimation = JSON.parse(JSON.stringify(animation)) as LottieAnimation;
      this.buildShaders(clonedAnimation);
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

define('skottie-shader-editor-sk', ShaderEditorSk);
