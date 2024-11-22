/**
 * @module skottie-exporter-gif-sk
 * @description <h2><code>skottie-exporter-sk</code></h2>
 *
 * <p>
 *   The skottie Gif exporter. It uses a WASM version of ffmpeg.
 *   It snapshots a skottie animation frame by frame
 *   and stores them temporarily as pngs in a virtual ffmpeg file system.
 *   Finally it executes a ffmpeg command to convert those pngs into a GIF.
 *   ffmpeg documentation: https://ffmpeg.org/ffmpeg.html
 *   useful links:
 *   - http://blog.pkh.me/p/21-high-quality-gif-with-ffmpeg.html
 *   - https://tyhopp.com/notes/ffmpeg-crosshatch
 *   - https://superuser.com/questions/1231645
 * </p>
 *
 */
import { html, TemplateResult } from 'lit/html.js';
import '../../skottie-dropdown-sk';
import { createFFmpeg, FFmpeg } from '@ffmpeg/ffmpeg';
import { define } from '../../../../elements-sk/modules/define';
import { SkottiePlayerSk } from '../../skottie-player-sk/skottie-player-sk';
import '../../../../elements-sk/modules/icons/info-icon-sk';
import '../../skottie-button-sk';
import {
  SkottieExporterBaseSk,
  Quality,
  ComponentState,
} from '../skottie-exporter-base-sk/skottie-exporter-base-sk';
import frameCollectorFactory, {
  FrameCollectorType,
} from '../../helpers/frameCollectorFactory';

interface Detail {
  colors: number;
  fps: number;
  scale: number;
}

const qualityDetails: Record<Quality, Detail> = {
  low: {
    colors: 16,
    fps: 6,
    scale: 540,
  },
  medium: {
    colors: 256,
    fps: 12,
    scale: 540,
  },
  high: {
    colors: 256,
    fps: 12,
    scale: 1080,
  },
};

export class SkottieExporterGifSk extends SkottieExporterBaseSk {
  private _ffmpeg: FFmpeg;

  private _frameCollector: FrameCollectorType;

  private _isLoaded: boolean = false;

  constructor() {
    super('gif');
    this._ffmpeg = createFFmpeg({
      log: false,
      corePath: `${window.location.origin}/static/ffmpeg-core.js`,
    });
    this._frameCollector = frameCollectorFactory(this._ffmpeg, (message) =>
      this.updateProgress({ ratio: 0, message })
    );
    this.loadFfmpeg();
  }

  private async loadFfmpeg() {
    await this._ffmpeg.load();
    this._isLoaded = true;
    this._render();
  }

  private renderDetails(): TemplateResult | null {
    if (!this.player) {
      return null;
    }
    const detail: Detail = qualityDetails[this._config.quality];
    const canvas = this.player.canvas()!;
    const maxSize = detail.scale;
    let finalWidth = maxSize;
    let finalHeight = maxSize;
    if (canvas.width >= canvas.height) {
      finalHeight = Math.round((finalWidth * canvas.height) / canvas.width);
    } else {
      finalWidth = Math.round((finalHeight * canvas.width) / canvas.height);
    }
    return html`
      <article>
        <div class="detail">
          <div class="detail__property">Colors</div>
          <div class="detail__value">${detail.colors}</div>
          <div class="detail__info">
            <info-icon-sk></info-icon-sk>
            <span class="detail__info__tooltip">Total palette colors</span>
          </div>
        </div>
        <div class="detail">
          <div class="detail__property">FPS</div>
          <div class="detail__value">${detail.fps}</div>
          <div class="detail__info">
            <info-icon-sk></info-icon-sk>
            <span class="detail__info__tooltip">Frames per second</span>
          </div>
        </div>
        <div class="detail">
          <div class="detail__property">Exported image size</div>
          <div class="detail__value">${finalWidth}x${finalHeight}</div>
          <div class="detail__info">
            <info-icon-sk></info-icon-sk>
            <span class="detail__info__tooltip">Exported size</span>
          </div>
        </div>
      </article>
    `;
  }

  protected renderIdle(): TemplateResult {
    if (!this._isLoaded) {
      return html`<span id="export-form-gif">Loading ffmpeg...</span>`;
    }
    return html`
      <form
        class="export-form"
        id="export-form-gif"
        @keydown=${this.handleKeyDown}>
        <input
          type="text"
          .value=${this._downloadFileName}
          class="export-form__filename"
          @input=${this.onNameChange}
          @change=${this.onNameChange} />
        <div class="export-form__label">Quality</div>
        <skottie-dropdown-sk
          .name=${'gif-quality'}
          .options=${[
            { id: 'low', value: 'Low' },
            { id: 'medium', value: 'Medium' },
            { id: 'high', value: 'High' },
          ]}
          @select=${this.qualitySelectHandler}
          full>
        </skottie-dropdown-sk>
        <div class="separator"></div>
        ${this.renderDetails()}
        <div class="navigation">
          <skottie-button-sk
            type="plain"
            @select=${this.cancel}
            .content=${'Cancel'}
            .classes=${['navigation__button']}></skottie-button-sk>
          <skottie-button-sk
            type="filled"
            @select=${this.start}
            .content=${'Export'}
            .classes=${['navigation__button']}></skottie-button-sk>
        </div>
      </form>
    `;
  }

  private renderRatio(): TemplateResult {
    if (this.progress.ratio) {
      return html`<div class="running__message">
        ${(this.progress.ratio * 100).toFixed(1)} % complete
      </div>`;
    }
    return html`<div class="running__message">${this.progress.message}</div>`;
  }

  protected renderRunning(): TemplateResult {
    return html`
      <div class="running">
        ${this.renderRatio()}
        <div class="running__details">${this.progress.details}</div>
        <div class="navigation">
          <skottie-button-sk
            type="filled"
            @select=${this.stop}
            .content=${'Stop'}
            .classes=${['navigation__button']}></skottie-button-sk>
        </div>
      </div>
    `;
  }

  private buildFilters(fps: number, scale: string, colors: number): string {
    if (this._config.quality !== 'high') {
      return `fps=${fps},scale=${scale}:flags=lanczos,split[s0][s1];\
      [s0] palettegen=max_colors=${colors}, split[pal1][pal2];\
      [s1][pal1] paletteuse=bayer [s2];\
      [s2][pal2] paletteuse=dither=bayer`;
    }
    // For high quality we add a parameter to palettegen
    // stats_mode=single that generates a new palette per frame
    // better quality but file size will be larger
    return `fps=${fps},scale=${scale}:flags=lanczos,split[s0][s1];\
      [s0] palettegen=stats_mode=single:max_colors=${colors}, split[pal1][pal2];\
      [s1][pal1] paletteuse=bayer [s2];\
      [s2][pal2] paletteuse=dither=bayer`;
  }

  private async generateGIF(
    player: SkottiePlayerSk,
    ffmpeg: FFmpeg
  ): Promise<Uint8Array | null> {
    const canvas = player.canvas();
    if (!canvas || this.renderState !== 'running') {
      return null;
    }
    const detail: Detail = qualityDetails[this._config.quality];
    const scale =
      canvas.width >= canvas.height
        ? `${detail.scale}:-1`
        : `-1:${detail.scale}`;
    const fps = player.fps();
    await ffmpeg.run(
      '-framerate',
      `${fps}`,
      '-pattern_type',
      'glob',
      '-i', // input file url
      '*.png',
      '-pix_fmt',
      'pal8',
      '-loop',
      '0', // infinite loop
      '-codec:v',
      'gif',
      '-gifflags',
      '+transdiff',
      '-filter_complex',
      this.buildFilters(fps, scale, detail.colors),
      '-f', // output file format
      'gif',
      'out.gif'
    );
    const data = ffmpeg.FS('readFile', 'out.gif');
    return data;
  }

  protected async start(): Promise<void> {
    if (!this.player) {
      return;
    }
    super.start();
    const ffmpeg = this._ffmpeg;
    const player = this.player;
    this._frameCollector.player = player;
    ffmpeg.setLogger(this.updateLog);
    ffmpeg.setProgress(this.updateProgress);
    await this._frameCollector.start();
    this.updateProgress({ ratio: 0.01, message: '' });
    const data = await this.generateGIF(player, ffmpeg);
    this.complete(data ? URL.createObjectURL(new Blob([data])) : '');
  }

  get renderState() {
    return super.renderState;
  }

  set renderState(value: ComponentState) {
    super.renderState = value;
    if (this.renderState !== 'running') {
      this._frameCollector.stop();
    }
  }
}

define('skottie-exporter-gif-sk', SkottieExporterGifSk);
