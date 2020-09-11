/**
 * @module modules/fiddle-sk
 * @description <h2><code>fiddle-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../textarea-numbers-sk';
import 'elements-sk/checkbox-sk';
import 'elements-sk/select-sk';
import 'elements-sk/spinner-sk';

export class FiddleSk extends ElementSk {
  private static displayOptions = (ele: FiddleSk) =>
    !ele.display_options
      ? html``
      : html`
          <details open=${ele.options_open}>
            <summary>Options</summary>
            <div class="options">
              <checkbox-sk
                id="textonly"
                label="Text Only [Use SkDebugf()]"
                ?checked=${ele.textonly}
                ?hidden=${ele.basic_mode}
              ></checkbox-sk>

              <checkbox-sk
                id="srgb"
                label="sRGB"
                ?checked=${ele.srgb}
                ?disabled=${ele.f16}
                ?hidden=${ele.basic_mode}
              ></checkbox-sk>

              <checkbox-sk
                id="f16"
                title="Half floats"
                label=${ele.f16}
                ?disabled=${!ele.srgb}
                ?hidden=${ele.basic_mode}
              >
              </checkbox-sk>

              <div>
                <label>
                  Width
                  <input
                    type="number"
                    min="4"
                    max="2048"
                    placeholder="128"
                    .value=${ele.width}
                  />
                </label>
                <label>
                  Height
                  <input
                    type="number"
                    min="4"
                    max="2048"
                    placeholder="128"
                    .value=${ele.height}
                  />
                </label>
              </div>
              <checkbox-sk
                id="animated"
                label="Animation"
                ?checked=${ele.animated}
              ></checkbox-sk>

              <div ?hidden=${!ele.animated} class="offset">
                <label>
                  duration (seconds)
                  <input
                    type="number"
                    min="1"
                    max="300"
                    .value=${ele.duration}
                    ?disabled=${!ele.animated}
                    @click=${ele.animatedClick}
                  />
                </label>

                <div ?hidden=${!ele.animated}>
                  <h4>These globals are now defined:</h4>
                  <pre class="source-select">
double duration; // The requested duration of the animation.
double frame;    // A value in [0, 1] of where we are in the animation.</pre
                  >
                </div>
              </div>

              <!-- Add offscreen opts here-->
              <div>
                <checkbox-sk
                  id="offscreen"
                  title="Create an offscreen render target on the GPU."
                  label="Offscreen Render Target"
                  ?checked=${ele.offscreen}
                  @click=${ele.offscreenClick}
                  ?hidden=${ele.basic_mode}
                >
                </checkbox-sk>
                <div ?hidden=${!ele.offscreen} class="offset">
                  <div>
                    <label>
                      Width
                      <input
                        type="number"
                        min="4"
                        max="2048"
                        value=${ele.offscreen_width}
                      />
                    </label>
                    <label>
                      Height
                      <input
                        type="number"
                        min="4"
                        max="2048"
                        value=${ele.offscreen_height}
                      />
                    </label>
                    <label>
                      Sample Count
                      <input
                        type="number"
                        value=${ele.offscreen_sample_count}
                      />
                    </label>
                  </div>
                  <checkbox-sk
                    id="texturable"
                    label="Texturable"
                    title="The offscreen render target can be used as a texture."
                    ?checked=${ele.offscreen_texturable}
                    @click=${ele.offscreenTexturableClick}
                  ></checkbox-sk>

                  <div class="indent">
                    <checkbox-sk
                      id="offscreen_mipmap"
                      label="MipMap"
                      title="The offscreen render target can be used as a texture that is mipmapped."
                      ?checked=${ele.offscreen_mipmap}
                      ?disabled=${!ele.offscreen_texturable}
                    ></checkbox-sk>
                  </div>

                  <h4>This global is now defined:</h4>
                  <pre
                    class="source-select"
                    ?hidden=${ele.offscreen_texturable}
                  >
GrBackendRenderTarget backEndRenderTarget;</pre
                  >
                  <pre
                    class="source-select"
                    ?hidden=${!ele.offscreen_texturable}
                  >
GrBackendTexture backEndTextureRenderTarget;</pre
                  >
                </div>
              </div>

              <h3>Optional source image</h3>
              <select-sk class="layout horizontal wrap">
                ${ele.sources.map((source, index) => {
                  html`
                    <img
                      width="64"
                      height="64"
                      ?selected=${source === ele.source}
                      name=${source}
                      src="${ele.domain}/s/${ele.source}"
                      class="imgsrc"
                    />
                  `;
                })}
              </select-sk>
              <div ?hidden=${!ele.source} class="offset">
                <checkbox-sk
                  title="The backEndTexture is mipmapped."
                  ?checked=${ele.source_mipmap}
                  >MipMap</checkbox-sk
                >
                <h4>These globals are now defined:</h4>
                <pre class="source-select">
SkBitmap source;
sk_sp&lt;SkImage> image;
GrBackendTexture backEndTexture; // GPU Only.</pre
                >
              </div>
            </div>
          </details>
        `;

  private static actions = (ele: FiddleSk) =>
    ele.embedded
      ? html``
      : html`
          <div id="submit">
            <button class="action" @click=${ele.run}>Run</button>
            <spinner-sk></spinner-sk>

            <a
              id="bug"
              ?hidden=${!ele.bug_link}
              target="_blank"
              href=${ele.bugLink(ele.fiddlehash)}
              >File Bug</a
            >
            <details hidden$="[[_basic_mode]]">
              <summary>Embed</summary>
              <h3>Embed as an image with a backlink:</h3>
              <input
                type="text"
                readonly
                size="150"
                value="&lt;a href='https://fiddle.skia.org/c/${ele.fiddlehash}'>&lt;img src='https://fiddle.skia.org/i/${ele.fiddlehash}_raster.png'>&lt;/a>"
              />
              <h3>Embed as custom element (skia.org only):</h3>
              <input
                type="text"
                readonly
                size="150"
                value="&lt;fiddle-embed name='${ele.fiddlehash}'>&lt;/fiddle-embed> "
              />
            </details>
          </div>
        `;

  private static template = (ele: FiddleSk) =>
    html`
    ${FiddleSk.displayOptions(ele)}

    <textarea-numbers-sk .value=${ele.code}></textarea-numbers-sk>

    ${FiddleSk.actions(ele)}
  <div on-tap="_errSelect">
    <template is="dom-if" if="{{_hasCompileWarningsOrErrors(_compile_errors)}}">
      <h2>Compilation Warnings/Errors</h2>
      <template is="dom-repeat" items="{{_compile_errors}}">
        <pre class=compile-error data-line$="{{item.line}}" data-col$="{{item.col}}">{{item.text}}</pre>
      </template>
    </template>
  </div>
  <template is="dom-if" if="{{_runtime_error}}">
    <h2>Runtime Errors</h2>
    <div>{{_runtime_error}}</div>
  </template>

  <template is="dom-if" if="{{_hasImages(fiddlehash, _compile_errors, _runtime_error, textonly)}}">
    <div id=results class="horizontal layout self-start">

      <template is="dom-if" if="{{_showCpu(cpu_embedded,gpu_embedded,embedded)}}">
        <div class="vertical layout center-justified">
          <template is="dom-if" if="{{_not(animated)}}">
            <img title=CPU src="[[domain]]/i/{{fiddlehash}}_raster.png" width="{{width}}" height="{{height}}">
          </template>
          <template is="dom-if" if="{{animated}}">
            <video title=CPU on-ended="_playEnded" autoplay loop="[[loop]]" src="[[domain]]/i/{{fiddlehash}}_cpu.webm" width="{{width}}" height="{{height}}"></video>
          </template>
          <template is="dom-if" if="{{_not(embedded)}}">
            <p>
              CPU
            </p>
          </template>
        </div>
      </template>

      <template is="dom-if" if="[[_showGpu(gpu_embedded,embedded,_basic_mode)]]">
        <div class="vertical layout center-justified">
          <template is="dom-if" if="[[_not(animated)]]">
            <img title=GPU src="[[domain]]/i/[[fiddlehash]]_gpu.png" width="[[width]]" height="[[height]]">
          </template>
          <template is="dom-if" if="{{animated}}">
            <video title=GPU loop="[[loop]]" autoplay src="[[domain]]/i/{{fiddlehash}}_gpu.webm" width="{{width}}" height="{{height}}"></video>
          </template>
          <template is="dom-if" if="{{_not(embedded)}}">
            <p>
              GPU
            </p>
          </template>
        </div>
      </template>

      <template is="dom-if" if="[[_showLinks(animated,embedded,_basic_mode)]]">
        <div class="vertical layout center">
          <a href="[[domain]]/i/[[fiddlehash]].pdf">PDF</a>
        </div>
        <div class="vertical layout center">
          <a href="[[domain]]/i/[[fiddlehash]].skp">SKP</a>
        </div>
        <div class="vertical layout center">
          <a href="https://debugger.skia.org/loadfrom?url=https://fiddle.skia.org/i/{{fiddlehash}}.skp">Debug</a>
        </div>
      </template>

      <template is="dom-if" if="{{embedded}}">
        <button class=action on-tap="_run">Run</button>
        <paper-spinner></paper-spinner>
        <a href="https://fiddle.skia.org/c/{{fiddlehash}}" target="_blank">Pop-out</a>
      </template>

      <template is="dom-if" if="{{animated}}">
        <div id=controls class="horizontal layout">
          <button on-tap="_playToggle" title="Play the animation."><iron-icon id=play icon="av:pause"></iron-icon> </button>
          <checkbox-sk id=loop ?checked="{{loop}}" title="Run animations in a loop">Loop</checkbox-sk>
          <select id=speed on-change="_speed" size="1">
            <option value="0.25">0.25</option>
            <option value="0.5">0.5</option>
            <option value="0.75">0.75</option>
            <option value="1" selected>Normal speed</option>
            <option value="1.25">1.25</option>
            <option value="1.5">1.5</option>
            <option value="2">2</option>
          </select>
        </div>
      </template>
    </div>
  </template>

  <template is="dom-if" if="{{textonly}}">
    <template is="dom-if" if="{{_not(embedded)}}">
      <h2>Output</h2>
    </template>

    <div class="layout horizontal">
      <div class=textoutput>
        <text-src src="[[_textURL(domain, fiddlehash)]]"></text-src>
      </div>
      <template is="dom-if" if="{{embedded}}">
        <button class=action on-tap="_run">Run</button>
        <paper-spinner></paper-spinner>
        <a href="https://fiddle.skia.org/c/{{fiddlehash}}" target=_blank">Pop-out</a>
      </template>
    </div>
  </template>

  <template is="dom-if" if="{{_not(embedded)}}">
    <template is="dom-if" if="{{_hasImages(fiddlehash, _compile_errors, _runtime_error, textonly)}}">
      <details-sk>
          <summary-sk>Run Details</summary-sk>
          <text-src id=glinfo src="[[_glinfoURL(domain, fiddlehash)]]"></text-src>
      </details-sk>
    </template>
  </template>
    `;

  private display_options: boolean = false;
  private embedded: boolean = false;
  private options_open: boolean = false;
  private textonly: boolean = false;
  private basic_mode: boolean = false;
  private srgb: boolean = false;
  private f16: boolean = false;
  private width: number = 128;
  private height: number = 128;
  private animated: boolean = false;
  private duration: number = 5;
  private offscreen: boolean = false;
  private offscreen_width: number = 128;
  private offscreen_height: number = 128;
  private offscreen_sample_count: number = 1;
  private offscreen_texturable: boolean = false;
  private offscreen_mipmap: boolean = false;
  private sources: number[] = [];
  private source: number = 0;
  private domain: string = 'fiddle.skia.org';
  private source_mipmap: boolean = false;
  private code: string = '';

  private fiddlehash: string = '';
  private bug_link: boolean = false;

  bugLink(fiddlehash: string): string {
    throw new Error('Method not implemented.');
  }
  run() {
    throw new Error('Method not implemented.');
  }
  animatedClick() {
    throw new Error('Method not implemented.');
  }
  offscreenTexturableClick() {
    throw new Error('Method not implemented.');
  }
  offscreenClick() {
    throw new Error('Method not implemented.');
  }

  constructor() {
    super(FiddleSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }
}

define('fiddle-sk', FiddleSk);
