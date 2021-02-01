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
import { $$ } from 'common-sk/modules/dom'
import { isDarkMode } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';

import 'elements-sk/error-toast-sk';
import 'elements-sk/styles/buttons';
import '../../../infra-sk/modules/theme-chooser-sk';

const defaultShader = `
uniform float2 in_origin;
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

  constructor() {
    super(ShadersAppSk.template);
  }

  private static template = (ele: ShadersAppSk) => html`
  <header>
    <h2>SkSL Shaders</h2>
    <span>
      <a
        id=githash
        href='https://skia.googlesource.com/skia/+show/${SKIA_VERSION}'
      >
        ${SKIA_VERSION.slice(0, 7)}
      </a>
      <theme-chooser-sk dark></theme-chooser-sk>
    </span>
  </header>
  <main>
    <div id=codeEditor></div>
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
  private static themeFromCurrentMode = () => (isDarkMode() ? 'base16-dark' : 'base16-light');

  connectedCallback(): void {
    super.connectedCallback();
    this._render();

    this.codeMirror = CodeMirror($$<HTMLDivElement>('#codeEditor', this)!, {
      lineNumbers: true,
      mode: 'text/x-c++src',
      theme: ShadersAppSk.themeFromCurrentMode(),
      viewportMargin: Infinity,
    });

    // Listen for theme changes.
    document.addEventListener('theme-chooser-toggle', () => {
      this.codeMirror!.setOption(
        'theme',
        ShadersAppSk.themeFromCurrentMode(),
      );
    });

    this.codeMirror.setValue(defaultShader);
  }
}

define('shaders-app-sk', ShadersAppSk);
