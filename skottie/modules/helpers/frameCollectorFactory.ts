/**
 * This helper factory generates an instance of a class that traverses a skottie animation
 * and stores every frame in a ffmpeg virtual file as pngs
 *
 */

import { SkottiePlayerSk } from '../skottie-player-sk/skottie-player-sk';
import { FFmpeg, fetchFile } from '@ffmpeg/ffmpeg';
import delay from './delay';

export type FrameCollectorType = {
  start: () => void;
  stop: () => void;
  player: SkottiePlayerSk | null;
};

type Progress = (message: string) => void;

class FrameCollector {
  private _player: SkottiePlayerSk | null = null;
  private _ffmpeg: FFmpeg | null = null;
  private _isRunning: boolean = false;
  private _onProgress: Progress = () => {};

  constructor(ffmpeg: FFmpeg | null = null, onProgress: Progress | null) {
    this._ffmpeg = ffmpeg;
    this._onProgress = onProgress || this._onProgress;
  }

  get player(): SkottiePlayerSk | null {
    return this._player;
  }

  set player(player: SkottiePlayerSk | null) {
    this._player = player;
  }

  async start(): Promise<void> {
    this._isRunning = true;
    const player = this._player;
    const ffmpeg = this._ffmpeg;
    if (!player || !ffmpeg) {
      return;
    }
    const fps = player.fps();
    const duration = player.duration();
    const canvasElement = player.canvas()!;
    let currentTime = 0;
    let counter = 1;
    const increment = 1000 / fps;
    while (currentTime < duration) {
      if (!this._isRunning) {
        return;
      }
      // This delay helps to maintain the browser responsive during export.
      await delay(1); // eslint-disable-line no-await-in-loop
      player.seek(currentTime / duration, true);
      const canvasData = canvasElement?.toDataURL();
      ffmpeg.FS(
        'writeFile',
        `tmp_${String(counter).padStart(4, '0')}.png`,
        await fetchFile(canvasData)
      );
      currentTime += increment;
      this._onProgress(`Creating frame ${counter}`);
      counter += 1;
    }
  }

  stop(): void {
    this._isRunning = false;
  }
}

const frameCollectorFactory = (
  ffmpeg: FFmpeg | null,
  onProgress: Progress
): FrameCollectorType => {
  return new FrameCollector(ffmpeg, onProgress);
};

export default frameCollectorFactory;
