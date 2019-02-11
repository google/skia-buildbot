import 'elements-sk/styles/buttons'

import { $$ } from 'common-sk/modules/dom'
import { errorMessage } from 'elements-sk/errorMessage'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'

import { html, render } from 'lit-html'


export function codeEditor(ele) {
  return html`
<div id=editor>
  <textarea class=code spellcheck=false rows=${lines(ele.content)} cols=80
        @paste=${ele._changed} @input=${ele._changed}
  ></textarea>
  <div class=numbers>
    ${repeat(lines(ele.content)).map((_, n) => _lineNumber(n+1))}
  </div>

</editor>`;
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
  <div id=${'L'+n}>${n}</div>
`;

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
  }

  /** @prop {String} content - The current code in the editor.*/
  get content() { return this._content; }
  set content(c) {
    this._content = c;
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

  _changed() {
    this.content = this._editor.value;
  }

  _loadCode() {
    // The location should be either /<fiddleType> or /<fiddleType>/<fiddlehash>
    let path = window.location.pathname;
    let hash = '';
    const len = this.fiddleType.length + 2; // count of chars in /<fiddleType>/
    if (path.length > len) {
      hash = path.slice(len);
    }

    fetch(`/_/code?type=${this.fiddleType}&hash=${hash}`)
      .then(jsonOrThrow)
      .then((json) => {
        this.content = json.code;
      }
    ).catch((e) => {
      errorMessage('Fiddle not Found', 10000);
      this.content = '';
      const canvas = $$('#canvas', this);
      this._resetCanvas(canvas);
    });
  }

  _render() {
    render(this.template(this), this, {eventContext: this});
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
    if (!this.Wasm) {
      errorMessage(`${this.libraryName} is still loading. Try again in a few seconds.`);
      return;
    }
    this.hasRun = true;
    this._render();
    const canvas = this._resetCanvas();

    try {
      let f = new Function(this.libraryName, 'canvas', // actual params
          this.content); // user given code
      f(this.Wasm, canvas);
    } catch(e) {
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
      })
    }).then(jsonOrThrow).then((json) => {
        history.pushState(null, '', json.new_url);
      }
    ).catch(errorMessage);
  }

};
