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
import { html, render } from 'lit-html';
import { repeat } from 'lit-html/directives/repeat';

const CanvasKitInit = require('../../build/canvaskit/canvaskit.js');

const loadingTemplate = (ele) => html`
<div class=player-loading title="Loading animation and engine."
     style='width: ${ele._config.width}px; height: ${ele._config.height}px;'>
  <div>Loading</div>
  <spinner-sk active></spinner-sk>
</div>`;

const settingsTemplate = (ele) => html`
<div class=skottie-player-settings-container ?hidden=${!ele._state.showSettings}>
  <div class=skottie-player-settings-row>
    <div class=skottie-player-settings-label>Colors</div>
    <select id=color-prop-select class=skottie-player-property-select
            @input=${ele._onPropertySelect} ?disabled=${ele._props.color.empty()}>
      ${repeat(ele._props.color.list, (c) => c.key, (c, index) => html`
        <option value=${index}>${c.key}</option>
      `)}
    <select>
    <input type=color class=skottie-player-picker id=color-picker
           value=${hexColor(ele._props.color.current().value)}
           @input=${ele._onColorInput} ?disabled=${ele._props.color.empty()}>
    <hr class=skottie-player-settings-divider>
  </div>
  <div class=skottie-player-settings-row>
    <div class=skottie-player-settings-label>Opacity</div>
    <select id=opacity-prop-select class=skottie-player-property-select
            @input=${ele._onPropertySelect} ?disabled=${ele._props.opacity.empty()}>
      ${repeat(ele._props.opacity.list, (o) => o.key, (o, index) => html`
        <option value=${index}>${o.key}</option>
      `)}
    <select>
    <input type=range min=0 max=100 class=skottie-player-picker id=opacity-picker
           value=${ele._props.opacity.current().value}
           @input=${ele._onOpacityInput} ?disabled=${ele._props.opacity.empty()}>
    <hr class=skottie-player-settings-divider>
  </div>
  <div class=skottie-player-settings-row>
    <div class=skottie-player-settings-label>Segments</div>
    <select id=segment-prop-select class=skottie-player-property-select
            style='width: 100%' @input=${ele._onPropertySelect}>
      ${repeat(ele._props.segments, (s) => s.name, (s, index) => html`
        <option value=${index}>${segmentLabel(s)}</option>
      `)}
    <select>
    <hr class=skottie-player-settings-divider>
  </div>
  <div class=skottie-player-settings-row>
    <input type=button value=Close @click=${ele._onSettings}>
  </div>
</div>
`;

function segmentLabel(s) {
  return `${s.name} [${s.t0.toFixed(2)} .. ${s.t1.toFixed(2)}]`;
}

function hexColor(c) {
  const rgb = c & 0x00ffffff;
  return `#${rgb.toString(16).padStart(6, '0')}`;
}

function skRectIsEmpty(rect) {
  if (!rect) {
    return true;
  }
  if (rect.constructor === Float32Array) {
    return rect[2] <= rect[0] || rect[3] <= rect[1];
  }
  // TODO(kjlubick) remove this deprecated rectangle format after the array version lands in the
  //   Skia repo.
  return rect.fRight <= rect.fLeft || rect.fBottom <= rect.fTop;
}

const runningTemplate = (ele) => html`
<div class=container>
  <div class=wrapper>
    <canvas class=skottie-canvas id=skottie
            width=${ele._config.width * window.devicePixelRatio}
            height=${ele._config.height * window.devicePixelRatio}
            style='width: ${ele._config.width}px; height: ${ele._config.height}px; background-color: ${ele._config.bgColor}'>
      Your browser does not support the canvas tag.
    </canvas>
    <div class=controls ?hidden=${!ele._config.controls}>
      <play-arrow-icon-sk @click=${ele._onPlay} ?hidden=${!ele._state.paused}></play-arrow-icon-sk>
      <pause-icon-sk @click=${ele._onPause} ?hidden=${ele._state.paused}></pause-icon-sk>
      <input type=range min=0 max=100 @input=${ele._onScrub} @change=${ele._onScrubEnd}
             class=skottie-player-scrubber>
      <settings-icon-sk @click=${ele._onSettings}></settings-icon-sk>
    </div>
  </div>
  ${settingsTemplate(ele)}
</div>`;

// This element might be loaded from a different site, and that means we need
// to be careful about how we construct the URL back to the canvas.wasm file.
// Start by recording the script origin.
const scriptOrigin = new URL(document.currentScript.src).origin;

const canvasReady = CanvasKitInit({
  locateFile: (file) => `${scriptOrigin}/static/${file}`,
});

define('skottie-player-sk', class extends HTMLElement {
  constructor() {
    super();

    this._engine = {
      kit: null, // CanvasKit instance
      context: null, // CK context.
      animation: null, // Skottie Animation instance
      surface: null, // SkSurface
      canvas: null, // Cached SkCanvas (surface.getCanvas()).
    };

    this._state = {
      loading: true,
      paused: this.hasAttribute('paused'),
      scrubPlaying: false, // Animation was playing when the user started scrubbing.
      duration: 0, // Animation duration (ms).
      nativeFps: 0, // Animation fps.
      timeOrigin: 0, // Animation start time (ms).
      seekPoint: 0, // Normalized [0..1] animation progress.
      showSettings: (new URL(document.location)).searchParams.has('settings'),
      currentSegment: { name: '', t0: 0, t1: 1 }, // One of the _props.segments
    };

    function PropList(list, defaultVal) {
      this.list = list;
      this.defaultVal = defaultVal;
      this.index = 0;
      this.empty = () => !this.list.length;
      this.current = () => (this.index >= this.list.length
        ? this.defaultVal
        : this.list[this.index]);
    }

    this._props = {
      color: new PropList([], 0.0), // Configurable color properties
      opacity: new PropList([], 1.0), // Configurable opacity properties
      segments: [], // Selectable animation segments
    };
  }

  connectedCallback() {
    const params = (new URL(document.location)).searchParams;
    this._config = {
      width: this.hasAttribute('width') ? this.getAttribute('width') : 256,
      height: this.hasAttribute('height') ? this.getAttribute('height') : 256,
      controls: params.has('controls'),
      bgColor: params.has('bg') ? params.get('bg') : '#fff',
    };
    this._render();
  }

  initialize(config) {
    this._config.width = config.width;
    this._config.height = config.height;
    this._config.fps = config.fps;
    this._animationName = config.lottie.nm;

    this._render();
    return canvasReady.then((ck) => {
      // Set a large-ish decode cache limit to accommodate potentially large images.
      const CACHE_SIZE = 512 * 1024 * 1024;
      ck.setDecodeCacheLimitBytes(CACHE_SIZE);

      this._engine.kit = ck;
      this._initializeSkottie(config.lottie, config.assets, config.soundMap);
      this._render();
    });
  }

  duration() {
    return this._state.duration * (this._state.currentSegment.t1 - this._state.currentSegment.t0);
  }

  fps() {
    return this._state.nativeFps;
  }

  animationName() {
    return this._animationName;
  }

  canvas() {
    return this.querySelector(".skottie-canvas");
  }

  seek(t) {
    this._state.timeOrigin = (Date.now() - this.duration() * t);

    if (!this.isPlaying()) {
      // Force-draw a static frame when paused.
      this._updateSeekPoint();
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

  _initializeSkottie(lottieJSON, assets, soundMap) {
    this._state.loading = false;

    // Rebuild the surface only if needed.
    if (!this._engine.surface
        || this._engine.surface.width !== this._config.width
        || this._engine.surface.height !== this._config.height) {
      this._render();

      if (this._engine.surface) {
        this._engine.surface.delete();
      }
      const canvasEle = $$('#skottie', this);
      this._engine.surface = this._engine.kit.MakeCanvasSurface(canvasEle);
      if (!this._engine.surface) {
        throw new Error('Could not make SkSurface.');
      }
      // We don't need to call .delete() on the canvas because
      // the parent surface will do that for us.
      this._engine.canvas = this._engine.surface.getCanvas();

      this._engine.context = this._engine.kit.currentContext();
    }

    if (this._engine.animation) {
      this._engine.animation.delete();
    }

    this._engine.animation = this._engine.kit.MakeManagedAnimation(
      JSON.stringify(lottieJSON), assets, null, soundMap
    );
    if (!this._engine.animation) {
      throw new Error('Could not parse Lottie JSON.');
    }

    this._state.duration = this._engine.animation.duration() * 1000;
    this._state.nativeFps = this._engine.animation.fps();
    this.seek(0);

    this._props.color.list = this._engine.animation.getColorProps();
    this._props.opacity.list = this._engine.animation.getOpacityProps();
    this._props.segments = [{ name: 'Full timeline', t0: 0, t1: 1 }]
      .concat(this._engine.animation.getMarkers());
    this._currentSegment = this._props.segments[0];

    this._render(); // re-render for animation-dependent elements (properties, etc).

    this._drawFrame(true);
  }

  _updateSeekPoint() {
    // t is in animation segment domain.
    const t = ((Date.now() - this._state.timeOrigin) / this.duration()) % 1;

    // map to the global animation timeline
    this._state.seekPoint = this._state.currentSegment.t0
                          + t * (this._state.currentSegment.t1 - this._state.currentSegment.t0);
    if (this._config.controls) {
      const scrubber = this.querySelector('.skottie-player-scrubber');
      if (scrubber) {
        scrubber.value = this._state.seekPoint * 100;
      }
    }
  }

  _drawFrame(firstFrame) {
    if (!this._engine.animation || !this._engine.canvas) {
      return;
    }

    // When paused, the progress is fully controlled externally.
    if (this.isPlaying()) {
      this._updateSeekPoint();
      window.requestAnimationFrame(this._drawFrame.bind(this));
    }

    let frame = this._state.seekPoint * this._state.duration * this._state.nativeFps / 1000;
    if (this._config.fps) {
      // When a render FPS is specified, quantize to the desired rate.
      const fpsScale = this._config.fps / this._state.nativeFps;
      frame = Math.trunc(frame * fpsScale) / fpsScale;
    }

    this._engine.kit.setCurrentContext(this._engine.context);
    const damage = this._engine.animation.seekFrame(frame);
    // Only draw frames when the content changes.
    if (firstFrame || !skRectIsEmpty(damage)) {
      const bounds = this._engine.kit.LTRBRect(0, 0, this._config.width * window.devicePixelRatio,
        this._config.height * window.devicePixelRatio);
      this._engine.animation.render(this._engine.canvas, bounds);
      this._engine.surface.flush();
    }
  }

  _render() {
    render(this._state.loading
      ? loadingTemplate(this)
      : runningTemplate(this),
    this, { eventContext: this });
  }

  _onPlay() {
    this.play();
    this._render();
  }

  _onPause() {
    this.pause();
    this._render();
  }

  // This fires every time the user moves the scrub slider.
  _onScrub(e) {
    this.seek(e.currentTarget.value / 100);

    // Pause the animation while dragging the slider.
    if (this.isPlaying()) {
      this._state.scrubPlaying = true;
      this.pause();
    }
  }

  // This fires when the user releases the scrub slider.
  _onScrubEnd(e) {
    if (this._state.scrubPlaying) {
      this._state.scrubPlaying = false;
      this.play();
    }
  }

  _onSettings() {
    this._state.showSettings = !this._state.showSettings;
    this._render();
  }

  _onPropertySelect(e) {
    switch (e.target.id) {
      case 'color-prop-select':
        this._props.color.index = e.target.value;
        this.querySelector('#color-picker').value = hexColor(this._props.color.current().value);
        break;
      case 'opacity-prop-select':
        this._props.opacity.index = e.target.value;
        this.querySelector('#opacity-picker').value = this._props.opacity.current().value;
        break;
      case 'segment-prop-select':
        this._state.currentSegment = this._props.segments[e.target.value];
        this.seek(0);
        this._render();
        break;
    }
  }

  _onColorInput(e) {
    const val = e.target.value;
    const prop = this._props.color.current();
    prop.value = this._engine.kit.Color(parseInt(val.substring(1, 3), 16),
      parseInt(val.substring(3, 5), 16),
      parseInt(val.substring(5, 7), 16),
      1.0); // Treat colors as fully opaque.

    this._engine.animation.setColor(prop.key, prop.value);
    this._render();

    if (!this.isPlaying()) {
      this._drawFrame();
    }
  }

  _onOpacityInput(e) {
    const prop = this._props.opacity.current();
    prop.value = Number(e.target.value);

    this._engine.animation.setOpacity(prop.key, prop.value);
    this._render();

    if (!this.isPlaying()) {
      this._drawFrame();
    }
  }
});
