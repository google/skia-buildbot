/**
 * @module modules/shaders-app-sk
 * @description <h2><code>shaders-app-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import 'codemirror/mode/clike/clike'; // Syntax highlighting for c-like languages.
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk/ElementSk';
import { SKIA_VERSION } from '../../build/version';
import { errorMessage } from 'elements-sk/errorMessage';
import CodeMirror from 'codemirror';
import { $$ } from 'common-sk/modules/dom';
import { isDarkMode } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';
import type {
  Particles,
  CanvasKit,
  Surface,
  Canvas,
  RuntimeEffect,
  Paint,
} from '../../build/canvaskit/canvaskit.js';

import 'elements-sk/error-toast-sk';
import 'elements-sk/styles/buttons';
import '../../../infra-sk/modules/theme-chooser-sk';

// eslint-disable-next-line @typescript-eslint/no-var-requires
const CanvasKitInit = require('../../build/canvaskit/canvaskit.js');

// This element might be loaded from a different site, and that means we need
// to be careful about how we construct the URL back to the canvas.wasm file.
// Start by recording the script origin.
const scriptOrigin = new URL((document!.currentScript as HTMLScriptElement).src)
  .origin;
const kitReady = CanvasKitInit({
  locateFile: (file: any) => `${scriptOrigin}/dist/${file}`,
});

const DEFAULT_SIZE = 256;

const defaultShader = `uniform float2 in_origin;
uniform float4 in_color;
uniform float in_progress;
uniform float in_maxRadius;

float dist2(vec2 p0, vec2 pf){
  return sqrt((pf.x-p0.x)*(pf.x-p0.x)+(pf.y-p0.y)*(pf.y-p0.y));
}

float mod2(float a, float b) {
  return a - (b * floor(a/b));
}

float rand(vec2 src){
  return fract(sin(dot(src.xy,vec2(12.9898,78.233)))*43758.5453123);
}

half4 main(float2 p) {
  float fraction = in_progress;
  float maxDist = in_maxRadius*2;

  float2 fragCoord = sk_FragCoord.xy;

  float fragDist = dist2(in_origin,fragCoord.xy);
  float radius = fragDist  * fraction;
  float d = 0.;
  float circleRadius = maxDist * fraction;

  float colorVal = (fragDist - circleRadius) / maxDist;

  d = fragDist < circleRadius
      ? 1.-abs(colorVal * 3. * smoothstep(0., 1., fraction ))
      : 1.-abs(colorVal * 4.);
  d = smoothstep(0., 1., d );


  // random points
  float divider = 2.;

  float x = floor(fragCoord.x/ divider);
  float y = floor(fragCoord.y/ divider);


  float fps = 20.;
  float density = .95;
  d = rand(vec2(x, y)) > density
      ? d
      : d * .2;


  // random brightness change TODO
  d = d * rand(vec2(fraction, x * y));

  return vec4(in_color.rgb*d, d);//vec4(d, d, d, 1);
}
`;

export class ShadersAppSk extends ElementSk {
  private codeMirror: CodeMirror.Editor | null = null;

  private kit: CanvasKit | null = null; // CanvasKit instance

  private context: number = -1; // CanvasKit context.

  private surface: Surface | null = null; // Surface

  private canvas: Canvas | null = null; // Cached Canvas (surface.getCanvas()).

  private paint: Paint | null = null;

  private width: number = DEFAULT_SIZE;

  private height: number = DEFAULT_SIZE;

  private duration: number = 2000; // ms

  private startTime: number = 0;

  private effect: RuntimeEffect | null = null;

  constructor() {
    super(ShadersAppSk.template);
  }

  private static template = (ele: ShadersAppSk) => html`
    <header>
      <h2>SkSL Shaders</h2>
      <span>
        <a
          id="githash"
          href="https://skia.googlesource.com/skia/+show/${SKIA_VERSION}"
        >
          ${SKIA_VERSION.slice(0, 7)}
        </a>
        <theme-chooser-sk dark></theme-chooser-sk>
      </span>
    </header>
    <main>
      <canvas
        id="player"
        width=${ele.width * window.devicePixelRatio}
        height=${ele.height * window.devicePixelRatio}
        style="width: ${ele.width}px; height: ${ele.height}px;"
      >
        Your browser does not support the canvas tag.
      </canvas>
      <div id="codeEditor"></div>
    </main>
    <footer>
      <error-toast-sk></error-toast-sk>
    </footer>
  `;

  /** Returns the CodeMirror theme based on the state of the page's darkmode.
   *
   * For this to work the associated CSS themes must be loaded. See
   * textarea-numbers-sk.scss.
   */
  private static themeFromCurrentMode = () =>
    isDarkMode() ? 'base16-dark' : 'base16-light';

  connectedCallback(): void {
    super.connectedCallback();
    this._render();

    this.startTime = Date.now();

    this.codeMirror = CodeMirror($$<HTMLDivElement>('#codeEditor', this)!, {
      lineNumbers: true,
      mode: 'text/x-c++src',
      theme: ShadersAppSk.themeFromCurrentMode(),
      viewportMargin: Infinity,
    });

    // Listen for theme changes.
    document.addEventListener('theme-chooser-toggle', () => {
      this.codeMirror!.setOption('theme', ShadersAppSk.themeFromCurrentMode());
    });

    this.codeMirror.setValue(defaultShader);

    kitReady.then((ck: CanvasKit) => {
      this.kit = ck;
      try {
        this.init();
      } catch (error) {
        errorMessage(error);
      }
    });
  }

  private init() {
    this.startTime = Date.now();

    const canvasEle = $$<HTMLCanvasElement>('#player', this)!;
    this.surface = this.kit!.MakeCanvasSurface(canvasEle);
    if (!this.surface) {
      throw new Error('Could not make SkSurface.');
    }
    // We don't need to call .delete() on the canvas because
    // the parent surface will do that for us.
    this.canvas = this.surface.getCanvas();
    this.context = this.kit!.currentContext();
    this.effect = this.kit!.RuntimeEffect.Make(defaultShader);
    this.paint = new this.kit!.Paint();
    this.drawFrame();
  }

  private drawFrame() {
    this.kit!.setCurrentContext(this.context);

    // properties of ripple
    let origin = [this.width / 2, this.height / 2];
    let color = this.getColor();
    let maxRadius = this.width * 2;

    const uniforms = [...origin, ...color, this.playbackPosition, maxRadius];
    const shader = this.effect!.makeShader(uniforms);

    this.canvas!.clear(this.kit!.BLACK);

    this.paint!.setShader(shader);
    const rect = this.kit!.XYWHRect(0, 0, this.width, this.height);
    this.canvas!.drawRect(rect, this.paint!);
    this.surface!.flush();

    requestAnimationFrame(() => {
      this.drawFrame();
    });
  }

  /** The position in [0, 1] where we are in the playback.
   *
   * The value returned depedns on Date.now() and this.startTime.
   */
  get playbackPosition() {
    const elapsedTime = Date.now() - this.startTime;
    let playbackPosition = elapsedTime / this.duration;
    if (playbackPosition > 1) {
      // Make sure we hit the end frame, but set us up to start at the beginning
      // on the next frame.
      playbackPosition = 1;
      this.startTime = Date.now();
    }
    return playbackPosition;
  }

  getColor() {
    // color is in #RRGGBB form
    // https://developer.mozilla.org/en-US/docs/Web/HTML/Element/input/color
    return this.kit!.parseColorString('#DC0000');
  }
}

define('shaders-app-sk', ShadersAppSk);
