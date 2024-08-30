/**
 * @module skottie-exporter-base-sk
 * @description <h2><code>skottie-exporter-base-sk</code></h2>
 *
 * <p>
 *   A Base class for exporters
 * </p>
 *
 */
import { html, TemplateResult } from 'lit/html.js';
import { define } from '../../../../elements-sk/modules/define';
import { ElementSk } from '../../../../infra-sk/modules/ElementSk';
import { $$ } from '../../../../infra-sk/modules/dom';
import '../../skottie-dropdown-sk';
import { DropdownSelectEvent } from '../../skottie-dropdown-sk/skottie-dropdown-sk';
import { SkottiePlayerSk } from '../../skottie-player-sk/skottie-player-sk';
import '../../../../elements-sk/modules/icons/info-icon-sk';
import '../../skottie-button-sk';

export type Quality = 'low' | 'medium' | 'high';

export interface ExportConfig {
  quality: Quality;
}

export type ComponentState = 'idle' | 'running' | 'complete';

type ProgressData = {
  ratio: number;
  message?: string;
  details?: string;
};
type LogData = {
  type: string;
  message: string;
};

export class SkottieExporterBaseSk extends ElementSk {
  protected _config: ExportConfig = {
    quality: 'low',
  };

  protected player: SkottiePlayerSk | null = null;

  protected _renderState: ComponentState = 'idle';

  protected _blobData: string = '';

  protected _extension = '';

  protected _downloadFileName: string = 'download';

  protected progress: ProgressData = {
    ratio: 0,
    message: '',
    details: '',
  };

  private static template = (ele: SkottieExporterBaseSk) => html`
    <div class="container">
      <div class="wrapper-exporter">${ele.renderMain()}</div>
    </div>
  `;

  constructor(extension: string) {
    super(SkottieExporterBaseSk.template);
    // These methods are binded to the class since they are attached to event handlers
    // but they need to be executed in the context of this class and not the event target.
    this.updateProgress = this.updateProgress.bind(this);
    this.updateLog = this.updateLog.bind(this);
    this._extension = extension;
    this._downloadFileName = `download.${this._extension}`;
  }

  protected updateProgress(progress: ProgressData): void {
    if (progress.ratio) {
      this.progress.ratio = progress.ratio;
    }
    this._render();
  }

  protected updateLog(log: LogData): void {
    if (log.type === 'info' || log.type === 'fferr' || log.type === 'ffout') {
      // Showing a partial message for now
      this.progress.details = log.message.substr(0, 40);
    }
    this._render();
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
  }

  protected buildTitle(text: string): TemplateResult {
    return html`
      <div class="header">
        <h2>${text}</h2>
      </div>
    `;
  }

  protected renderIdle(): TemplateResult {
    return html``;
  }

  protected renderRunning(): TemplateResult {
    return html``;
  }

  private renderComplete(): TemplateResult {
    return html`
      <div>
        <a
          id="asset-download"
          href=${this._blobData}
          download=${this._downloadFileName}>
          Click here if your download didn't start automatically
        </a>
        <div class="navigation">
          <skottie-button-sk
            type="filled"
            @select=${this.cancel}
            .content=${'Close'}
            .classes=${['navigation__button']}></skottie-button-sk>
        </div>
      </div>
    `;
  }

  private renderMain(): TemplateResult {
    switch (this.renderState) {
      case 'running':
        return this.renderRunning();
      case 'complete':
        return this.renderComplete();
      case 'idle':
      default:
        return this.renderIdle();
    }
  }

  // Stores filename and extension that will be used for download.
  // It makes sure the file has an extension and matches the file tyle.
  protected updateFileName(name: string, extension: string): void {
    const dottedExtension = `.${extension}`;
    if (name.indexOf('.') !== -1) {
      const currentExtension = name.substr(name.lastIndexOf('.'));
      name = name.replace(currentExtension, dottedExtension);
    } else {
      name += dottedExtension;
    }
    this._downloadFileName = name;
    this._extension = extension;
  }

  protected onNameChange(ev: Event): void {
    const target = ev.target as HTMLInputElement;
    this.updateFileName(target.value, this._extension);
    (ev.target as HTMLInputElement).value = this._downloadFileName;
    this._render();
  }

  protected qualitySelectHandler(e: CustomEvent<DropdownSelectEvent>): void {
    this._config.quality = e.detail.value as Quality;
    this._render();
  }

  protected async start(): Promise<void> {
    if (this.player) {
      this.renderState = 'running';
    }
  }

  protected async stop(): Promise<void> {
    this.renderState = 'idle';
    this._render();
  }

  protected complete(data: string): void {
    if (data && this.renderState === 'running') {
      this._blobData = data;
      this.renderState = 'complete';
      this._render();
      const downloadLink = $$('#asset-download') as HTMLAnchorElement;
      downloadLink.click();
    }
  }

  protected cancel(): void {
    this.dispatchEvent(
      new CustomEvent('cancel', {
        bubbles: true,
      })
    );
  }

  protected handleKeyDown(ev: Event): void {
    if ((ev as KeyboardEvent).key === 'Enter') {
      ev.preventDefault();
      ev.stopImmediatePropagation();
      if (document.activeElement) {
        (document.activeElement as HTMLElement).blur();
      }
      this.start();
    }
  }

  set skottiePlayer(player: SkottiePlayerSk) {
    this.player = player;
    this._render();
  }

  set renderState(value: ComponentState) {
    this._renderState = value;
  }

  public get renderState(): ComponentState {
    return this._renderState;
  }

  set downloadFileName(value: string) {
    this.updateFileName(value, this._extension);
  }
}

define('skottie-exporter-base-sk', SkottieExporterBaseSk);
