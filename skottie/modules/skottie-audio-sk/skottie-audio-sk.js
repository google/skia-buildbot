/**
 * @module skottie-audio-sk
 * @description <h2><code>skottie-audio-sk</code></h2>
 *
 * <p>
 *   A skottie audio sync manager.
 * </p>
 * <p>
 *   This module lets the user upload a song track and synchronize
 *   it with the animation by speeding up or slowing down
 *   the animation to fit the music.
 * </p>
 * <p>
 *   To make this easier, the module can do some analysis of the song
 *   to suggest the tempo of the song (in beats per minute or BPMs).
 *   Once a song is uploaded, we identify the top 5 likely bpms
 *   for the song and suggest those options or read the bpm encoded in the file name.
 *   If none of those options work, BPMs can be set manually.
 * </p>
 * <p>
 *   Once the BPM is established, a second "Beat duration" value is required.
 *   This Beat duration reflects how many frames of the animation
 *   represent a single beat. This value will be used to calculate the final speed
 *   of the animation itself.
 * </p>
 *
 *
 * @evt apply - This event is triggered when the audio settings are set.
 *
 *
 */
import { define } from 'elements-sk/define';
import { html, render } from 'lit-html';
import { Howl } from 'howler';

const INPUT_FILE_ID = 'fileInput';
const BPM_ID = 'bpm';
const BEAT_DURATION_ID = 'beatDuration';
const START_BUTTON_ID = 'startButton';

const LOADING_STATES = {
  IDLE: 'idle',
  LOADING: 'loading',
  LOADED: 'loaded',
  SUBMITTED: 'submitted',
};

const startButtonTemplate = (ele) => {
  if (ele._state.loadingState === LOADING_STATES.IDLE
    || ele._state.loadingState === LOADING_STATES.SUBMITTED
  ) {
    return null;
  }
  if (ele._state.loadingState === LOADING_STATES.LOADING
    || ele._state.bpmCalculationState === LOADING_STATES.LOADING) {
    return html`<div>Loading...</div>`;
  }
  return html`
    <button
    class=start
    id=${START_BUTTON_ID}
    @click=${ele._start}
    >Start</button>
  `;
};

const bpmListOptionTemplate = (option, onClick) => html`
  <li class=bpm-options-item>
    <button class=bpm-options-item-button
      @click=${() => onClick(option.tempo)}
    >
      ${option.tempo}
    </button>
  </li>
`;

const bpmListTemplate = (ele) => html`
  <ul class=bpm-options>
    ${ele._state.bmpList.map(((bpmOption) => bpmListOptionTemplate(
    bpmOption,
    (option) => ele._onBpmSelected(option),
  )))}
  </ul>
`;

const template = (ele) => html`
  <div>
    <header class="header">
      Audio
    </header>
    <section class=section>
        <div class=inputs>
          <label class=input-label>
            <input
                type=file
                name=file
                id=${INPUT_FILE_ID}
            /> Choose audio file
          </label>
          <label class=input-label>
            <input
              type=number
              id=${BPM_ID}
              .value=${ele._state.bpm}
              required
            /> BPM
          </label>
          ${bpmListTemplate(ele)}
          <label class=input-label>
            <input
              type=number
              id=${BEAT_DURATION_ID}
              .value=${ele._state.beatDuration}
              required
            /> Beat Duration (in frames)
          </label>
        </div>
        ${startButtonTemplate(ele)}
    <section>
  </div>
`;

class SkottieAudioSk extends HTMLElement {
  constructor() {
    super();
    this._state = {
      bpm: 0,
      beatDuration: 0,
      loadingState: LOADING_STATES.IDLE,
      bpmCalculationState: LOADING_STATES.IDLE,
      bmpList: [],
    };
    this._sound = null;
    this._file = null;
    this._animation = null;
  }

  /** @prop animation {Object} new animation to traverse. */
  set animation(val) {
    if (this._animation !== val) {
      this._animation = val;
      this._updateAnimation(val);
    }
  }

  _updateAnimation(animation) {
    const markers = animation.markers || [];
    const marker = markers.find((markerItem) => {
      try {
        const markerData = JSON.parse(markerItem.cm);
        if (markerData.type === 'beat') {
          return true;
        }
      } catch (err) {
        // Marker does not have beat information
      }
      return false;
    });
    if (marker) {
      const beatData = {
        ...marker,
        payload: JSON.parse(marker.cm),
      };
      if (beatData.payload.beat) {
        this._state.beatDuration = beatData.payload.beat;
        this._render();
      }
    }
  }

  _getPeaks(data) {
    // What we're going to do here, is to divide up our audio into parts.

    // We will then identify, for each part, what the loudest sample is in that
    // part.

    // It's implied that that sample would represent the most likely 'beat'
    // within that part.

    // Each part is 0.5 seconds long - or 22,050 samples.

    // This will give us 60 'beats' - we will only take the loudest half of
    // those.

    // This will allow us to ignore breaks, and allow us to address tracks with
    // a BPM below 120.

    const partSize = 22050;
    const parts = data[0].length / partSize;
    let peaks = [];

    for (let i = 0; i < parts; i++) {
      let max = 0;
      for (let j = i * partSize; j < (i + 1) * partSize; j++) {
        const volume = Math.max(Math.abs(data[0][j]), Math.abs(data[1][j]));
        if (!max || (volume > max.volume)) {
          max = {
            position: j,
            volume: volume,
          };
        }
      }
      peaks.push(max);
    }
    // We then sort the peaks according to volume...
    peaks.sort((a, b) => b.volume - a.volume);
    // ...take the loundest half of those...
    peaks = peaks.splice(0, peaks.length * 0.5);
    // ...and re-sort it back based on position.
    peaks.sort((a, b) => a.position - b.position);
    return peaks;
  }

  _getIntervals(peaks) {
    // What we now do is get all of our peaks, and then measure the distance to
    // other peaks, to create intervals.  Then based on the distance between
    // those peaks (the distance of the intervals) we can calculate the BPM of
    // that particular interval.
    // The interval that is seen the most should have the BPM that corresponds
    // to the track itself.
    const groups = [];

    peaks.forEach((peak, index) => {
      for (let i = 1; (index + i) < peaks.length && i < 10; i++) {
        const group = {
          tempo: (60 * 44100) / (peaks[index + i].position - peak.position),
          count: 1,
        };

        while (group.tempo < 90) {
          group.tempo *= 2;
        }

        while (group.tempo > 180) {
          group.tempo /= 2;
        }

        group.tempo = Math.round(group.tempo);

        const groupTempo = groups.find((interval) => interval.tempo === group.tempo);
        if (!groupTempo) {
          groups.push(group);
        } else {
          groupTempo.count += 1;
        }
      }
    });
    return groups;
  }

  _onOfflineRenderComplete(e) {
    const buffer = e.renderedBuffer;
    const peaks = this._getPeaks([buffer.getChannelData(0), buffer.getChannelData(1)]);
    const groups = this._getIntervals(peaks);

    const top = groups.sort((intA, intB) => intB.count - intA.count).splice(0, 5);
    this._state.bmpList = top;
    if (!this._state.bpm) {
      this._state.bpm = top[0].tempo;
    }
    this._state.bpmCalculationState = LOADING_STATES.LOADED;
    this._render();
  }

  // It looks for peaks on the audio to estimate best guesses of the bpm
  // more info about it https://jmperezperez.com/bpm-detection-javascript/
  _detectBPMs(ev) {
    const track = ev.target.result;

    // Create offline context
    const OfflineContext = window.OfflineAudioContext || window.webkitOfflineAudioContext;
    const offlineContext = new OfflineContext(2, 30 * 44100, 44100);

    offlineContext.decodeAudioData(track, (buffer) => {
      // Create buffer source
      const source = offlineContext.createBufferSource();
      source.buffer = buffer;

      // Beats, or kicks, generally occur around the 100 to 150 hz range.
      // Below this is often the bassline.  So let's focus just on that.

      // First a lowpass to remove most of the song.

      const lowpass = offlineContext.createBiquadFilter();
      lowpass.type = 'lowpass';
      lowpass.frequency.value = 150;
      lowpass.Q.value = 1;

      // Run the output of the source through the low pass.

      source.connect(lowpass);

      // Now a highpass to remove the bassline.

      const highpass = offlineContext.createBiquadFilter();
      highpass.type = 'highpass';
      highpass.frequency.value = 100;
      highpass.Q.value = 1;

      // Run the output of the lowpass through the highpass.

      lowpass.connect(highpass);

      // Run the output of the highpass through our offline context.

      highpass.connect(offlineContext.destination);

      // Start the source, and render the output into the offline conext.

      source.start(0);
      offlineContext.startRendering();
    });

    offlineContext.oncomplete = (ev) => this._onOfflineRenderComplete(ev);
  }

  _onBpmSelected(option) {
    this._state.bpm = option;
    if (this._state.loadingState === LOADING_STATES.SUBMITTED) {
      this._start();
    } else {
      this._render();
    }
  }

  _onFileDataLoaded(ev) {
    const result = ev.target.result;
    if (this._sound) {
      this._sound.unload();
    }
    this._sound = new Howl({
      src: [result],
    });
    window._sound = this._sound;
    this._sound.on('load', () => this._onAudioLoaded());
  }

  _onBpmChange(ev) {
    ev.preventDefault();
    const value = parseInt(ev.target.value, 10);
    if (this._state.bpm !== value) {
      this._state.bpm = value;
      if (this._state.loadingState === LOADING_STATES.SUBMITTED) {
        this._start();
      } else {
        this._render();
      }
    }
  }

  _onBeatDurationChange(ev) {
    ev.preventDefault();
    const value = Number(ev.target.value);
    if (this._state.beatDuration !== value) {
      this._state.beatDuration = value;
    }
    this._render();
  }

  _onAudioLoaded() {
    this._state.loadingState = LOADING_STATES.LOADED;
    this._render();
  }

  // It looks for a bpm as part of the file name.
  // The format of the name should be [name]bpm_[number]
  _searchBpmOnName(name) {
    const regex = /bpm_([0-9]*)/i;
    const found = name.match(regex);
    if (found) {
      return found[1];
    }
    return 0;
  }

  _start() {
    const animBeat = this._state.beatDuration;
    const animFps = this._animation.fr;
    const songBpm = this._state.bpm;
    const songBps = songBpm / 60;
    const animBps = animFps / animBeat;
    let animSpeed = songBps / animBps;
    if (animSpeed > 1.5) {
      animSpeed /= 2;
    }
    this.dispatchEvent(new CustomEvent('apply', {
      detail: {
        speed: animSpeed,
      },
    }));
    this._sound.seek(0);
    if (!this._sound.playing()) {
      this._sound.play();
    }
    this._state.loadingState = LOADING_STATES.SUBMITTED;
    this._render();
  }

  pause() {
    if (this._sound) {
      this._sound.pause();
    }
  }

  resume() {
    if (this._sound) {
      this._sound.play();
    }
  }

  rewind() {
    if (this._sound) {
      this._sound.seek(0);
    }
  }

  _onFileChange(event) {
    this._file = event.target.files[0];
    this._state.bpm = this._searchBpmOnName(this._file.name);
    this._state.loadingState = LOADING_STATES.LOADING;
    this._state.bpmCalculationState = LOADING_STATES.LOADING;
    const reader = new FileReader();
    reader.readAsDataURL(this._file);
    reader.addEventListener('load', (ev) => this._onFileDataLoaded(ev), false);
    const arrayBufferReader = new FileReader();
    arrayBufferReader.readAsArrayBuffer(this._file);
    arrayBufferReader.addEventListener('load', (ev) => this._detectBPMs(ev), false);
    this._render();
  }

  async _inputEvent(ev) {
    if (ev.target.id === INPUT_FILE_ID) {
      this._onFileChange(ev);
    } else if (ev.target.id === BPM_ID) {
      this._onBpmChange(ev);
    } else if (ev.target.id === BEAT_DURATION_ID) {
      this._onBeatDurationChange(ev);
    }
    this._render();
  }

  connectedCallback() {
    this._render();
    this.addEventListener('input', this._inputEvent);
  }

  _render() {
    render(template(this), this, { eventContext: this });
  }
}

define('skottie-audio-sk', SkottieAudioSk);
