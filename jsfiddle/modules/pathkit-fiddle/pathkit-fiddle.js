import 'elements-sk/styles/buttons'
import 'elements-sk/error-toast-sk'

import { errorMessage } from 'elements-sk/errorMessage'

import { $$ } from 'common-sk/modules/dom'
import { diffDate } from 'common-sk/modules/human'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'

import { html, render } from 'lit-html'

const PathKitInit = require('pathkit-wasm/bin/pathkit.js');

// Main template for this element
const template = (ele) => html`
<header>
  <div class=title>PathKit Fiddle</div>
  <div class=flex></div>
  <div class=version>PathKit Version: <a href="https://www.npmjs.com/package/pathkit-wasm">0.4.0</a></div>
</header>

<main>
  ${codeEditor(ele)}
  <div class=output>
    <div class=buttons>
      <button class=action @click=${() => ele._run()}>Run</button>
      <button class=action @click=${() => ele._save()}>Save</button>
    </div>
    <canvas id=canvas width=500 height=500></canvas>
  </div>
</main>
<footer>
  <error-toast-sk></error-toast-sk>
</footer>`;


const codeEditor = (ele) => html`
<div id=editor>
  <textarea class=code spellcheck=false rows=${lines(ele.content)} cols=80
        @paste=${() => ele._changed()} @input=${() => ele._changed()}
  >${ele.content}</textarea>
  <div class=numbers>
    ${repeat(lines(ele.content)).map((_, n) => _lineNumber(n+1))}
  </div>

</editor>
`

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

function resetCanvas(canvas) {
  // Reset the transform of the canvas then re-draw it blank.
  let ctx = canvas.getContext('2d');
  ctx.setTransform(1, 0, 0, 1, 0, 0);
  ctx.clearRect(0, 0, canvas.width, canvas.height);
}

/**
 * @module jsfiddle/modules/pathkit-fiddle
 * @description <h2><code>pathkit-fiddle</code></h2>
 *
 * <p>
 *   The top level element for displaying pathkit fiddles.
 *   The main elements are a code editor box (textarea), a canvas
 *   on which to render the result and a few buttons.
 * </p>
 *
 */
window.customElements.define('pathkit-fiddle', class extends HTMLElement {

  constructor() {
    super();

    this._content = '';
    this.PathKit = null;
    this._editor = null; // set in render to be the textarea
  }

  /** @prop {String} content - The current code in the editor.*/
  get content() { return this._content; }
  set content(c) {
    this._content = c;
    this._render();
  }

  connectedCallback() {
    this._render();
    PathKitInit({
      locateFile: (file) => '/res/'+file,
    }).then((PathKit) => {
      this.PathKit = PathKit;
      if (this.content) {
        this._run(); // auto-run the code if the code was loaded.
      }
    });

    if (!this.content) {
      fetch("/_/code?type=pathkit&hash=demo")
        .then(jsonOrThrow)
        .then((json) => {
          this.content = json.code;
          if (this.PathKit) {
            this._run(); // auto-run the code if PathKit is loaded.
          }
        }
      );
    }
  }

  _changed() {
    this.content = this._editor.value;
  }

  _render() {
    render(template(this), this);
    this._editor = $$('#editor textarea', this);
  }

  _run() {
    if (!this.PathKit) {
      errorMessage('PathKit is still loading.');
      return;
    }

    const canvas = $$('#canvas', this);
    resetCanvas(canvas);
    try {
      const f = new Function('PathKit', 'canvas', this.content);
      f(this.PathKit, canvas);
    } catch(e) {
      errorMessage(e);
    }
  }

  _save() {
    // TODO(kjlubick):
    // make a POST request to /_/save with form
    // {
    //   "code": ...,
    //   "type": "pathkit",
    // }
    // which will return JSON of the form
    // {
    //   "new_url": "/pathkit/123adfs45asdf59923123"
    // }
    // where new_url is the hash (probably sha256) of the content
    // and this will re-direct the browser to that new url.
  }

});
