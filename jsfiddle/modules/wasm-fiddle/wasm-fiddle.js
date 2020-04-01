import 'elements-sk/styles/buttons';

import { $$ } from 'common-sk/modules/dom';
import { errorMessage } from 'elements-sk/errorMessage';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';

import { html, render } from 'lit-html';


export function codeEditor(ele) {
  return html`
<div id=editor>
  <textarea class=code spellcheck=false rows=${lines(ele.content)} cols=80
        @paste=${ele._changed} @input=${ele._changed}
  ></textarea>
  <div class=numbers>
    ${repeat(lines(ele.content)).map((_, n) => _lineNumber(n + 1))}
  </div>

</editor>`;
}

export function floatSlider(name, i) {
  if (!name) {
    return '';
  }
  // By setting the input's name=sliderN, the JS function will magically have a global variable
  // called sliderN that refers to the input HTML element.
  // https://www.w3schools.com/tags/att_input_name.asp
  return html`
<div class=widget>
  <input name=${`slider${i}`} id=${`slider${i}`} min=0 max=1 step=0.00001 type=range>
  <label for=${`slider${i}`}>${name}</label>
</div>`;
}

export function colorPicker(name, i) {
  if (!name) {
    return '';
  }
  // By setting the input's name=colorN, the JS function will magically have a global variable
  // called colorN that refers to the input HTML element.
  // https://www.w3schools.com/tags/att_input_name.asp
  return html`
<div class=widget>
  <input name=${`color${i}`} id=${`color${i}`} type=color>
  <label for=${`color${i}`}>${name}</label>
</div>`;
}

// Returns the number of lines in str, with a minimum of 10
// (because the editor with less than 10 lines looks a bit strange).
function lines(str) {
  // see https://stackoverflow.com/a/4009768
  return Math.max(10, (str.match(/\n/g) || []).length + 1);
}

// repeat returns an array of n 'undefined' which allows
// for repeating a template a fixed number of times
// using map.
function repeat(n) {
  // See https://stackoverflow.com/a/10050831
  return [...Array(n)];
}

const _lineNumber = (n) => html`
  <div id=${`L${n}`}>${n}</div>
`;

const sliderRegex = /#slider(\d):(\S+)/g;
const colorPickerRegex = /#color(\d):(\S+)/g;
const fpsRegex = /benchmarkFPS/;

/**
 * @module jsfiddle/modules/wasm-fiddle
 * @description <h2><code>wasm-fiddle</code></h2>
 *
 * <p>
 *   An element that has a code editor and canvas on which
 *   to display the output of a WASM-based library.
 * </p>
 * <h2> Assumptions <h2>
 * <ul>
 *   <li>The main template makes a call to codeEditor() and has a
 *   &lt;div&gt; element with id 'canvasContainer' where the canvas
 *   should go.</li>
 *   <li>There are buttons that call run() and save() on 'this'</li>
 *   <li>The foo.wasm has been copied into /res/ by means of CopyWebpackPlugin.</li>
 * </ul>
 *
 *
 */
export class WasmFiddle extends HTMLElement {
  /**
  * @param {Promise} wasmPromise: promise that will resolve with the WASM library.
  * @param {Object} template: The base template for this element.
  * @param {String} libraryName: What users call the library e.g. 'CanvasKit'
  * @param {String} fiddleType: The backend name for the fiddle e.g. 'canvasKit'
  */
  constructor(wasmPromise, template, libraryName, fiddleType) {
    super();

    // allows demo pages to supply content w/o making a network request
    this._content = this.getAttribute('content') || '';
    this.Wasm = null;
    this.wasmPromise = wasmPromise;
    this._editor = null; // set in render to be the textarea
    this.template = template;
    this.libraryName = libraryName; // e.g. 'CanvasKit' , 'PathKit'
    this.fiddleType = fiddleType; // e.g. 'canvaskit', 'pathkit'
    this.hasRun = false;
    this.loadedWasm = false;
    this.sliders = [];
    this.colorpickers = [];
    this.fpsMeter = false;
    // This will be updated to have any captured console.log (but not console.error or console.warn)
    // messages. this._render will be called on any updates to log as well.
    this.log = '';
    // runID is a unique identifier that changes every time run is clicked. This allows the client
    // code to stop animation loops when run is clicked a second time. See _activeRunInstance().
    this.runID = 0;
  }

  /** @prop {String} content - The current code in the editor. */
  get content() { return this._content; }

  set content(c) {
    this._content = c;
    this._enumerateWidgets();
    this._render();
    this._editor.value = c;
  }

  connectedCallback() {
    this._render();
    this.wasmPromise.then((LoadedWasm) => {
      this.Wasm = LoadedWasm;
      this.loadedWasm = true;
      this._render();
    });

    if (!this.content) {
      this._loadCode();
    }
    // Listen for the forward and back buttons and re-load the code
    // on any changes. Without this, the url changes, but nothing
    // happens in the DOM.
    window.addEventListener('popstate', this._loadCode.bind(this));
  }

  disconnectedCallback() {
    window.removeEventListener('popstate', this._loadCode.bind(this));
  }

  // Returns a helper function that will return true if the current running instance is the most
  // recent instance. This can be used by client code to stop their animation loops when the Run
  // button is hit again.
  _activeRunInstance(currentRunID) {
    return () => currentRunID === this.runID;
  }

  // Returns a helper function that will store the last 10 frame times and every tenth frame will
  // output the average FPS from those ten frames. The returned function has its variables tied up
  // in a closure so that new instances will not conflict with each other (e.g. when run is clicked)
  // It also checks to see if this invocation is the latest and will do nothing if it is not (e.g.
  // prevent competing updates to the fps meter.
  _benchmarkFPSInstance(currentRunID) {
    let lastTime = null;
    let frameIdx = 0;
    const frames = new Float64Array(10);
    let fpsEle = null;
    return () => {
      if (this.runID !== currentRunID) {
        return;
      }
      if (!lastTime || !fpsEle) {
        lastTime = performance.now();
        fpsEle = $$('#fps');
        return;
      }
      const now = performance.now();
      frames[frameIdx] = now - lastTime;
      lastTime = now;
      frameIdx = (frameIdx + 1) % frames.length;
      if (frameIdx === 0) {
        let sum = 0;
        for (let i = 0; i < frames.length; i++) {
          sum += frames[i];
        }
        fpsEle.textContent = `${(10000 / sum).toFixed(1)} FPS`;
        lastTime = performance.now();
      }
    };
  }

  _changed() {
    this.content = this._editor.value;
  }

  // Look through the current source code for references to sliders or colorpickers.
  // These have the magic values #sliderN:displayName and #colorN:displayName and we just
  // search the code given to use with two regex.
  _enumerateWidgets() {
    this.sliders = [];
    this.colorpickers = [];
    this.fpsMeter = !!this.content.match(fpsRegex);

    const sliderMatches = this.content.matchAll(sliderRegex);
    for (const match of sliderMatches) {
      // match[1] is the index of the slider.
      // match[2] is the display name.
      this.sliders[match[1]] = match[2];
    }

    const colorMatches = this.content.matchAll(colorPickerRegex);
    for (const match of colorMatches) {
      // match[1] is the index of the color picker.
      // match[2] is the display name.
      this.colorpickers[match[1]] = match[2];
    }
  }

  _loadCode() {
    // The location should be either /<fiddleType> or /<fiddleType>/<fiddlehash>
    const path = window.location.pathname;
    let hash = '';
    const len = this.fiddleType.length + 2; // count of chars in /<fiddleType>/
    if (path.length > len) {
      hash = path.slice(len);
    }

    fetch(`/_/code?type=${this.fiddleType}&hash=${hash}`)
      .then(jsonOrThrow)
      .then((json) => {
        this.content = json.code;
      }).catch((e) => {
        errorMessage('Fiddle not Found', 10000);
        this.content = '';
        const canvas = $$('#canvas', this);
        this._resetCanvas(canvas);
      });
  }

  _render() {
    render(this.template(this), this, { eventContext: this });
    this._editor = $$('#editor textarea', this);
  }

  // create a brand new canvas. Without this, the context can get muddled
  // between calls, especially when using WebGL. We can't simply drawRect
  // to clear it because that creates a 2d drawing context which prevents
  // use with a webGL context.
  _resetCanvas() {
    const cc = $$('#canvasContainer', this);
    cc.innerHTML = '';
    const canvas = document.createElement('canvas');
    canvas.width = 500;
    canvas.height = 500;
    canvas.id = 'canvas';
    cc.appendChild(canvas);
    return canvas;
  }

  /**
    Runs the code, allowing the user to see the result on the canvas.
  */
  run() {
    this.runID = Date.now();
    // reset the log on each run.
    this.log = '';
    // consoleInterceptor is used to intercept console.log calls and store them.
    const consoleInterceptor = {
      log: (...rest) => {
        // pipe this through to regular console.log
        console.log(...rest);
        // stringify all the arguments for rendering using the log property.
        for (let i = 0; i < rest.length; i++) {
          const a = rest[i];
          if (typeof a === 'object') {
            // Make an attempt to prettify objects - this doesn't work well on WASM objects
            // or DOMElements.
            this.log += JSON.stringify(a);
          } else {
            this.log += a;
          }
          this.log += ' ';
        }
        this.log += '\n';
        this._render();
      },
      warn: console.warn,
      error: console.error,
    };

    if (!this.Wasm) {
      errorMessage(`${this.libraryName} is still loading. Try again in a few seconds.`);
      return;
    }
    this.hasRun = true;

    this._render();
    const canvas = this._resetCanvas();


    try {
      // Because of the magic of setting <input name=sliderN>, we don't need to declare any
      // variables for sliders or colorpickers (see floatSlider and colorPicker above).
      const f = new Function(
        this.libraryName, // e.g. "CanvasKit", the name of the WASM library.
        'canvas', // We provide the canvas element to the user as a parameter named 'canvas'.
        'console', // By having this parameter named 'console', we intercept a user's normal
        // calls to the window.console object [unless they happen to actually say
        // window.console.log('foo')].
        'benchmarkFPS', // provide a helper that the user can call to get an FPS output.
        'isRunning', // provide a helper for the user to stop their animation when run is clicked.
        this.content, // user provided code, as a string, which will be interpreted and executed.
      );
      f(this.Wasm, canvas, consoleInterceptor, this._benchmarkFPSInstance(this.runID),
        this._activeRunInstance(this.runID));
    } catch (e) {
      errorMessage(e);
    }
  }

  /**
    Sends the code to the backend to be saved. Updates the URL upon success
    to the new permalink for this fiddle.
  */
  save() {
    fetch('/_/save', {
      method: 'PUT',
      headers: new Headers({
        'content-type': 'application/json',
      }),
      body: JSON.stringify({
        code: this.content,
        type: this.fiddleType,
      }),
    }).then(jsonOrThrow).then((json) => {
      history.pushState(null, '', json.new_url);
    }).catch(errorMessage);
  }
}
