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
import { Options } from '../json';

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
                ?checked=${ele._options.textOnly}
                ?hidden=${ele.basic_mode}
              ></checkbox-sk>

              <checkbox-sk
                id="srgb"
                label="sRGB"
                ?checked=${ele._options.srgb}
                ?disabled=${ele._options.f16}
                ?hidden=${ele.basic_mode}
              ></checkbox-sk>

              <checkbox-sk
                id="f16"
                title="Half floats"
                label=${ele._options.f16}
                ?disabled=${!ele._options.srgb}
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
                    .value=${ele._options.width}
                  />
                </label>
                <label>
                  Height
                  <input
                    type="number"
                    min="4"
                    max="2048"
                    placeholder="128"
                    .value=${ele._options.height}
                  />
                </label>
              </div>
              <checkbox-sk
                id="animated"
                label="Animation"
                ?checked=${ele._options.animated}
              ></checkbox-sk>

              <div ?hidden=${!ele._options.animated} class="offset">
                <label>
                  duration (seconds)
                  <input
                    type="number"
                    min="1"
                    max="300"
                    .value=${ele._options.duration}
                    ?disabled=${!ele._options.animated}
                    @click=${ele.animatedClick}
                  />
                </label>

                <div ?hidden=${!ele._options.animated}>
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
                  ?checked=${ele._options.offscreen}
                  @click=${ele.offscreenClick}
                  ?hidden=${ele.basic_mode}
                >
                </checkbox-sk>
                <div ?hidden=${!ele._options.offscreen} class="offset">
                  <div>
                    <label>
                      Width
                      <input
                        type="number"
                        min="4"
                        max="2048"
                        value=${ele._options.offscreen_width}
                      />
                    </label>
                    <label>
                      Height
                      <input
                        type="number"
                        min="4"
                        max="2048"
                        value=${ele._options.offscreen_height}
                      />
                    </label>
                    <label>
                      Sample Count
                      <input
                        type="number"
                        value=${ele._options.offscreen_sample_count}
                      />
                    </label>
                  </div>
                  <checkbox-sk
                    id="texturable"
                    label="Texturable"
                    title="The offscreen render target can be used as a texture."
                    ?checked=${ele._options.offscreen_texturable}
                    @click=${ele.offscreenTexturableClick}
                  ></checkbox-sk>

                  <div class="indent">
                    <checkbox-sk
                      id="offscreen_mipmap"
                      label="MipMap"
                      title="The offscreen render target can be used as a texture that is mipmapped."
                      ?checked=${ele._options.offscreen_mipmap}
                      ?disabled=${!ele._options.offscreen_texturable}
                    ></checkbox-sk>
                  </div>

                  <h4>This global is now defined:</h4>
                  <pre
                    class="source-select"
                    ?hidden=${ele._options.offscreen_texturable}
                  >
GrBackendRenderTarget backEndRenderTarget;</pre
                  >
                  <pre
                    class="source-select"
                    ?hidden=${!ele._options.offscreen_texturable}
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
                      ?selected=${source === ele._options.source}
                      name=${source}
                      src="${ele.domain}/s/${ele._options.source}"
                      class="imgsrc"
                    />
                  `;
                })}
              </select-sk>
              <div ?hidden=${!ele._options.source} class="offset">
                <checkbox-sk
                  title="The backEndTexture is mipmapped."
                  ?checked=${ele._options.source_mipmap}
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
    if (!ele._options.textOnly) {
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
            ?hidden=${ele._options.animated}
            title="CPU"
            src="${ele.domain}/i/${ele.fiddlehash}_raster.png"
            width=${ele._options.width}
            height=${ele._options.height}
          />
          <video
            ?hidden=${!ele._options.animated}
            title="CPU"
            @ended=${ele.playEnded}
            autoplay
            loop=${ele.loop}
            src="${ele.domain}/i/${ele.fiddlehash}_cpu.webm"
            width=${ele._options.width}
            height=${ele._options.height}
          >
          </video>
          <p ?hidden=${ele.embedded}> CPU </p>
        </div>

        <div ?hidden=${!ele.showGPU()} class="vertical layout center-justified">
          <img
            ?hidden=${ele._options.animated}
            title="GPU"
            src="${ele.domain}/i/${ele.fiddlehash}_gpu.png"
            width=${ele._options.width}
            height=${ele._options.height}
          />
          <video
            ?hidden=${!ele._options.animated}
            title="GPU"
            loop=${ele.loop}
            autoplay
            src="${ele.domain}/i/${ele.fiddlehash}_gpu.webm"
            width=${ele._options.width}
            height=${ele._options.height}
          ></video>
          <p ?hidden=${ele.embedded}> GPU </p>
        </div>

        <div ?hidden=${!ele.showLinks()}>
          <div class="vertical layout center">
            <a href="${ele.domain}/i/${ele.fiddlehash}.pdf">PDF</a>
          </div>
          <div class="vertical layout center">
            <a href="${ele.domain}/i/${ele.fiddlehash}.skp">SKP</a>
          </div>
          <div class="vertical layout center">
            <a
              href="https://debugger.skia.org/loadfrom?url=https://fiddle.skia.org/i/${ele.fiddlehash}.skp"
              >Debug</a
            >
          </div>
        </div>

        <div ?hidden="${!ele.embedded}">
          <button class="action" @click=${ele.run}>Run</button>
          <spinner-sk></spinner-sk>
          <a href="https://fiddle.skia.org/c/${ele.fiddlehash}" target="_blank"
            >Pop-out</a
          >
        </div>

        <div
          ?hidden=${!ele._options.animated}
          id="controls"
          class="horizontal layout"
        >
          <button @click=${ele.playToggle} title="Play the animation."
            >Play
            <!-- toggle Play/Pause -->
          </button>
          <checkbox-sk
            id="loop"
            ?checked=${ele.loop}
            label="Loop"
            title="Run animations in a loop"
            @click=${ele.loopClick}
          ></checkbox-sk>
          <select id="speed" @change=${ele.speed} size="1">
            <option value="0.25">0.25</option>
            <option value="0.5">0.5</option>
            <option value="0.75">0.75</option>
            <option value="1" selected>Normal speed</option>
            <option value="1.25">1.25</option>
            <option value="1.5">1.5</option>
            <option value="2">2</option>
          </select>
        </div>
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

  private _options: Options = {
    textOnly: false,
    srgb: false,
    f16: false,
    width: 128,
    height: 128,
    animated: false,
    duration: 5,
    offscreen: false,
    offscreen_width: 128,
    offscreen_height: 128,
    offscreen_sample_count: 1,
    offscreen_texturable: false,
    offscreen_mipmap: false,
    source: 0,
    source_mipmap: false,
  };

  private display_options: boolean = false;
  private embedded: boolean = false;
  private options_open: boolean = false;
  private basic_mode: boolean = false;
  private sources: number[] = [];
  private domain: string = 'fiddle.skia.org';
  private bug_link: boolean = false;

  private loop: boolean = true;

  private code: string = '';
  private fiddlehash: string = '';

  private runtime_error: string = '';
  private compile_errors: CompilerError[] = [];

  /** @prop options {string} The options for the fiddle. */
  get options() {
    return this._options;
  }
  set options(val) {
    this._options = val;
    this._render();
  }

  loopClick() {
    throw new Error('Method not implemented.');
  }
  speed() {
    throw new Error('Method not implemented.');
  }
  playToggle() {
    throw new Error('Method not implemented.');
  }
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
  showLinks(): boolean {
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
