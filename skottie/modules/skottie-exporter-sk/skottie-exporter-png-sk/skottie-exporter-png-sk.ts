/**
 * @module skottie-exporter-png-sk
 * @description <h2><code>skottie-exporter-png-sk</code></h2>
 *
 * <p>
 *   The skottie PNG exporter.
 *   It will output either a single PNG file or a ZIP if it's more than one frame.
 * </p>
 *
 */
import { html, TemplateResult } from 'lit/html.js';
import JSZip from 'jszip';
import { define } from '../../../../elements-sk/modules/define';
import '../../skottie-dropdown-sk';
import {
  DropdownOption,
  DropdownSelectEvent,
} from '../../skottie-dropdown-sk/skottie-dropdown-sk';
import { SkottiePlayerSk } from '../../skottie-player-sk/skottie-player-sk';
import '../../../../elements-sk/modules/icons/info-icon-sk';
import '../../skottie-button-sk';
import {
  SkottieExporterBaseSk,
  Quality,
} from '../skottie-exporter-base-sk/skottie-exporter-base-sk';
import delay from '../../helpers/delay';

interface ExportConfig {
  quality: Quality;
  rangeStart: number;
  rangeEnd: number;
}

interface Detail {
  scale: number;
}

const qualityDetails: Record<Quality, Detail> = {
  low: {
    scale: 0.25,
  },
  medium: {
    scale: 0.5,
  },
  high: {
    scale: 1,
  },
};

export class SkottieExporterPNGSk extends SkottieExporterBaseSk {
  protected _config: ExportConfig = {
    quality: 'high',
    rangeStart: 0,
    rangeEnd: -1,
  };

  constructor() {
    super('zip');
  }

  _render() {
    this.updateRanges();
    super._render();
  }

  private updateRanges(): void {
    if (!this.player) {
      return;
    }
    const fps = this.player.fps();
    const duration = this.player.duration();
    const totalFrames = Math.round(duration * (fps / 1000));
    if (this._config.rangeEnd === -1 || this._config.rangeEnd > totalFrames) {
      this._config.rangeEnd = totalFrames - 1;
    }
  }

  private renderDetails(): TemplateResult | null {
    if (!this.player) {
      return null;
    }
    const detail: Detail = qualityDetails[this._config.quality];
    const canvas = this.player.canvas()!;
    const finalWidth = Math.round(canvas.width * detail.scale);
    const finalHeight = Math.round(canvas.height * detail.scale);
    return html`
      <article>
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
          .name=${'png-quality'}
          .options=${[
            {
              id: 'low',
              value: 'Low',
              selected: this._config.quality === 'low',
            },
            {
              id: 'medium',
              value: 'Medium',
              selected: this._config.quality === 'medium',
            },
            {
              id: 'high',
              value: 'High',
              selected: this._config.quality === 'high',
            },
          ]}
          @select=${this.qualitySelectHandler}
          full>
        </skottie-dropdown-sk>
        <div class="separator"></div>
        ${this.renderDetails()}
        <div class="separator"></div>
        <div class="label">Select range start</div>
        <skottie-dropdown-sk
          .name=${'range-start'}
          .options=${this.buildRangeStartOptions()}
          @select=${this.rangeStartHandler}
          full></skottie-dropdown-sk>
        <div class="label">Select range end</div>
        <skottie-dropdown-sk
          .name=${'range-end'}
          .options=${this.buildRangeEndOptions()}
          @select=${this.rangeEndHandler}
          full></skottie-dropdown-sk>
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

  private buildRangeStartOptions(): DropdownOption[] {
    let start = 0;
    const end = this._config.rangeEnd;
    const options = [];
    while (start <= end) {
      const option: DropdownOption = {
        id: start.toString(),
        value: `${start + 1}`,
      };
      if (start === this._config.rangeStart) {
        option.selected = true;
      }
      options.push(option);
      start += 1;
    }
    return options;
  }

  private buildRangeEndOptions(): DropdownOption[] {
    if (!this.player) {
      return [];
    }
    const options: DropdownOption[] = [];
    let start = this._config.rangeStart;
    const fps = this.player.fps();
    const duration = this.player.duration();
    const totalFrames = Math.round(duration * (fps / 1000));
    while (start < totalFrames) {
      const option: DropdownOption = {
        id: start.toString(),
        value: `${start + 1}`,
      };
      if (start === this._config.rangeEnd) {
        option.selected = true;
      }
      options.push(option);
      start += 1;
    }
    return options;
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

  private getExtension(): string {
    return this._config.rangeStart === this._config.rangeEnd ? 'png' : 'zip';
  }

  private rangeStartHandler(e: CustomEvent<DropdownSelectEvent>): void {
    this._config.rangeStart = parseInt(e.detail.value);
    this.updateFileName(this._downloadFileName, this.getExtension());
    this._render();
  }

  private rangeEndHandler(e: CustomEvent<DropdownSelectEvent>): void {
    this._config.rangeEnd = parseInt(e.detail.value);
    this.updateFileName(this._downloadFileName, this.getExtension());
    this._render();
  }

  private getOutputCanvas(canvasElement: HTMLCanvasElement): HTMLCanvasElement {
    let outputCanvas: HTMLCanvasElement;
    const detail: Detail = qualityDetails[this._config.quality];
    if (detail.scale !== 1) {
      outputCanvas = document.createElement('canvas');
      outputCanvas.width = Math.round(canvasElement.width * detail.scale);
      outputCanvas.height = Math.round(canvasElement.height * detail.scale);
    } else {
      outputCanvas = canvasElement;
    }
    return outputCanvas;
  }

  private async exportMultiFile(player: SkottiePlayerSk): Promise<string> {
    const fps = player.fps();
    const duration = player.duration();
    const canvasElement = player.canvas()!;
    let currentTime = (this._config.rangeStart * 1000) / fps;
    const endTime = (this._config.rangeEnd * 1000) / fps;
    let counter = 1;
    const increment = 1000 / fps;
    const zip = new JSZip();
    const fileName = this._downloadFileName.substr(
      0,
      this._downloadFileName.lastIndexOf('.')
    );
    const detail: Detail = qualityDetails[this._config.quality];
    const outputCanvas = this.getOutputCanvas(canvasElement);
    while (currentTime <= endTime) {
      if (this.renderState !== 'running') {
        return '';
      }
      // This delay helps to maintain the browser responsive during export.
      await delay(1); // eslint-disable-line no-await-in-loop
      player.seek(currentTime / duration, true);
      // Only copying canvas if it has a different output size
      if (detail.scale !== 1) {
        const outputCanvasContext = outputCanvas.getContext('2d');
        outputCanvasContext?.clearRect(
          0,
          0,
          outputCanvas.width,
          outputCanvas.height
        );
        outputCanvasContext?.drawImage(
          canvasElement,
          0,
          0,
          canvasElement.width,
          canvasElement.height,
          0,
          0,
          outputCanvas.width,
          outputCanvas.height
        );
      }
      // eslint-disable-next-line no-await-in-loop
      const blob: Blob | null = await new Promise((res) =>
        outputCanvas.toBlob(res)
      );
      if (blob) {
        const file = `${fileName}_${String(counter).padStart(4, '0')}.png`;
        zip.file(file, blob);
      }
      currentTime += increment;
      this.progress.ratio = 0;
      this.progress.message = `Creating frame ${counter}`;
      counter += 1;
      this._render();
    }
    const content = await zip.generateAsync({ type: 'base64' });
    return `data:application/zip;base64,${content}`;
  }

  private async exportSingleFile(player: SkottiePlayerSk): Promise<string> {
    const fps = player.fps();
    const duration = player.duration();
    const canvasElement = player.canvas()!;
    const currentTime = (this._config.rangeStart * 1000) / fps;
    player.seek(currentTime / duration, true);
    const detail: Detail = qualityDetails[this._config.quality];
    const outputCanvas = this.getOutputCanvas(canvasElement);
    // Only copying canvas if it has a different output size
    if (detail.scale !== 1) {
      const outputCanvasContext = outputCanvas.getContext('2d');
      outputCanvasContext?.clearRect(
        0,
        0,
        outputCanvas.width,
        outputCanvas.height
      );
      outputCanvasContext?.drawImage(
        canvasElement,
        0,
        0,
        canvasElement.width,
        canvasElement.height,
        0,
        0,
        outputCanvas.width,
        outputCanvas.height
      );
    }
    const blob: Blob | null = await new Promise((res) =>
      outputCanvas.toBlob(res)
    );
    if (blob) {
      return URL.createObjectURL(blob);
    }
    return '';
  }

  protected async start(): Promise<void> {
    if (!this.player) {
      return;
    }
    super.start();
    const player = this.player;
    let data;
    if (this._config.rangeStart === this._config.rangeEnd) {
      data = await this.exportSingleFile(player);
    } else {
      data = await this.exportMultiFile(player);
    }
    this.complete(data);
  }
}

define('skottie-exporter-png-sk', SkottieExporterPNGSk);
