/**
 * @module skottie-player-sk
 * @description <h2><code>skottie-player-sk</code></h2>
 *
 * <p>
 *   Displays a CanvasKit-based Skottie animation and provides various controls.
 * </p>
 *
 */
import 'elements-sk/icon/pause-icon-sk'
import 'elements-sk/icon/play-arrow-icon-sk'
import 'elements-sk/spinner-sk'
import { html, render } from 'lit-html'

const CanvasKitInit = require('../../build/canvaskit/canvaskit.js');

const loadingTemplate = (ele) => html`
<div class=player-loading title="Loading animation and engine."
     style='width: ${ele._config.width}px; height: ${ele._config.height}px;'>
  <div>Loading</div>
  <spinner-sk active></spinner-sk>
</div>`;

const runningTemplate = (ele) => html`
<div class=skottie-player-wrapper>
  <canvas class=skottie-canvas id=skottie
          width=${ele._config.width * window.devicePixelRatio}
          height=${ele._config.height * window.devicePixelRatio}
          style='width: ${ele._config.width}px; height: ${ele._config.height}px;'>
    Your browser does not support the canvas tag.
  </canvas>
  <div class=skottie-player-controls ?hidden=${!ele._config.controls}>
    <play-arrow-icon-sk @click=${ele._onPlay} ?hidden=${!ele._state.paused}></play-arrow-icon-sk>
    <pause-icon-sk @click=${ele._onPause} ?hidden=${ele._state.paused}></pause-icon-sk>
  </div>
</div>`;

window.customElements.define('skottie-player-sk', class extends HTMLElement {
  constructor() {
    super();

    this._config = {
      width:    this.hasAttribute('width')  ? this.getAttribute('width')  : 256,
      height:   this.hasAttribute('height') ? this.getAttribute('height') : 256,
      controls: (new URL(document.location)).searchParams.has('controls'),
    };

    this._engine = {
      kit:       null, // CanvasKit instance
      context:   null, // CK context.
      animation: null, // Skottie Animation instance
      surface:   null, // SkSurface
      canvas:    null, // Cached SkCanvas (surface.getCanvas()).
    };

    this._state = {
      loading:    true,
      paused:     this.hasAttribute('paused'),
      duration:   0,   // Animation duration (ms).
      timeOrigin: 0,   // Animation start time (ms).
      seekPoint:  0,   // Normalized [0..1] animation progress.
    };
  }

  connectedCallback() {
    this._render();
  }

  initialize(config) {
    this._config.width = config.width;
    this._config.height = config.height;

    if (this._engine.kit) {
      return new Promise((resolve, reject) => {
              this._initializeSkottie(config.lottie);
              resolve();
             });
    }

    this._render();
    return new Promise((resolve, reject) => {
                 CanvasKitInit({
                   locateFile: (file) => '/static/'+file,
                 }).then((ck) => {
                   this._engine.kit = ck;
                   this._initializeSkottie(config.lottie);
                   resolve();
                 });
               });
  }

  duration() {
    return this._state.duration;
  }

  seek(t) {
    this._state.seekPoint = t;
    this._state.timeOrigin = (Date.now() - this._state.duration * t);

    if (!this.isPlaying()) {
      // Force-draw a static frame when paused.
      this._drawFrame();
    }
  }

  isPlaying() {
    return !this._state.paused;
  }

  pause() {
    if (this.isPlaying()) {
      this._state.paused = true;
      // Save the exact/current seek point at pause time.
      this._updateSeekPoint();
    }
  }

  play() {
    if (!this.isPlaying()) {
      this._state.paused = false;
      // Shift timeOrigin to continue from where we paused.
      this.seek(this._state.seekPoint);
      this._drawFrame();
    }
  }

  _initializeSkottie(lottieJSON) {
    this._state.loading = false;

    // Rebuild the surface only if needed.
    if (!this._engine.surface ||
        this._engine.surface.width  != this._config.width ||
        this._engine.surface.height != this._config.height) {

      this._render();

      this._engine.surface && this._engine.surface.delete();
      this._engine.surface = this._engine.kit.MakeCanvasSurface('skottie');
      if (!this._engine.surface) {
        throw new Error('Could not make SkSurface.');
      }
      // We don't need to call .delete() on the canvas because
      // the parent surface will do that for us.
      this._engine.canvas = this._engine.surface.getCanvas();

      this._engine.context = this._engine.kit.currentContext();
    }

    this._engine.animation && this._engine.animation.delete();

    this._engine.animation = this._engine.kit.MakeAnimation(JSON.stringify(lottieJSON));
    if (!this._engine.surface) {
      throw new Error('Could not parse Lottie JSON.');
    }

    this._state.duration = this._engine.animation.duration() * 1000;
    this.seek(0);

    this._drawFrame();
  }

  _updateSeekPoint() {
    this._state.seekPoint = ((Date.now() - this._state.timeOrigin) / this.duration()) % 1;
  }

  _drawFrame() {
    if (!this._engine.animation || !this._engine.canvas) {
      return;
    }

    // When paused, the progress is fully controlled externally.
    if (this.isPlaying()) {
      this._updateSeekPoint();
      window.requestAnimationFrame(this._drawFrame.bind(this));
    }

    this._engine.animation.seek(this._state.seekPoint);
    this._engine.kit.setCurrentContext(this._engine.context);
    this._engine.animation.render(this._engine.canvas, {
                                  fLeft: 0,
                                  fTop:  0,
                                  fRight:  this._config.width  * window.devicePixelRatio,
                                  fBottom: this._config.height * window.devicePixelRatio });
    this._engine.surface.flush();
  }

  _render() {
    render(this._state.loading
               ? loadingTemplate(this)
               : runningTemplate(this),
           this, {eventContext: this});
  }

  _onPlay() {
    this.play();
    this._render();
  }

  _onPause() {
    this.pause();
    this._render();
  }
});
