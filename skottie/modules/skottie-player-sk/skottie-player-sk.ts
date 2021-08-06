/**
 * @module skottie-player-sk
 * @description <h2><code>skottie-player-sk</code></h2>
 *
 * <p>
 *   Displays a CanvasKit-based Skottie animation and provides various controls.
 * </p>
 *
 */
import { $$ } from 'common-sk/modules/dom';
import 'elements-sk/icon/pause-icon-sk';
import 'elements-sk/icon/play-arrow-icon-sk';
import 'elements-sk/icon/settings-icon-sk';
import 'elements-sk/spinner-sk';
import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';
import { repeat } from 'lit-html/directives/repeat';
import {
  Canvas,
  CanvasKit, CanvasKitInitOptions, ColorProperty, ManagedSkottieAnimation, OpacityProperty, SoundMap, Surface,
} from 'canvaskit-wasm';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

// This is how we can bundle in our CanvasKit build at TOT, not the one from npm.
// eslint-disable-next-line @typescript-eslint/no-var-requires
const CanvasKitInit: (_: CanvasKitInitOptions)=> Promise<CanvasKit> = require('../../build/canvaskit/canvaskit.js');

function hexColor(c: number) {
  // eslint-disable-next-line no-bitwise
  const rgb = c & 0x00ffffff;
  return `#${rgb.toString(16).padStart(6, '0')}`;
}

function skRectIsEmpty(rect: Float32Array|null) {
  if (!rect) {
    return true;
  }
  return rect[2] <= rect[0] || rect[3] <= rect[1];
}

type Property = ColorProperty | OpacityProperty;

class PropList<T extends Property> {
  private readonly defaultVal: T;

  index: number = 0;

  list: T[]

  constructor(list: T[], defaultVal: T) {
    this.list = list;
    this.defaultVal = defaultVal;
  }

  current = (): T => (this.index >= this.list.length
    ? this.defaultVal
    : this.list[this.index]);

  empty = () => !this.list.length;
}

// TODO(kjlubick) replace after https://skia-review.googlesource.com/c/skia/+/437316 is deployed
interface AnimationSegment {
  name: string
  t0: number
  t1: number
}

export interface SkottiePlayerConfig {
  assets?: Record<string, ArrayBuffer>;
  fps: number;
  height: number;
  lottie: Record<string, unknown>;
  soundMap?: SoundMap;
  width: number;
}

function segmentLabel(s: AnimationSegment) {
  return `${s.name} [${s.t0.toFixed(2)} .. ${s.t1.toFixed(2)}]`;
}

// This element might be loaded from a different site, and that means we need
// to be careful about how we construct the URL back to the canvas.wasm file.
// Start by recording the script origin.
const currentScript = document.currentScript! as HTMLScriptElement;
const scriptOrigin = new URL(currentScript.src).origin;

const canvasReady: Promise<CanvasKit> = CanvasKitInit({
  locateFile: (file: string) => `${scriptOrigin}/static/${file}`,
});

export class SkottiePlayerSk extends ElementSk {
  private static template = (ele: SkottiePlayerSk): TemplateResult => {
    if (ele.loading) {
      return ele.loadingTemplate();
    }
    return ele.runningTemplate();
  }

  private runningTemplate = () => html`
<div class=container>
  <div class=wrapper>
    <canvas class=skottie-canvas id=skottie
            width=${this.width * window.devicePixelRatio}
            height=${this.height * window.devicePixelRatio}
            style='width: ${this.width}px; height: ${this.height}px; background-color: ${this.bgColor}'>
      Your browser does not support the canvas tag.
    </canvas>
    <div class=controls ?hidden=${!this.showControls}>
      <play-arrow-icon-sk @click=${this.onPlay} ?hidden=${!this.paused}></play-arrow-icon-sk>
      <pause-icon-sk @click=${this.onPause} ?hidden=${this.paused}></pause-icon-sk>
      <input type=range min=0 max=100 @input=${this.onScrub} @change=${this.onScrubEnd}
             class=skottie-player-scrubber>
      <settings-icon-sk @click=${this.onSettings}></settings-icon-sk>
    </div>
  </div>
  ${this.settingsTemplate()}
</div>`;

  private settingsTemplate = () => html`
<div class=skottie-player-settings-container ?hidden=${!this.showSettings}>
  <div class=skottie-player-settings-row>
    <div class=skottie-player-settings-label>Colors</div>
    <select id=color-prop-select class=skottie-player-property-select
            @input=${this.onPropertySelect} ?disabled=${this.colorProps.empty()}>
      ${repeat(this.colorProps.list, (c: ColorProperty) => c.key, (c: ColorProperty, index: number) => html`
        <option value=${index}>${c.key}</option>
      `)}
    <select>
    <input type=color class=skottie-player-picker id=color-picker
           value=${hexColor(this.colorProps.current().value)}
           @input=${this.onColorInput} ?disabled=${this.colorProps.empty()}>
    <hr class=skottie-player-settings-divider>
  </div>
  <div class=skottie-player-settings-row>
    <div class=skottie-player-settings-label>Opacity</div>
    <select id=opacity-prop-select class=skottie-player-property-select
            @input=${this.onPropertySelect} ?disabled=${this.opacityProps.empty()}>
      ${repeat(this.opacityProps.list, (o: OpacityProperty) => o.key, (o: OpacityProperty, index: number) => html`
        <option value=${index}>${o.key}</option>
      `)}
    <select>
    <input type=range min=0 max=100 class=skottie-player-picker id=opacity-picker
           value=${this.opacityProps.current().value}
           @input=${this.onOpacityInput} ?disabled=${this.opacityProps.empty()}>
    <hr class=skottie-player-settings-divider>
  </div>
  <div class=skottie-player-settings-row>
    <div class=skottie-player-settings-label>Segments</div>
    <select id=segment-prop-select class=skottie-player-property-select
            style='width: 100%' @input=${this.onPropertySelect}>
      ${repeat(this.animationSegments, (s: AnimationSegment) => s.name, (s: AnimationSegment, index: number) => html`
        <option value=${index}>${segmentLabel(s)}</option>
      `)}
    <select>
    <hr class=skottie-player-settings-divider>
  </div>
  <div class=skottie-player-settings-row>
    <input type=button value=Close @click=${this.onSettings}>
  </div>
</div>
`;

  private loadingTemplate = () => html`
<div class=player-loading title="Loading animation and engine."
     style='width: ${this.width}px; height: ${this.height}px;'>
  <div>Loading</div>
  <spinner-sk active></spinner-sk>
</div>`;

  private animation: ManagedSkottieAnimation | null = null; // Skottie Animation instance

  private _animationName: string = '';

  private animationSegments: AnimationSegment[] = []; // Selectable animation segments

  private bgColor: string = '#fff';

  private colorProps: PropList<ColorProperty> = new PropList([], { key: '', value: 0 });

  private context: number = 0; // CK context.

  private currentSegment: AnimationSegment = { name: '', t0: 0, t1: 1 };

  private height: number = 0;

  private kit: CanvasKit| null = null;// CanvasKit instance

  private loading: boolean = true;

  private nativeFPS: number = 0; // Animation fps.

  private opacityProps: PropList<OpacityProperty> = new PropList([], { key: '', value: 1 });

  private paused: boolean;

  private renderFPS: number = 0;

  private scrubPlaying: boolean = false; // Animation was playing when the user started scrubbing.

  private seekPoint: number = 0; // Normalized [0..1] animation progress.

  private showSettings: boolean;

  private showControls: boolean = false;

  private skcanvas: Canvas | null = null;// Cached SkCanvas (surface.getCanvas()).

  private surface: Surface | null = null;

  private timeOrigin: number = 0; // Animation start time (ms).

  private totalDuration: number = 0; // Animation duration (ms).

  private width: number = 0;

  constructor() {
    super(SkottiePlayerSk.template);

    this.paused = this.hasAttribute('paused');
    this.showSettings = (new URL(document.location.href)).searchParams.has('settings');
  }

  connectedCallback(): void {
    super.connectedCallback();
    const params = (new URL(document.location.href)).searchParams;
    this.width = this.hasAttribute('width') ? +this.getAttribute('width')! : 256;
    this.height = this.hasAttribute('height') ? +this.getAttribute('height')! : 256;
    this.showControls = params.has('controls');
    this.bgColor = params.has('bg') ? params.get('bg')! : '#fff';
    this._render();
  }

  initialize(config: SkottiePlayerConfig): Promise<void> {
    this.width = config.width;
    this.height = config.height;
    this.renderFPS = config.fps;
    this._animationName = config.lottie.nm as string;

    this._render();
    return canvasReady.then((ck: CanvasKit) => {
      // Set a large-ish decode cache limit to accommodate potentially large images.
      const CACHE_SIZE = 512 * 1024 * 1024;
      ck.setDecodeCacheLimitBytes(CACHE_SIZE);

      this.kit = ck;
      this.initializeSkottie(config.lottie, config.assets, config.soundMap);
      this._render();
    });
  }

  duration(): number {
    return this.totalDuration * (this.currentSegment.t1 - this.currentSegment.t0);
  }

  fps(): number {
    return this.nativeFPS;
  }

  animationName(): string {
    return this._animationName;
  }

  canvas(): HTMLCanvasElement | null {
    return this.querySelector<HTMLCanvasElement>('.skottie-canvas');
  }

  seek(t: number, forceRender: boolean = false): void {
    this.timeOrigin = (Date.now() - this.duration() * t);

    if (!this.isPlaying()) {
      // Force-draw a static frame when paused.
      this.updateSeekPoint();
      this.drawFrame(forceRender);
    }
  }

  isPlaying(): boolean {
    return !this.paused;
  }

  pause(): void {
    if (this.isPlaying()) {
      this.paused = true;
      // Save the exact/current seek point at pause time.
      this.updateSeekPoint();
    }
  }

  play(): void {
    if (!this.isPlaying()) {
      this.paused = false;
      // Shift timeOrigin to continue from where we paused.
      this.seek(this.seekPoint);
      this.drawFrame();
    }
  }

  initializeSkottie(lottieJSON: unknown, assets?: Record<string, ArrayBuffer>, soundMap?: SoundMap): void {
    if (!this.kit) {
      console.error('Could not load CanvasKit');
      return;
    }
    this.loading = false;

    // Rebuild the surface only if needed.
    if (!this.surface
         || this.surface.width() !== this.width
         || this.surface.height() !== this.height) {
      this._render();

      if (this.surface) {
        this.surface.delete();
      }
      const canvasEle = $$<HTMLCanvasElement>('#skottie', this)!;
      this.surface = this.kit.MakeCanvasSurface(canvasEle);
      if (!this.surface) {
        throw new Error('Could not make SkSurface.');
      }
      // We don't need to call .delete() on the canvas because
      // the parent surface will do that for us.
      this.skcanvas = this.surface.getCanvas();

      this.context = this.kit.currentContext();
    }

    if (this.animation) {
      this.animation.delete();
    }

    this.animation = this.kit.MakeManagedAnimation(
      JSON.stringify(lottieJSON), assets, '', soundMap,
    );
    if (!this.animation) {
      throw new Error('Could not parse Lottie JSON.');
    }

    this.totalDuration = this.animation.duration() * 1000;
    this.nativeFPS = this.animation.fps();
    this.seek(0);

    this.colorProps.list = this.animation.getColorProps();
    this.opacityProps.list = this.animation.getOpacityProps();
    this.animationSegments = [{ name: 'Full timeline', t0: 0, t1: 1 }]
      .concat(this.animation.getMarkers() as AnimationSegment[]);
    this.currentSegment = this.animationSegments[0];

    this._render(); // re-render for animation-dependent elements (properties, etc).

    this.drawFrame(true);
  }

  private updateSeekPoint(): void {
    // t is in animation segment domain.
    const t = ((Date.now() - this.timeOrigin) / this.duration()) % 1;

    // map to the global animation timeline
    this.seekPoint = this.currentSegment.t0
         + t * (this.currentSegment.t1 - this.currentSegment.t0);
    if (this.showControls) {
      const scrubber = this.querySelector<HTMLInputElement>('.skottie-player-scrubber');
      if (scrubber) {
        scrubber.value = String(this.seekPoint * 100);
      }
    }
  }

  private drawFrame(forceRender: boolean = false): void {
    if (!this.animation || !this.skcanvas || !this.kit || !this.surface) {
      return;
    }

    // When paused, the progress is fully controlled externally.
    if (this.isPlaying()) {
      this.updateSeekPoint();
      window.requestAnimationFrame(() => {
        this.drawFrame();
      });
    }

    let frame = (this.seekPoint * this.totalDuration * this.nativeFPS) / 1000;
    if (this.renderFPS > 0) {
      // When a render FPS is specified, quantize to the desired rate.
      const fpsScale = this.renderFPS / this.nativeFPS;
      frame = Math.trunc(frame * fpsScale) / fpsScale;
    }

    this.kit.setCurrentContext(this.context);
    const damage = this.animation.seekFrame(frame);
    // Only draw frames when the content changes.
    if (forceRender || !skRectIsEmpty(damage)) {
      const bounds = this.kit.LTRBRect(0, 0, this.width * window.devicePixelRatio,
        this.height * window.devicePixelRatio);
      this.animation.render(this.skcanvas, bounds);
      this.surface.flush();
    }
  }

  private onPlay(): void {
    this.play();
    this._render();
  }

  private onPause(): void {
    this.pause();
    this._render();
  }

  // This fires every time the user moves the scrub slider.
  private onScrub(e: Event): void {
    const target = (e.target as HTMLInputElement);
    this.seek(target.valueAsNumber / 100);

    // Pause the animation while dragging the slider.
    if (this.isPlaying()) {
      this.scrubPlaying = true;
      this.pause();
    }
  }

  // This fires when the user releases the scrub slider.
  private onScrubEnd(): void {
    if (this.scrubPlaying) {
      this.scrubPlaying = false;
      this.play();
    }
  }

  private onSettings(): void {
    this.showSettings = !this.showSettings;
    this._render();
  }

  private onPropertySelect(e: Event): void {
    const target = (e.target as HTMLInputElement);
    switch (target.id) {
      case 'color-prop-select':
        this.colorProps.index = target.valueAsNumber;
        this.querySelector<HTMLInputElement>('#color-picker')!.value = hexColor(this.colorProps.current().value);
        break;
      case 'opacity-prop-select':
        this.opacityProps.index = target.valueAsNumber;
        this.querySelector<HTMLInputElement>('#opacity-picker')!.value = String(this.opacityProps.current().value);
        break;
      case 'segment-prop-select':
        this.currentSegment = this.animationSegments[target.valueAsNumber];
        this.seek(0);
        this._render();
        break;
      default:
        console.warn('unknown property select', target);
        break;
    }
  }

  private onColorInput(e: Event): void {
    const val = (e.target as HTMLInputElement).value;
    const prop = this.colorProps.current();
    // TODO(kjlubick) Why is there this combination of ColorAsInt and Color?
    const r = parseInt(val.substring(1, 3), 16);
    const g = parseInt(val.substring(3, 5), 16);
    const b = parseInt(val.substring(5, 7), 16);
    prop.value = this.kit!.ColorAsInt(r, g, b,
      1.0); // Treat colors as fully opaque.

    this.animation!.setColor(prop.key, this.kit!.Color(r, g, b, 1.0));
    this._render();

    if (!this.isPlaying()) {
      this.drawFrame();
    }
  }

  private onOpacityInput(e: Event): void {
    const prop = this.opacityProps.current();
    prop.value = (e.target as HTMLInputElement).valueAsNumber;

    this.animation!.setOpacity(prop.key, prop.value);
    this._render();

    if (!this.isPlaying()) {
      this.drawFrame();
    }
  }
}

define('skottie-player-sk', SkottiePlayerSk);
