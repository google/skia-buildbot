import { Howl, Howler } from 'howler';

// seek tolerance in seconds, keeps the Howl player from seeking unnecessarily
// if the number is too small, Howl.seek() is called too often and creates a popping noise
// too large and audio layers may be skipped over
const kTolerance = 0.75;

/**
 * AudioPlayers wrap a howl and control playback through seek calls
 *
 * @param source - URL or base64 data URI pointing to audio data
 * @param format - only needed if extension is not provided by source (inline URI)
 *
 */
export class AudioPlayer {
  private playing: boolean = false;

  private howl: Howl;

  constructor(source: string) {
    this.howl = new Howl({
      src: [source],
      preload: true,
    });
  }

  pause(): void {
    if (this.playing) {
      this.howl.pause();
      this.playing = false;
    }
  }

  seek(t: number): void {
    if (!this.playing && t >= 0) {
      // Sometimes browsers will prevent the audio from playing.
      // We need to resume the AudioContext or it will never play.
      if (Howler.ctx.state === 'suspended') {
        Howler.ctx.resume().then(() => this.howl.play());
      } else {
        this.howl.play();
      }
      this.playing = true;
    }

    if (this.playing) {
      if (t < 0) {
        this.howl.stop();
        this.playing = false;
      } else {
        const playerPos = this.howl.seek() as number;

        if (Math.abs(playerPos - t) > kTolerance) {
          this.howl.seek(t);
        }
      }
    }
  }

  volume(v: number): void {
    this.howl.volume(v);
  }
}

export class SoundMap {
  private map: Map<string, AudioPlayer> = new Map()

  setPlayer(name: string, player: AudioPlayer): void {
    this.map.set(name, player);
  }

  getPlayer(name: string): AudioPlayer | undefined {
    return this.map.get(name);
  }

  pause(): void {
    for (const player of this.map.values()) {
      player.pause();
    }
  }

  stop(): void {
    for (const player of this.map.values()) {
      player.seek(-1);
    }
  }

  setVolume(v: number): void {
    Howler.volume(v);
  }
}
