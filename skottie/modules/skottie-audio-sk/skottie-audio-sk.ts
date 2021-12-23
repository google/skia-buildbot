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
import { html } from 'lit-html';
import { Howl } from 'howler';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { LottieAnimation } from '../types';

const INPUT_FILE_ID = 'fileInput';
const BPM_ID = 'bpm';
const BEAT_DURATION_ID = 'beatDuration';
const START_BUTTON_ID = 'startButton';

type LoadingState = 'idle' | 'loading' | 'loaded' | 'submitted';

export interface AudioStartEventDetail {
  speed: number;
}

interface animationMarker {
  cm: string;
}

interface markerData {
  beat?: number;
  type: string;
}

interface tempoInterval {
  count: number;
  tempo: number;
}

interface tempoPeak {
  volume: number;
  position: number;
}

export class SkottieAudioSk extends ElementSk {
  private static template = (ele: SkottieAudioSk) => html`
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
          <checkbox-sk label="Loop"
            ?checked=${ele.shouldLoop}
            @click=${ele.toggleLoop}>
          </checkbox-sk>
          <label class=input-label>
            <input
              type=number
              id=${BPM_ID}
              .value=${ele.bpm}
              required
            /> BPM
          </label>
          ${ele.bpmListTemplate()}
          <label class=input-label>
            <input
              type=number
              id=${BEAT_DURATION_ID}
              .value=${ele.beatDuration}
              required
            /> Beat Duration (in frames)
          </label>
        </div>
        ${ele.startButtonTemplate()}
    <section>
  </div>
`;

  private startButtonTemplate = () => {
    if (this.loadingState === 'idle' || this.loadingState === 'submitted'
    ) {
      return null;
    }
    if (this.loadingState === 'loading' || this.bpmCalculationState === 'loading') {
      return html`<div>Loading...</div>`;
    }
    return html`
    <button
    class=start
    id=${START_BUTTON_ID}
    @click=${this.start}
    >Start</button>
  `;
  };

  private static bpmListOptionTemplate = (option: tempoInterval, onClick: (t: number)=> void) => html`
  <li class=bpm-options-item>
    <button class=bpm-options-item-button
      @click=${() => onClick(option.tempo)}
    >
      ${option.tempo}
    </button>
  </li>
`;

  private bpmListTemplate = () => html`
  <ul class=bpm-options>
    ${this.bmpList.map(((b: tempoInterval) => SkottieAudioSk.bpmListOptionTemplate(
    b,
    (option: number) => this.onBpmSelected(option),
  )))}
  </ul>
`;

  private _animation: LottieAnimation | null = null;

  private beatDuration: number = 0;

  private bmpList: tempoInterval[] = [];

  private bpm: number = 0;

  private bpmCalculationState: LoadingState = 'idle';

  private file: File | null = null;

  private loadingState: LoadingState = 'idle';

  private shouldLoop: boolean = true;

  private sound: Howl | null = null;

  constructor() {
    super(SkottieAudioSk.template);
  }

  set animation(val: LottieAnimation) {
    if (this._animation !== val) {
      this._animation = val;
      this.updateAnimation(val);
    }
  }

  private updateAnimation(animation: LottieAnimation): void {
    const markers = (animation.markers || []) as animationMarker[];
    const marker = markers.find((markerItem: animationMarker) => {
      try {
        const md = JSON.parse(markerItem.cm) as markerData;
        if (md.type === 'beat') {
          return true;
        }
      } catch (err) {
        // Marker does not have beat information
      }
      return false;
    });
    if (marker) {
      const payload = JSON.parse(marker.cm) as markerData;
      if (payload.beat) {
        this.beatDuration = payload.beat;
        this._render();
      }
    }
  }

  private static getPeaks(data: Float32Array[]): tempoPeak[] {
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
    let peaks: tempoPeak[] = [];

    for (let i = 0; i < parts; i++) {
      let max: tempoPeak | null = null;
      for (let j = i * partSize; j < (i + 1) * partSize; j++) {
        const volume = Math.max(Math.abs(data[0][j]), Math.abs(data[1][j]));
        if (!max || (volume > max.volume)) {
          max = {
            position: j,
            volume: volume,
          };
        }
      }
      peaks.push(max!);
    }
    // We then sort the peaks according to volume...
    peaks.sort((a: tempoPeak, b: tempoPeak) => b.volume - a.volume);
    // ...take the loundest half of those...
    peaks = peaks.splice(0, peaks.length * 0.5);
    // ...and re-sort it back based on position.
    peaks.sort((a: tempoPeak, b: tempoPeak) => a.position - b.position);
    return peaks;
  }

  private getIntervals(peaks: tempoPeak[]) {
    // What we now do is get all of our peaks, and then measure the distance to
    // other peaks, to create intervals.  Then based on the distance between
    // those peaks (the distance of the intervals) we can calculate the BPM of
    // that particular interval.
    // The interval that is seen the most should have the BPM that corresponds
    // to the track itself.
    const groups: tempoInterval[] = [];

    peaks.forEach((peak: tempoPeak, index: number) => {
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

        const groupTempo = groups.find((interval: tempoInterval) => interval.tempo === group.tempo);
        if (!groupTempo) {
          groups.push(group);
        } else {
          groupTempo.count += 1;
        }
      }
    });
    return groups;
  }

  private onOfflineRenderComplete(e: OfflineAudioCompletionEvent): void {
    const buffer = e.renderedBuffer;
    const peaks = SkottieAudioSk.getPeaks([buffer.getChannelData(0), buffer.getChannelData(1)]);
    const groups = this.getIntervals(peaks);

    const top = groups.sort((intA: tempoInterval, intB: tempoInterval) => intB.count - intA.count).splice(0, 5);
    this.bmpList = top;
    if (!this.bpm) {
      this.bpm = top[0].tempo;
    }
    this.bpmCalculationState = 'loaded';
    this._render();
  }

  // It looks for peaks on the audio to estimate best guesses of the bpm
  // more info about it https://jmperezperez.com/bpm-detection-javascript/
  private detectBPMs(ev: ProgressEvent<FileReader>): void {
    const track = ev.target!.result as ArrayBuffer;

    // Create offline context
    const offlineContext = new OfflineAudioContext(2, 30 * 44100, 44100);

    offlineContext.decodeAudioData(track, (buffer: AudioBuffer) => {
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

    offlineContext.oncomplete = (e: OfflineAudioCompletionEvent) => this.onOfflineRenderComplete(e);
  }

  private onBpmSelected(option: number): void {
    this.bpm = option;
    if (this.loadingState === 'submitted') {
      this.start();
    } else {
      this._render();
    }
  }

  private onFileDataLoaded(ev: ProgressEvent<FileReader>): void {
    const result = ev.target!.result as string;
    if (this.sound) {
      this.sound.unload();
    }
    this.sound = new Howl({
      src: [result],
      loop: this.shouldLoop,
    });
    this.sound.on('load', () => this.onAudioLoaded());
  }

  private onBpmChange(ev: Event): void {
    ev.preventDefault();
    const value = parseInt((ev.target as HTMLInputElement).value, 10);
    if (this.bpm !== value) {
      this.bpm = value;
      if (this.loadingState === 'submitted') {
        this.start();
      } else {
        this._render();
      }
    }
  }

  private onBeatDurationChange(ev: Event): void {
    ev.preventDefault();
    const value = +(ev.target as HTMLInputElement).value;
    if (this.beatDuration !== value) {
      this.beatDuration = value;
    }
    this._render();
  }

  private toggleLoop(ev: Event): void {
    ev.preventDefault();
    this.shouldLoop = !this.shouldLoop;
    if (this.sound) {
      this.sound.loop(this.shouldLoop);
    }
    this._render();
  }

  private onAudioLoaded(): void {
    this.loadingState = 'loaded';
    this._render();
  }

  // It looks for a bpm as part of the file name.
  // The format of the name should be [name]bpm_[number]
  private static searchBpmOnName(name: string): number {
    const regex = /bpm_([0-9]*)/i;
    const found = name.match(regex);
    if (found) {
      return +found[1];
    }
    return 0;
  }

  private start(): void {
    const animBeat = this.beatDuration;
    const animFps = this.animation.fr as number;
    const songBpm = this.bpm;
    const songBps = songBpm / 60;
    const animBps = animFps / animBeat;
    let animSpeed = songBps / animBps;
    if (animSpeed > 1.5) {
      animSpeed /= 2;
    }
    this.dispatchEvent(new CustomEvent<AudioStartEventDetail>('apply', {
      detail: {
        speed: animSpeed,
      },
    }));
    if (this.sound) {
      this.sound.seek(0);
      if (!this.sound.playing()) {
        this.sound.play();
      }
    }
    this.loadingState = 'submitted';
    this._render();
  }

  pause(): void {
    if (this.sound) {
      this.sound.pause();
    }
  }

  resume(): void {
    if (this.sound) {
      this.sound.play();
    }
  }

  rewind(): void {
    if (this.sound) {
      this.sound.seek(0);
    }
  }

  private onFileChange(ev: Event): void {
    this.file = (ev.target as HTMLInputElement).files![0];
    this.bpm = SkottieAudioSk.searchBpmOnName(this.file.name);
    this.loadingState = 'loading';
    this.bpmCalculationState = 'loading';
    const reader = new FileReader();
    reader.readAsDataURL(this.file);
    reader.addEventListener('load', (e: ProgressEvent<FileReader>) => this.onFileDataLoaded(e), false);
    const arrayBufferReader = new FileReader();
    arrayBufferReader.readAsArrayBuffer(this.file);
    arrayBufferReader.addEventListener('load', (e: ProgressEvent<FileReader>) => this.detectBPMs(e), false);
    this._render();
  }

  async inputEvent(ev: Event): Promise<void> {
    const target = ev.target as HTMLInputElement;
    if (target.id === INPUT_FILE_ID) {
      this.onFileChange(ev);
    } else if (target.id === BPM_ID) {
      this.onBpmChange(ev);
    } else if (target.id === BEAT_DURATION_ID) {
      this.onBeatDurationChange(ev);
    }
    this._render();
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.addEventListener('input', this.inputEvent);
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    this.removeEventListener('input', this.inputEvent);
  }
}

define('skottie-audio-sk', SkottieAudioSk);
