import {Howl, Howler} from 'howler';

// SoundMaps have string : player pairs
export function SoundMap() {
  this.map = new Map();
  this.setPlayer = function(name, player) {
    if (typeof name == "string" && player.hasOwnProperty('seek')) {
      this.map.set(name, player);
    }
  };
  this.getPlayer = function(name) {
    return this.map.get(name);
  };
  this.pause = function() {
    for(const player of this.map.values()) {
      player.seek(-1);
    }
  }
}

/**
 * AudioPlayers wrap a howl and control playback through seek calls
 *
 * @param source - URL or base64 data URI pointing to audio data
 * @param format - only needed if extention is not provided by source (inline URI)
 *
 */
export function AudioPlayer(source) {
  this.playing = false;
  this.howl = new Howl({
    src: [source],
    preload: true
  });
  this.seek = function(t) {
    if (!this.playing && t >=0) {
      this.howl.play();
      this.playing = true;
    }

    if (this.playing) {
      if (t < 0) {
        this.howl.pause();
        this.playing = false;
      } else {
        let kTolerance = 0.075;
        let playerPos = this.howl.seek();

        if (Math.abs(playerPos - t) > kTolerance) {
          this.howl.seek(t);
        }
      }
    }
  };
}
