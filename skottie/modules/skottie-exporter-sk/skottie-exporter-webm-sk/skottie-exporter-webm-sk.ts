/**
 * @module skottie-exporter-webm-sk
 * @description <h2><code>skottie-exporter-webm-sk</code></h2>
 *
 * <p>
 *   The skottie WebM exporter. It uses a WASM version of ffmpeg.
 *   It snapshots a skottie animation frame by frame
 *   and stores them temporarily as pngs in a virtual ffmpeg file system.
 *   Finally it executes a ffmpeg command to convert those pngs into a WebM.
 *   ffmpeg documentation: https://ffmpeg.org/ffmpeg.html
 *   useful links:
 *   - https://slhck.info/video/2017/02/24/crf-guide.html
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
  crf: number;
  scale: number;
  passes: 'one-pass' | 'two-pass';
}

const qualityDetails: Record<Quality, Detail> = {
  low: {
    crf: 45,
    scale: 540,
    passes: 'one-pass',
  },
  medium: {
    crf: 23,
    scale: 540,
    passes: 'two-pass',
  },
  high: {
    crf: 10,
    scale: 1080,
    passes: 'two-pass',
  },
};

export class SkottieExporterWebMSk extends SkottieExporterBaseSk {
  private _ffmpeg;

  private _frameCollector: FrameCollectorType;

  constructor() {
    super('webm');
    this._ffmpeg = createFFmpeg({
      log: true,
      corePath: `${window.location.origin}/static/ffmpeg-core.js`,
    });
    this._ffmpeg.load();
    this._frameCollector = frameCollectorFactory(this._ffmpeg, (message) =>
      this.updateProgress({ ratio: 0, message })
    );
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
          <div class="detail__property">CRF</div>
          <div class="detail__value">${detail.crf}</div>
          <div class="detail__info">
            <info-icon-sk></info-icon-sk>
            <span class="detail__info__tooltip"
              >Constant Rate Factor<br />Lower is better quality</span
            >
          </div>
        </div>
        <div class="detail">
          <div class="detail__property">Passes</div>
          <div class="detail__value">${detail.passes}</div>
          <div class="detail__info">
            <info-icon-sk></info-icon-sk>
            <span class="detail__info__tooltip"
              >Number of passes <br />More passes is smaller filesize <br />but
              slower render times
            </span>
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
    return html`
      <form class="export-form" @keydown=${this.handleKeyDown}>
        <input
          type="text"
          .value=${this._downloadFileName}
          class="export-form__filename"
          @input=${this.onNameChange}
          @change=${this.onNameChange} />
        <div class="export-form__label">Quality</div>
        <skottie-dropdown-sk
          .name=${'webm-quality'}
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

  private async generate(
    player: SkottiePlayerSk,
    ffmpeg: FFmpeg
  ): Promise<Uint8Array | null> {
    const canvas = player.canvas();
    if (!canvas || this.renderState !== 'running') {
      return null;
    }
    const detail: Detail = qualityDetails[this._config.quality];
    let scaledWidth = 1080;
    let scaledHeight = 1080;
    if (canvas.width > canvas.height) {
      scaledHeight = Math.round((scaledWidth * canvas.height) / canvas.width);
    } else {
      scaledWidth = Math.round((scaledHeight * canvas.width) / canvas.height);
    }
    if (detail.passes === 'one-pass') {
      await ffmpeg.run(
        '-pattern_type',
        'glob',
        '-i', // input file url
        '*.png',
        '-codec:v',
        'libvpx-vp9',
        '-vf', // https://ffmpeg.org/ffmpeg.html#Simple-filtergraphs
        `scale=${scaledHeight}:${scaledWidth}`,
        '-b:v', // specifies the target (average) bit rate for the encoder to use
        '0', // needs to be set as 0 for the crf parameter to work
        '-crf', // https://slhck.info/video/2017/02/24/crf-guide.html
        `${detail.crf}`,
        '-f', // output format
        'webm',
        'out.webm'
      );
    } else {
      // This is a two step process to get better quality and smaller file sizes
      await ffmpeg.run(
        '-pattern_type',
        'glob',
        '-i', // input file url
        '*.png',
        '-codec:v',
        'libvpx-vp9',
        '-vf',
        `scale=${scaledHeight}:${scaledWidth}`,
        '-b:v', // specifies the target (average) bit rate for the encoder to use
        '0', // needs to be set as 0 for the crf parameter to work
        '-crf', // https://slhck.info/video/2017/02/24/crf-guide.html
        `${detail.crf}`,
        '-pass',
        '1',
        '-f',
        'null',
        '/null'
      );
      await ffmpeg.run(
        '-pattern_type',
        'glob',
        '-i', // input file url
        '*.png',
        '-codec:v',
        'libvpx-vp9',
        '-vf', // https://ffmpeg.org/ffmpeg.html#Simple-filtergraphs
        `scale=${scaledHeight}:${scaledWidth}`,
        '-b:v', // specifies the target (average) bit rate for the encoder to use
        '0', // needs to be set as 0 for the crf parameter to work
        '-crf', // https://slhck.info/video/2017/02/24/crf-guide.html
        `${detail.crf}`,
        '-pass',
        '2',
        '-f', // output format
        'webm',
        'out.webm'
      );
    }
    const data = ffmpeg.FS('readFile', 'out.webm');
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
    const data = await this.generate(player, ffmpeg);
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

define('skottie-exporter-webm-sk', SkottieExporterWebMSk);
