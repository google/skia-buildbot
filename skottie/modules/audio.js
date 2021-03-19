import {Howl, Howler} from 'howler';

// seek tolerance in seconds, keeps the Howl player from seeking unnecessarily
// if the number is too small, Howl.seek() is called too often and creates a popping noise
// too large and audio layers may be skipped over
const kTolerance = 0.75;

// SoundMaps have string : player pairs
export function SoundMap() {
  this.map = new Map();
  this.setPlayer = function(name, player) {
    if (typeof name == 'string' && player.hasOwnProperty('seek')) {
      this.map.set(name, player);
    }
  };
  this.getPlayer = function(name) {
    return this.map.get(name);
  };
  this.pause = function() {
    for(const player of this.map.values()) {
      player.pause();
    }
  }
  this.stop = function() {
    for(const player of this.map.values()) {
      player.seek(-1);
    }
  }
  this.setVolume = function(v) {
    Howler.volume(v)
  }
}

/**
 * AudioPlayers wrap a howl and control playback through seek calls
 *
 * @param source - URL or base64 data URI pointing to audio data
 * @param format - only needed if extension is not provided by source (inline URI)
 *
 */
export function AudioPlayer(source) {
  this.playing = false;
  this.howl = new Howl({
    src: [source],
    preload: true
  });
  this.pause = function() {
    if(this.playing) {
      this.howl.pause();
      this.playing = false
    }
  }
  this.seek = function(t) {
    if (!this.playing && t >= 0) {
      // Sometimes browsers will prevent the audio from playing.
      // We need to resume the AudioContext or it will never play.
      if (Howler.ctx.state == "suspended") {
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
        const playerPos = this.howl.seek();

        if (Math.abs(playerPos - t) > kTolerance) {
          this.howl.seek(t);
        }
      }
    }
  };
  this.volume = function(v) {
    this.howl.volume(v);
  };
}
