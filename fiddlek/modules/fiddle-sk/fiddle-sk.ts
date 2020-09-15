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
import { Options, RunResults } from '../json';
import { fromObject } from 'common-sk/modules/query';
import { TextareaNumbersSk } from '../textarea-numbers-sk/textarea-numbers-sk';
import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk';

// Config represents the configuration for how the FiddleSk element appears.
interface Config {
  display_options: boolean;
  embedded: boolean;
  options_open: boolean;
  basic_mode: boolean;
  domain: string;
  bug_link: boolean;
  sources: number[];
  loop: boolean;
}

export class FiddleSk extends ElementSk {
  private static displayOptions = (ele: FiddleSk) =>
    !ele._config.display_options
      ? html``
      : html`
          <details ?open=${ele._config.options_open}>
            <summary>Options</summary>
            <div class="options">
              <checkbox-sk
                id="textonly"
                label="Text Only [Use SkDebugf()]"
                ?checked=${ele._options.textOnly}
                ?hidden=${ele._config.basic_mode}
                @change=${ele.textOnlyChange}
              ></checkbox-sk>

              <checkbox-sk
                id="srgb"
                label="sRGB"
                ?checked=${ele._options.srgb}
                ?disabled=${ele._options.f16}
                ?hidden=${ele._config.basic_mode}
                @change=${ele.srgbChange}
              ></checkbox-sk>

              <checkbox-sk
                id="f16"
                title="Half floats"
                label="F16"
                ?checked=${ele._options.f16}
                ?disabled=${!ele._options.srgb}
                ?hidden=${ele._config.basic_mode}
                @change=${ele.f16Change}
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
                  ?hidden=${ele._config.basic_mode}
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
              <select-sk>
                ${ele._config.sources.map((source, index) => {
                  return html`
                    <img
                      width="64"
                      height="64"
                      ?selected=${source === ele._options.source}
                      name=${source}
                      src="https://${ele._config.domain}/s/${source}"
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
    ele._config.embedded
      ? html``
      : html`
          <div id="submit">
            <button class="action" @click=${ele.run}>Run</button>
            <spinner-sk></spinner-sk>

            <a
              id="bug"
              ?hidden=${!ele._config.bug_link}
              target="_blank"
              href=${ele.bugLink(ele._runResults.fiddleHash)}
              >File Bug</a
            >
            <details ?hidden=${ele._config.basic_mode}>
              <summary>Embed</summary>
              <h3>Embed as an image with a backlink:</h3>
              <input
                type="text"
                readonly
                size="150"
                value="&lt;a href='https://fiddle.skia.org/c/${ele._runResults
                  .fiddleHash}'>&lt;img src='https://fiddle.skia.org/i/${ele
                  ._runResults.fiddleHash}_raster.png'>&lt;/a>"
              />
              <h3>Embed as custom element (skia.org only):</h3>
              <input
                type="text"
                readonly
                size="150"
                value="&lt;fiddle-embed name='${ele._runResults
                  .fiddleHash}'>&lt;/fiddle-embed> "
              />
            </details>
          </div>
        `;

  private static textOnlyResults = (ele: FiddleSk) => {
    if (!ele._options.textOnly) {
      return html``;
    }
    return html`
      <h2 ?hidden=${ele._config.embedded}>Output</h2>

      <div>
        <div class="textoutput">
          <test-src-sk src=${ele.textURL()}></test-src-sk>
        </div>
        <div ?hidden=${!ele._config.embedded}>
          <button class="action" @click=${ele.run}>Run</button>
          <spinner-sk></spinner-sk>
          <a
            href="https://fiddle.skia.org/c/${ele._runResults.fiddleHash}"
            target="_blank"
            >Pop-out</a
          >
        </div>
      </div>
    `;
  };

  private static runDetails = (ele: FiddleSk) => {
    if (ele._config.embedded || !ele.hasImages()) {
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
      <div id="results">
        <div ?hidden=${!ele.showCPU()}>
          <img
            ?hidden=${ele._options.animated}
            title="CPU"
            src="${ele._config.domain}/i/${ele._runResults
              .fiddleHash}_raster.png"
            width=${ele._options.width}
            height=${ele._options.height}
          />
          <video
            ?hidden=${!ele._options.animated}
            title="CPU"
            @ended=${ele.playEnded}
            autoplay
            loop=${ele._config.loop}
            src="${ele._config.domain}/i/${ele._runResults.fiddleHash}_cpu.webm"
            width=${ele._options.width}
            height=${ele._options.height}
          >
          </video>
          <p ?hidden=${ele._config.embedded}> CPU </p>
        </div>

        <div ?hidden=${!ele.showGPU()}>
          <img
            ?hidden=${ele._options.animated}
            title="GPU"
            src="${ele._config.domain}/i/${ele._runResults.fiddleHash}_gpu.png"
            width=${ele._options.width}
            height=${ele._options.height}
          />
          <video
            ?hidden=${!ele._options.animated}
            title="GPU"
            loop=${ele._config.loop}
            autoplay
            src="${ele._config.domain}/i/${ele._runResults.fiddleHash}_gpu.webm"
            width=${ele._options.width}
            height=${ele._options.height}
          ></video>
          <p ?hidden=${ele._config.embedded}> GPU </p>
        </div>

        <div ?hidden=${!ele.showLinks()}>
          <div>
            <a href="${ele._config.domain}/i/${ele._runResults.fiddleHash}.pdf"
              >PDF</a
            >
          </div>
          <div>
            <a href="${ele._config.domain}/i/${ele._runResults.fiddleHash}.skp"
              >SKP</a
            >
          </div>
          <div>
            <a
              href="https://debugger.skia.org/loadfrom?url=https://fiddle.skia.org/i/${ele
                ._runResults.fiddleHash}.skp"
              >Debug</a
            >
          </div>
        </div>

        <div ?hidden="${!ele._config.embedded}">
          <button class="action" @click=${ele.run}>Run</button>
          <spinner-sk></spinner-sk>
          <a
            href="https://fiddle.skia.org/c/${ele._runResults.fiddleHash}"
            target="_blank"
            >Pop-out</a
          >
        </div>

        <div ?hidden=${!ele._options.animated} id="controls">
          <button @click=${ele.playToggle} title="Play the animation."
            >Play
            <!-- toggle Play/Pause -->
          </button>
          <checkbox-sk
            id="loop"
            ?checked=${ele._config.loop}
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

  private static errors = (ele: FiddleSk) => html`
    <div @click=${ele.errClick} ?hidden=${!ele.hasCompileWarningsOrErrors()}>
      <h2>Compilation Warnings/Errors</h2>
      ${ele._runResults.compile_errors!.map(
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
  <div ?hidden=${!ele._runResults.runtime_error}>
    <h2>Runtime Errors</h2>
    <div>${ele._runResults.runtime_error}</div>
  </template>
  `;

  private static template = (ele: FiddleSk) =>
    html`
      ${FiddleSk.displayOptions(ele)}

      <textarea-numbers-sk .value=${ele._runResults.text}></textarea-numbers-sk>

      ${FiddleSk.actions(ele)} ${FiddleSk.errors(ele)} ${FiddleSk.results(ele)}
    `;

  // Private properties.

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

  private _config: Config = {
    display_options: true,
    embedded: false,
    options_open: false,
    basic_mode: false,
    domain: 'fiddle.skia.org',
    bug_link: false,
    sources: [],
    loop: true,
  };

  private _runResults: RunResults = {
    text: '',
    fiddleHash: '',
    runtime_error: '',
    compile_errors: [],
  };

  private textarea: TextareaNumbersSk | null = null;

  // Properties

  /** @prop runResults The results from running a fiddle. */
  get runResults() {
    return this._runResults;
  }
  set runResults(val) {
    this.textarea!.clearErrors();
    this._runResults = val;
    this._render();
    val.compile_errors!.forEach((err) => {
      if (err.line === 0) {
        return;
      }
      this.textarea?.setErrorLine(err.line);
    });
  }

  /** @prop options The options for the fiddle. */
  get options() {
    return this._options;
  }
  set options(val) {
    this._options = val;
    this._render();
  }

  /** @prop config The configuration of the element.  */
  get config() {
    return this._config;
  }
  set config(val) {
    this._config = val;
    this._render();
  }

  // Event listeners.
  private f16Change(e: Event) {
    this._options.f16 = ((e.target as unknown) as CheckOrRadio).checked;
    this._render();
  }

  private textOnlyChange(e: Event) {
    this._options.textOnly = ((e.target as unknown) as CheckOrRadio).checked;
  }

  private srgbChange(e: Event) {
    this._options.srgb = ((e.target as unknown) as CheckOrRadio).checked;
    this._render();
  }

  private loopClick() {
    throw new Error('Method not implemented.');
  }
  private speed() {
    throw new Error('Method not implemented.');
  }
  private playToggle() {
    throw new Error('Method not implemented.');
  }
  private playEnded() {
    throw new Error('Method not implemented.');
  }

  private hasImages(): boolean {
    return (
      !this._options.textOnly &&
      this._runResults.fiddleHash != '' &&
      this._runResults.runtime_error === '' &&
      !this.hasCompileErrors()
    );
  }

  private hasCompileErrors() {
    return this._runResults.compile_errors!.some((e) =>
      e.text.includes('error:')
    );
  }

  private hasCompileWarningsOrErrors(): boolean {
    return this._runResults!.compile_errors!.length > 0;
  }
  private errClick() {
    throw new Error('Method not implemented.');
  }
  private bugLink(fiddleHash: string): string {
    var comment =
      'Visit this link to see the issue on fiddle:\n\n https://fiddle.skia.org/c/' +
      fiddleHash;
    return (
      'https://bugs.chromium.org/p/skia/issues/entry?' +
      fromObject({
        comment: comment,
      })
    );
    throw new Error('Method not implemented.');
  }
  private run() {
    throw new Error('Method not implemented.');
  }
  private animatedClick() {
    throw new Error('Method not implemented.');
  }
  private offscreenTexturableClick() {
    throw new Error('Method not implemented.');
  }
  private offscreenClick() {
    throw new Error('Method not implemented.');
  }
  private glinfoURL(): string {
    throw new Error('Method not implemented.');
  }
  private textURL(): unknown {
    throw new Error('Method not implemented.');
  }
  private showCPU(): boolean {
    throw new Error('Method not implemented.');
  }
  private showGPU(): boolean {
    throw new Error('Method not implemented.');
  }
  private showLinks(): boolean {
    throw new Error('Method not implemented.');
  }

  constructor() {
    super(FiddleSk.template);
    console.log('fiddle-sk constructor');
  }

  connectedCallback() {
    super.connectedCallback();
    console.log('fiddle-sk connectedCallback');
    this._render();
    this.textarea = this.querySelector('textarea-numbers-sk');
    this._upgradeProperty('options');
    this._upgradeProperty('config');
    this._upgradeProperty('runResults');
  }
}

define('fiddle-sk', FiddleSk);
