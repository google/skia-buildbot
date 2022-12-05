/**
 * @module particles-sk
 * @description <h2><code>particles-player-sk</code></h2>
 *
 * <p>
 *   Handles the bulk of the work displaying Particles.
 * </p>
 *
 */
import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { html, TemplateResult } from 'lit-html';
import type {
  Particles, CanvasKit, Surface, Canvas, CanvasKitInit as CKInit,
} from 'canvaskit-wasm';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

// It is assumed that canvaskit.js has been loaded and this symbol is available globally.
declare const CanvasKitInit: typeof CKInit;

const DEFAULT_SIZE = 256;
const ZOOM_IN_FACTOR = 1.1; // 10%
const ZOOM_OUT_FACTOR = 1 / ZOOM_IN_FACTOR;

export interface PlayerConfig {
  body: any;
  width: number;
  height: number;
}

// This element might be loaded from a different site, and that means we need
// to be careful about how we construct the URL back to the canvas.wasm file.
// Start by recording the script origin.
const scriptOrigin = new URL((document!.currentScript as HTMLScriptElement).src).origin;
const kitReady = CanvasKitInit({
  locateFile: (file: any) => `${scriptOrigin}/dist/${file}`,
});

/**
 * Information needed to construct a single HTML control for a uniform. Note
 * that some uniforms actually represent more than one control, such as a
 * 'float3', in which case code will need to create three instances of
 * UniformControl.
*/
interface UniformControl {
  id: string;
  uniformSlot: number;
}

interface Point {
  x: number;
  y: number;
}

export function floatSlider(uniform: UniformControl | null): TemplateResult {
  if (!uniform) {
    return html``;
  }
  return html` <div class="widget">
    <input
      name=${uniform.id}
      id=${uniform.id}
      min="0"
      max="1"
      step="0.00001"
      type="range"
    />
    <label for=${uniform.id}>${uniform.id}</label>
  </div>`;
}

export class ParticlesPlayerSk extends ElementSk {
  private sliders: UniformControl[] = [];

  private zoomLevel: number = 1.0;

  private kit: CanvasKit | null = null; // CanvasKit instance

  private animation: Particles | null = null; // Particles instance

  private surface: Surface | null = null; // Surface

  private canvas: Canvas | null = null; // Cached Canvas (surface.getCanvas()).

  private time: number = 0;

  private lastTime: number = 0;

  private lastDrag: Point | null = null;

  constructor() {
    super(ParticlesPlayerSk.template);
  }

  private static template = (ele: ParticlesPlayerSk) => html`
    <div class="container">
      ${ele.sliders.map(floatSlider)}
      <canvas
        id="player"
        @wheel=${ele.wheelHandler}
        @mousemove=${ele.dragHandler}
        width=${ele.width * window.devicePixelRatio}
        height=${ele.height * window.devicePixelRatio}
        style="width: ${ele.width}px; height: ${ele.height}px;"
      >
        Your browser does not support the canvas tag.
      </canvas>
    </div>`;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  attributeChangedCallback(): void {
    this._render();
  }

  initialize(config: PlayerConfig): Promise<void> {
    this.width = config.width;
    this.height = config.height;
    this._render();

    return kitReady.then((ck: CanvasKit) => {
      this.kit = ck;
      try {
        this._initializeParticles(config.body);
      } catch (error) {
        errorMessage(error as Error);
      }
      this._render();
    });
  }

  isPlaying(): boolean {
    return !this.paused;
  }

  play(): void {
    if (!this.isPlaying()) {
      this.paused = false;
    }
    this._render();
  }

  pause(): void {
    if (this.isPlaying()) {
      this.paused = true;
    }
  }

  resetView(): void {
    const ck = this.kit;
    const canvas = this.canvas;
    // Reset to identity
    const tt = canvas!.getTotalMatrix();
    const itt = ck!.Matrix.invert(tt)!;
    canvas!.concat(itt);
    // Zoom to the middle of the animation
    canvas!.translate(this.width / 2, this.height / 2);
    this.zoomLevel = 1.0;
  }

  restartAnimation(): void {
    this.time = 0;
    this.lastTime = 0;
  }

  private dragHandler(e: MouseEvent) {
    if (!e.buttons) {
      this.lastDrag = null;
      return;
    }
    if (this.lastDrag) {
      const dx = e.clientX - this.lastDrag.x;
      const dy = e.clientY - this.lastDrag.y;

      this.canvas!.translate(dx / this.zoomLevel, dy / this.zoomLevel);
    }
    this.lastDrag = {
      x: e.clientX,
      y: e.clientY,
    };
  }

  private drawFrame() {
    if (!this.animation || !this.canvas) {
      return;
    }

    // Go through all the sliders on the page that we created and poll those inputs for their
    // value. Plug those values (range [0.0, 1.0]) into the uniforms.
    const uniforms = this.animation.uniforms();
    this.sliders.forEach((slider) => {
      const s = $$<HTMLInputElement>(`input#${slider.id}`, this);
      if (!s) {
        return;
      }
      uniforms[slider.uniformSlot] = s.valueAsNumber;
    });
    window.requestAnimationFrame(() => this.drawFrame());
    if (!this.lastTime) {
      this.animation.start(0, true);
      this.lastTime = Date.now();
    }

    if (this.isPlaying()) {
      this.time += Date.now() - this.lastTime;
    }
    this.lastTime = Date.now();

    this.canvas.clear(this.kit!.BLACK);
    this.animation.update(this.time / 1000.0);
    this.animation.draw(this.canvas);
    this.surface!.flush();
  }

  private _initializeParticles(particlesJSON: any) {
    // Rebuild the surface only if needed.
    if (
      !this.surface
      || (this.surface!.width() !== this.width)
      || (this.surface!.height() !== this.height)
    ) {
      this._render();

      // eslint-disable-next-line no-unused-expressions
      this.surface?.delete();
      const canvasEle = $$<HTMLCanvasElement>('#player', this)!;
      this.surface = this.kit!.MakeCanvasSurface(canvasEle);
      if (!this.surface) {
        throw new Error('Could not make SkSurface.');
      }
      // We don't need to call .delete() on the canvas because
      // the parent surface will do that for us.
      this.canvas = this.surface.getCanvas();
    }

    // eslint-disable-next-line no-unused-expressions
    this.animation?.delete();

    this.animation = this.kit!.MakeParticles(
      JSON.stringify(particlesJSON),
    );
    if (!this.animation) {
      throw new Error('Could not parse Particles JSON.');
    }

    // Go through all uniforms this animation has and look for those with the
    // prefix 'slider_' For those uniforms, we will make a slider on the UI and
    // then every frame, we will poll those inputs for their value and plug the
    // values into the uniforms. The sliders will be in range [0.0, 1.0]. Note
    // that the matrices are column major.

    // TODO(jcgregorio) Group rows together on the display so matrices look like
    // matrices.

    // TODO(jgrergorio) If the name contains "color" then either display a color
    // picker or at the very least change the postfixes to _r, _g_, and _b.

    // TODO(jcgregorio) Break out the uniforms handling into its own element
    // to be re-used on shaders.skia.org.
    this.sliders = [];
    const an = this.animation;
    for (let i = 0; i < an.getUniformCount(); i++) {
      const name = an.getUniformName(i);
      if (name.startsWith('slider_')) {
        const uniform = an.getUniform(i);
        for (let row = 0; row < uniform.rows; row++) {
          for (let col = 0; col < uniform.columns; col++) {
            let id = `${name.substring('slider_'.length)}`;
            if (uniform.rows > 1) {
              id += `_${row}`;
            }
            if (uniform.columns > 1) {
              id += `_${col}`;
            }
            this.sliders.push({
              id: id,
              uniformSlot: uniform.slot + row + col * uniform.rows,
            });
          }
        }
      }
    }

    this._render();
    this.canvas!.clear(this.kit!.BLACK);
    this.resetView();
    this.restartAnimation();
    this.drawFrame();
  }

  private wheelHandler(e: WheelEvent) {
    e.preventDefault();
    e.stopPropagation();

    let zoom = 0;
    if (e.deltaY < 0) {
      zoom = ZOOM_IN_FACTOR;
    } else {
      zoom = ZOOM_OUT_FACTOR;
    }
    this.zoomLevel *= zoom;
    const ck = this.kit;
    const canvas = this.canvas;

    const tt = canvas!.getTotalMatrix();
    const itt = ck!.Matrix.invert(tt)!;
    const pts = [e.clientX, e.clientY];
    ck!.Matrix.mapPoints(itt, pts); // Transform DOM pts into canvas space

    const matr = ck!.Matrix.scaled(zoom, zoom, pts[0], pts[1]);
    canvas!.concat(matr);
  }

  static get observedAttributes(): string[] {
    return ['width', 'height', 'paused'];
  }

  get width(): number { return +(this.getAttribute('width') || DEFAULT_SIZE); }

  set width(val: number) { this.setAttribute('width', val.toFixed(0)); }

  get height(): number { return +(this.getAttribute('height') || DEFAULT_SIZE); }

  set height(val: number) { this.setAttribute('height', val.toFixed(0)); }

  get paused(): boolean { return this.hasAttribute('paused'); }

  set paused(val: boolean) {
    if (val) {
      this.setAttribute('paused', '');
    } else {
      this.removeAttribute('paused');
    }
  }
}

define('particles-player-sk', ParticlesPlayerSk);
