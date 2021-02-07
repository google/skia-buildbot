/** The number of measurements we gather before calculating the fps. */
export const numMeasurements = 30;

/**
 * Measures the fps (frames per second) while drawing.
 *
 * The 'raf()' method should be called in every requestAnimationFrame callback.
 */
export class FPS {
    private timestamps: number[] = []; // timestamps of raf() calls (ms).

    private _fps: number = 0;

    raf(): void {
      if (this.timestamps.length >= numMeasurements) {
        let total = 0;
        for (let i = 1; i < this.timestamps.length; i++) {
          total += this.timestamps[i] - this.timestamps[i - 1];
        }
        this._fps = 1000 / (total / (this.timestamps.length - 1));
        this.timestamps = [];
      }
      this.timestamps.push(performance.now());
    }

    get fps(): number { return this._fps; }
}
