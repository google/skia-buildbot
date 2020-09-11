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
import '../test-src-sk';

interface CompilerError {
  line: number;
  col: number;
  text: string;
}

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

  private static textOnlyResults = (ele: FiddleSk) => {
    if (!ele.textonly) {
      return html``;
    }
    return html`
      <h2 ?hidden=${ele.embedded}>Output</h2>

      <div class="layout horizontal">
        <div class="textoutput">
          <test-src-sk src=${ele.textURL()}></test-src-sk>
        </div>
        <div ?hidden=${!ele.embedded}>
          <button class="action" @click=${ele.run}>Run</button>
          <spinner-sk></spinner-sk>
          <a href="https://fiddle.skia.org/c/${ele.fiddlehash}" target="_blank"
            >Pop-out</a
          >
        </div>
      </div>
    `;
  };

  private static runDetails = (ele: FiddleSk) => {
    if (ele.embedded || !ele.hasImages()) {
      return html``;
    }

    return html`
      <details>
        <summary>Run Details</summary>
        <test-src-sk id="glinfo" src=${ele.glinfoURL()}></test-src-sk>
      </details>
    `;
  };

  private static results = (ele: FiddleSk) => {
    if (!ele.hasImages()) {
      return html``;
    }
    return html`
      <div id="results" class="horizontal layout self-start">
        <div ?hidden=${!ele.showCPU()} class="vertical layout center-justified">
          <img
            ?hidden=${ele.animated}
            title="CPU"
            src="${ele.domain}/i/${ele.fiddlehash}_raster.png"
            width=${ele.width}
            height=${ele.height}
          />
          <video
            ?hidden=${!ele.animated}
            title="CPU"
            @ended=${ele.playEnded}
            autoplay
            loop=${ele.loop}
            src="${ele.domain}/i/${ele.fiddlehash}_cpu.webm"
            width=${ele.width}
            height=${ele.height}
          >
          </video>
          <p ?hidden=${ele.embedded}> CPU </p>
        </div>

        <div ?hidden=${!ele.showGPU()} class="vertical layout center-justified">
          <img
            ?hidden=${ele.animated}
            title="GPU"
            src="[[domain]]/i/[[fiddlehash]]_gpu.png"
            width="[[width]]"
            height="[[height]]"
          />
          <video
            ?hidden=${!ele.animated}
            title="GPU"
            loop=${ele.loop}
            autoplay
            src="${ele.domain}/i/${ele.fiddlehash}_gpu.webm"
            width=${ele.width}
            height=${ele.height}
          ></video>
          <p ?hidden=${ele.embedded}> GPU </p>
        </div>

        <template
          is="dom-if"
          if="[[_showLinks(animated,embedded,_basic_mode)]]"
        >
          <div class="vertical layout center">
            <a href="[[domain]]/i/[[fiddlehash]].pdf">PDF</a>
          </div>
          <div class="vertical layout center">
            <a href="[[domain]]/i/[[fiddlehash]].skp">SKP</a>
          </div>
          <div class="vertical layout center">
            <a
              href="https://debugger.skia.org/loadfrom?url=https://fiddle.skia.org/i/{{fiddlehash}}.skp"
              >Debug</a
            >
          </div>
        </template>

        <template is="dom-if" if="{{embedded}}">
          <button class="action" on-tap="_run">Run</button>
          <spinner-sk></spinner-sk>
          <a href="https://fiddle.skia.org/c/{{fiddlehash}}" target="_blank"
            >Pop-out</a
          >
        </template>

        <template is="dom-if" if="{{animated}}">
          <div id="controls" class="horizontal layout">
            <button on-tap="_playToggle" title="Play the animation."
              ><iron-icon id="play" icon="av:pause"></iron-icon>
            </button>
            <checkbox-sk
              id="loop"
              ?checked="{{loop}}"
              title="Run animations in a loop"
              >Loop</checkbox-sk
            >
            <select id="speed" on-change="_speed" size="1">
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

      ${FiddleSk.textOnlyResults(ele)} ${FiddleSk.runDetails(ele)}
    `;
  };
  private static template = (ele: FiddleSk) =>
    html`
    ${FiddleSk.displayOptions(ele)}

    <textarea-numbers-sk .value=${ele.code}></textarea-numbers-sk>

    ${FiddleSk.actions(ele)}

  <div @click=${ele.errClick} ?hidden=${!ele.hasCompileWarningsOrErrors()}>
      <h2>Compilation Warnings/Errors</h2>
      ${ele.compile_errors.map(
        (err) =>
          html`<pre
            class="compile-error"
            data-line=${err.line}
            data-col=${err.col}
          >
            ${err.text}
          </pre
          >`
      )}
  </div>
  <div ?hidden=${!ele.runtime_error}>
    <h2>Runtime Errors</h2>
    <div>${ele.runtime_error}</div>
  </template>

  ${FiddleSk.results(ele)}
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
  private compile_errors: CompilerError[] = [];
  private runtime_error: string = '';
  private loop: boolean = true;

  playEnded() {
    throw new Error('Method not implemented.');
  }
  hasImages(): boolean {
    throw new Error('Method not implemented.');
  }
  hasCompileWarningsOrErrors(): unknown {
    throw new Error('Method not implemented.');
  }
  errClick() {
    throw new Error('Method not implemented.');
  }
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
  glinfoURL(): string {
    throw new Error('Method not implemented.');
  }
  textURL(): unknown {
    throw new Error('Method not implemented.');
  }
  showCPU(): boolean {
    throw new Error('Method not implemented.');
  }
  showGPU(): boolean {
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
