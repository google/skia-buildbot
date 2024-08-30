/**
 * @module skottie-exporter-sk
 * @description <h2><code>skottie-exporter-sk</code></h2>
 *
 * <p>
 *   A modal component that manages all exporting options for Skottie animations.
 * </p>
 *
 */
import { html, TemplateResult } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../skottie-dropdown-sk';
import './skottie-exporter-gif-sk';
import './skottie-exporter-webm-sk';
import './skottie-exporter-png-sk';
import { SkottiePlayerSk } from '../skottie-player-sk/skottie-player-sk';

export type ExportType = 'none' | 'gif' | 'png' | 'webM';

export class SkottieExporterSk extends ElementSk {
  private _exportType: ExportType = 'none';

  private _skottiePlayer: SkottiePlayerSk | null = null;

  private _downloadFileName: string = '';

  private static template = (ele: SkottieExporterSk) => {
    if (ele._exportType === 'none') {
      return null;
    }
    return html`
      <aside class="wrapper">
        <div class="background"></div>
        <div class="main">${ele.buildExporter(ele)}</div>
      </aside>
    `;
  };

  constructor() {
    super(SkottieExporterSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
  }

  private buildTitle(text: string): TemplateResult {
    return html`
      <div class="header">
        <h2 class="header__text">${text}</h2>
      </div>
    `;
  }

  private buildGifExporter(ele: SkottieExporterSk): TemplateResult {
    return html`
      <div class="modal">
        ${this.buildTitle('Export GIF')}
        <skottie-exporter-gif-sk
          @cancel=${ele.cancel}
          .skottiePlayer=${ele._skottiePlayer}
          .downloadFileName=${this._downloadFileName}></skottie-exporter-gif-sk>
      </div>
    `;
  }

  private buildWebMExporter(ele: SkottieExporterSk): TemplateResult {
    return html`
      <div class="modal">
        ${this.buildTitle('Export WebM')}
        <skottie-exporter-webm-sk
          @cancel=${ele.cancel}
          .skottiePlayer=${ele._skottiePlayer}
          .downloadFileName=${this._downloadFileName}>
        </skottie-exporter-webm-sk>
      </div>
    `;
  }

  private buildPNGExporter(ele: SkottieExporterSk): TemplateResult {
    return html`
      <div class="modal">
        ${this.buildTitle('Export PNG Sequence')}
        <skottie-exporter-png-sk
          @cancel=${ele.cancel}
          .skottiePlayer=${ele._skottiePlayer}
          .downloadFileName=${this._downloadFileName}></skottie-exporter-png-sk>
      </div>
    `;
  }

  private buildExporter(ele: SkottieExporterSk): TemplateResult | null {
    if (this._exportType === 'gif') {
      return this.buildGifExporter(ele);
    }
    if (this._exportType === 'webM') {
      return this.buildWebMExporter(ele);
    }
    if (this._exportType === 'png') {
      return this.buildPNGExporter(ele);
    }
    return null;
  }

  export(type: ExportType, skottiePlayer: SkottiePlayerSk): void {
    this._skottiePlayer = skottiePlayer;
    switch (type) {
      case 'gif':
      case 'webM':
      case 'png':
        this.exportType = type;
        break;
      default:
        this.exportType = 'none';
        break;
    }
  }

  private cancel(): void {
    this.exportType = 'none';
  }

  set exportType(value: ExportType) {
    this._exportType = value;
    this.dispatchEvent(new CustomEvent('start'));
    this._render();
  }

  set downloadFileName(value: string) {
    this._downloadFileName = value;
  }
}

define('skottie-exporter-sk', SkottieExporterSk);
