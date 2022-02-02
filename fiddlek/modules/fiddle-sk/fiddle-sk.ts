/**
 * @module modules/fiddle-sk
 * @description <h2><code>fiddle-sk</code></h2>
 *
 * Displays all the options and input to running fiddles, along with the output
 * if the fiddle has been run.
 *
 * This is essentially the entire fiddle UI, but subsets of it can also be used
 * in cases where we embed fiddles on https://skia.org.
 *
 * @event fiddle-success is emitted when a run of a fiddle has completed successfully.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import '../textarea-numbers-sk';
import 'elements-sk/checkbox-sk';
import 'elements-sk/select-sk';
import 'elements-sk/spinner-sk';
import 'elements-sk/icon/play-arrow-icon-sk';
import 'elements-sk/icon/pause-icon-sk';
import '../test-src-sk';
import { fromObject } from 'common-sk/modules/query';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk';
import { SelectSkSelectionChangedEventDetail } from 'elements-sk/select-sk/select-sk';
import { errorMessage } from 'elements-sk/errorMessage';
import 'elements-sk/styles/buttons';
import 'elements-sk/styles/select';
import { SpinnerSk } from 'elements-sk/spinner-sk/spinner-sk';
import { TextareaNumbersSk } from '../textarea-numbers-sk/textarea-numbers-sk';
import { Options, RunResults, FiddleContext } from '../json';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

// The type for the 'fiddle-success' CustomEvent detail.
export type FiddleSkFiddleSuccessEventDetail = string;

// Config represents the configuration for how the FiddleSk element appears.
export interface Config {
  // Should the options be displayed.
  display_options: boolean;

  // Is this control embedded in another page (as opposed to being on fiddle.skia.org.).
  embedded: boolean;

  // Should the CPU result be displayed in embedded mode.
  cpu_embedded: boolean;

  // Should the GPU result be displayed in embedded mode.
  gpu_embedded: boolean;

  // Should the options details be open, as opposed to just displaying the summary.
  options_open: boolean;

  // If true then the more esoteric options are removed.
  basic_mode: boolean;

  // The domain where fiddle is running. Includes scheme, e.g. https://fiddle.skia.org.
  domain: string;

  // Should a link to create a bug be displayed near the Run button.
  bug_link: boolean;

  // The ids of the possible input source images.
  sources: number[];

  // Should animated images continuously loop.
  loop: boolean;

  // If true then animations are playing, otherwise they're paused.
  play: boolean;
}

export class FiddleSk extends ElementSk {
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
    cpu_embedded: true,
    gpu_embedded: true,
    options_open: false,
    basic_mode: false,
    domain: 'https://fiddle.skia.org',
    bug_link: false,
    sources: [],
    loop: true,
    play: true,
  };

  private _runResults: RunResults = {
    text: '',
    fiddleHash: '',
    runtime_error: '',
    compile_errors: [],
  };

  private textarea: TextareaNumbersSk | null = null;

  private spinner: SpinnerSk | null = null;

  constructor() {
    super(FiddleSk.template);
  }

  private static displayOptions = (ele: FiddleSk) => (!ele._config.display_options
    ? html``
    : html`
          <details ?open=${ele._config.options_open}>
            <summary>Options</summary>
            <div id="options">
              <label>
                <input
                  type="number"
                  min="4"
                  max="2048"
                  placeholder="128"
                  .value=${ele._options.width}
                  @change=${ele.widthChange}
                />
                Width
              </label>

              <label>
                <input
                  type="number"
                  min="4"
                  max="2048"
                  placeholder="128"
                  .value=${ele._options.height}
                  @change=${ele.heightChange}
                />
                Height
              </label>

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

              <checkbox-sk
                id="animated"
                label="Animation"
                ?checked=${ele._options.animated}
                @change=${ele.animatedChange}
              ></checkbox-sk>

              <div ?hidden=${!ele._options.animated} id="animated-options">
                <label>
                  <input
                    type="number"
                    min="1"
                    max="300"
                    .value=${ele._options.duration}
                    ?disabled=${!ele._options.animated}
                    @change=${ele.durationChange}
                  />
                  Duration (seconds)
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
              <checkbox-sk
                id="offscreen"
                title="Create an offscreen render target on the GPU."
                label="Offscreen Render Target"
                ?checked=${ele._options.offscreen}
                @change=${ele.offscreenChange}
                ?hidden=${ele._config.basic_mode}
              >
              </checkbox-sk>
              <div ?hidden=${!ele._options.offscreen} id="offscreen-options">
                <label>
                  <input
                    type="number"
                    min="4"
                    max="2048"
                    value=${ele._options.offscreen_width}
                    @change=${ele.offscreenWidthChange}
                  />
                  Width
                </label>
                <label>
                  <input
                    type="number"
                    min="4"
                    max="2048"
                    value=${ele._options.offscreen_height}
                    @change=${ele.offscreenHeightChange}
                  />
                  Height
                </label>
                <label>
                  <input
                    type="number"
                    value=${ele._options.offscreen_sample_count}
                    @change=${ele.offscreenSampleCountChange}
                  />
                  Sample Count
                </label>
                <checkbox-sk
                  id="texturable"
                  label="Texturable"
                  title="The offscreen render target can be used as a texture."
                  ?checked=${ele._options.offscreen_texturable}
                  @change=${ele.offscreenTexturableChange}
                ></checkbox-sk>

                <div class="indent">
                  <checkbox-sk
                    id="offscreen_mipmap"
                    label="MipMap"
                    title="The offscreen render target can be used as a texture that is mipmapped."
                    ?checked=${ele._options.offscreen_mipmap}
                    ?disabled=${!ele._options.offscreen_texturable}
                    @change=${ele.offscreenMipMapChange}
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

              <h3>Optional source image</h3>
              <select-sk @selection-changed=${ele.sourceChange}>
                ${ele._config.sources.map((source) => html`
                    <img
                      width="64"
                      height="64"
                      ?selected=${source === ele._options.source}
                      name=${source}
                      src="${ele._config.domain}/s/${source}"
                      class="imgsrc"
                    />
                  `)}
              </select-sk>
              <div ?hidden=${!ele._options.source} class="offset">
                <checkbox-sk
                  label="MipMap"
                  title="The backEndTexture is mipmapped."
                  ?checked=${ele._options.source_mipmap}
                  @change=${ele.sourceMipMapChange}
                ></checkbox-sk>
                <h4>These globals are now defined:</h4>
                <pre class="source-select">
SkBitmap source;
sk_sp&lt;SkImage> image;
GrBackendTexture backEndTexture; // GPU Only.</pre
                >
              </div>
              <div class="offset">
                Note:<br/>
                <div class="notes">
                  Adding comments with SK_FOLD_START and SK_FOLD_END creates foldable code blocks.<br/>
                  These blocks will be folded by default and are useful for highlighting specific lines of code.<br/>
                  You can also use the keyboard shortcuts Ctrl+S and Ctrl+E in the code editor to set them.
                </div>
              </div>
            </div>
          </details>
        `);

  private static actions = (ele: FiddleSk) => html`
      <div id="submit">
        <button class="action" @click=${ele.run}>Run</button>
        <spinner-sk></spinner-sk>

        <a
          ?hidden=${!ele._config.embedded}
          href="https://fiddle.skia.org/c/${ele._runResults.fiddleHash}"
          target="_blank"
          >Pop-out</a
        >

        <a
          id="bug"
          ?hidden=${!ele._config.bug_link}
          target="_blank"
          href=${ele.bugReportingURL(ele._runResults.fiddleHash)}
          >File Bug</a
        >
        <details id="embed" ?hidden=${ele._config.basic_mode}>
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

      <div class="textoutput">
        <test-src-sk .src=${ele.textURL()}></test-src-sk>
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
        <test-src-sk id="glinfo" .src=${ele.glinfoURL()}></test-src-sk>
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
            class="result_image cpu"
            ?hidden=${ele._options.animated}
            title="CPU"
            src="${ele._config.domain}/i/${ele._runResults.fiddleHash}_raster.png"
            width=${ele._options.width}
            height=${ele._options.height}
          />
          <video
            ?hidden=${!ele._options.animated}
            title="CPU"
            @ended=${ele.playEnded}
            ?autoplay=${ele._config.play} muted
            ?loop=${ele._config.loop}
            src="${ele._config.domain}/i/${ele._runResults.fiddleHash}_cpu.webm"
            width=${ele._options.width}
            height=${ele._options.height}
          >
          </video>
          <p ?hidden=${ele._config.embedded}> CPU </p>
        </div>

        <div ?hidden=${!ele.showGPU()}>
          <img
            class="result_image gpu"
            ?hidden=${ele._options.animated}
            title="GPU"
            src="${ele._config.domain}/i/${ele._runResults.fiddleHash}_gpu.png"
            width=${ele._options.width}
            height=${ele._options.height}
          />
          <video
            ?hidden=${!ele._options.animated}
            title="GPU"
            ?loop=${ele._config.loop}
            ?autoplay=${ele._config.play} muted
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
              href="https://debugger.skia.org?url=https://fiddle.skia.org/i/${
                ele._runResults.fiddleHash
              }.skp"
              >Debug</a
            >
          </div>
        </div>

        <div ?hidden=${!ele._options.animated} id="controls">
          <button @click=${ele.playClick} title="Play the animation.">
            <play-arrow-icon-sk ?hidden=${
              ele._config.play
            }></play-arrow-icon-sk>
            <pause-icon-sk ?hidden=${!ele._config.play} ><pause-icon-sk>
          </button>
          <checkbox-sk
            id="loop"
            ?checked=${ele._config.loop}
            label="Loop"
            title="Run animations in a loop"
            @change=${ele.loopChange}
          ></checkbox-sk>
          <select id="speed" @change=${ele.speedChange} size="1">
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
    `;
  };

  private static errors = (ele: FiddleSk) => html`
    <div @click=${ele.compilerErrorLineClick} ?hidden=${!ele.hasCompileWarningsOrErrors()}>
      <h2>Compilation Warnings/Errors</h2>
      ${ele._runResults.compile_errors?.map(
    (err) => html`<pre
            class="compile-error ${err.line > 0 ? 'clickable' : ''}"
            data-line=${err.line}
            data-col=${err.col}
          >
${err.text}</pre
          >`,
  )}
  </div>
  <div ?hidden=${!ele._runResults.runtime_error}>
    <h2>Runtime Errors</h2>
    <div>${ele._runResults.runtime_error}</div>
  </template>
  `;

  private static template = (ele: FiddleSk) => html`
      ${FiddleSk.displayOptions(ele)}

      <textarea-numbers-sk .value=${ele._runResults.text}></textarea-numbers-sk>

      ${FiddleSk.actions(ele)}

      ${FiddleSk.errors(ele)}

      ${FiddleSk.results(ele)}

      ${FiddleSk.textOnlyResults(ele)}

      ${FiddleSk.runDetails(ele)}
    `;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.textarea = this.querySelector('textarea-numbers-sk');
    this.spinner = this.querySelector('spinner-sk');
    this._upgradeProperty('options');
    this._upgradeProperty('config');
    this._upgradeProperty('runResults');
    this._upgradeProperty('context');
  }

  // Properties

  /** The results from running a fiddle. */
  get runResults(): RunResults {
    return this._runResults;
  }

  set runResults(val: RunResults) {
    this.textarea!.clearErrors();
    this._runResults = val;
    this._render();
    if (!val.compile_errors) {
      return;
    }
    val.compile_errors.forEach((err) => {
      if (err.line === 0) {
        return;
      }
      this.textarea!.setErrorLine(err.line);
    });
  }

  /** The options for the fiddle. */
  get options(): Options {
    return this._options;
  }

  set options(val: Options) {
    this._options = val;
    this._render();
  }

  /** The configuration of the element.  */
  get config(): Config {
    return this._config;
  }

  set config(val: Config) {
    this._config = val;
    this._render();
  }

  set context(val: FiddleContext | null) {
    if (!val) {
      return;
    }

    this.options = val.options;
    this.runResults.text = val.code;
    this.runResults.fiddleHash = val.fiddlehash;
    this._render();
  }

  // Event listeners.

  private sourceMipMapChange(e: Event) {
    this._options.source_mipmap = (e.target as CheckOrRadio).checked;
  }

  private sourceChange(e: CustomEvent<SelectSkSelectionChangedEventDetail>) {
    this._options.source = this._config.sources[e.detail.selection];
    this._render();
  }

  private offscreenMipMapChange(e: Event) {
    this._options.offscreen_mipmap = (e.target as CheckOrRadio).checked;
  }

  private offscreenTexturableChange(e: Event) {
    this._options.offscreen_texturable = (e.target as CheckOrRadio).checked;
    this._render();
  }

  private offscreenSampleCountChange(e: Event) {
    this._options.offscreen_sample_count = +(e.target as HTMLInputElement)
      .value;
  }

  private offscreenWidthChange(e: Event) {
    this._options.offscreen_width = +(e.target as HTMLInputElement).value;
  }

  private offscreenHeightChange(e: Event) {
    this._options.offscreen_height = +(e.target as HTMLInputElement).value;
  }

  private offscreenChange(e: Event) {
    this._options.offscreen = (e.target as CheckOrRadio).checked;
    this._render();
  }

  private durationChange(e: Event) {
    this._options.duration = +(e.target as HTMLInputElement).value;
  }

  private animatedChange(e: Event) {
    this._options.animated = (e.target as CheckOrRadio).checked;
    this._render();
  }

  private widthChange(e: Event) {
    this._options.width = +(e.target as HTMLInputElement).value;
  }

  private heightChange(e: Event) {
    this._options.height = +(e.target as HTMLInputElement).value;
  }

  private f16Change(e: Event) {
    this._options.f16 = (e.target as CheckOrRadio).checked;
    this._render();
  }

  private textOnlyChange(e: Event) {
    this._options.textOnly = (e.target as CheckOrRadio).checked;
  }

  private srgbChange(e: Event) {
    this._options.srgb = (e.target as CheckOrRadio).checked;
    this._render();
  }

  private loopChange() {
    this._config.loop = !this._config.loop;
    this._render();
  }

  private speedChange(e: Event) {
    const speed = (e.target! as HTMLSelectElement).value;
    this.querySelectorAll<HTMLVideoElement>('video').forEach((ele) => {
      ele.playbackRate = +speed;
    });
  }

  private playClick() {
    this._config.play = !this._config.play;
    this._render();
    this.querySelectorAll<HTMLVideoElement>('video').forEach((e) => {
      if (this._config.play) {
        e.play();
      } else {
        e.pause();
      }
    });
  }

  private playEnded() {
    this._config.play = false;
    this._render();
  }

  private hasImages(): boolean {
    return (
      !this._options.textOnly
      && this._runResults.fiddleHash !== ''
      && this._runResults.runtime_error === ''
      && !this.hasCompileErrors()
    );
  }

  private hasCompileErrors() {
    return !!this._runResults.compile_errors?.some((e) => e.text.includes('error:'));
  }

  private hasCompileWarningsOrErrors(): boolean {
    return (this._runResults!.compile_errors?.length || 0) > 0;
  }

  private compilerErrorLineClick(e: MouseEvent) {
    const ele = e.target! as HTMLElement;
    if (ele.nodeName === 'PRE') {
      this.textarea!.setCursor(+ele.dataset!.line!, +ele.dataset!.col!);
    }
  }

  private bugReportingURL(fiddleHash: string): string {
    const comment = `Visit this link to see the issue on fiddle:\n\n https://fiddle.skia.org/c/${
      fiddleHash}`;
    return (
      `https://bugs.chromium.org/p/skia/issues/entry?${
        fromObject({
          comment: comment,
        })}`
    );
  }

  private async run() {
    try {
      this._runResults.fiddleHash = '';
      this.spinner!.active = true;
      const body: FiddleContext = {
        version: '',
        sources: '',
        fiddlehash: '',
        name: '',
        overwrite: false,
        fast: false,
        code: this.textarea!.value,
        options: this._options,
      };
      const request = await fetch(`${this._config.domain}/_/run`, {
        method: 'POST',
        body: JSON.stringify(body),
      });
      const results = (await jsonOrThrow(request)) as RunResults;

      // The .text field returned is empty, so repopulate it with the value we
      // stored in body.
      results.text = body.code;
      this.runResults = results;
      this._render();
      this.dispatchEvent(
        new CustomEvent<FiddleSkFiddleSuccessEventDetail>('fiddle-success', {
          detail: this._runResults.fiddleHash,
          bubbles: true,
        }),
      );
    } catch (error) {
      const ignoredPromise = errorMessage(error);
    } finally {
      this.spinner!.active = false;
    }
  }

  private glinfoURL(): string {
    if (this._runResults.fiddleHash === '') {
      return '';
    }
    return (
      `${this._config.domain}/i/${this._runResults.fiddleHash}_glinfo.txt`
    );
  }

  private textURL(): string {
    if (this._runResults.fiddleHash === '') {
      return '';
    }
    return `${this._config.domain}/i/${this._runResults.fiddleHash}.txt`;
  }

  private showCPU(): boolean {
    if (!this._config.embedded) {
      return true;
    }
    return this._config.cpu_embedded || !this._config.gpu_embedded;
  }

  private showGPU(): boolean {
    if (!this._config.embedded) {
      return true;
    }
    return this._config.gpu_embedded;
  }

  private showLinks(): boolean {
    return (
      !this._options.animated
      && !this._config.embedded
      && !this._config.basic_mode
    );
  }
}

define('fiddle-sk', FiddleSk);
