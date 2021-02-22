/** Describes a single shader uniform, which may be a single float value, or a 2
 * dimensional value such as float4x4.
 */
export interface Uniform {
  name: string;
  rows: number;
  columns: number;

  /**
   * The location in the uniforms value array that this uniform starts.
   *
   * Note that a uniform occupies rows*columns spots in the uniform value array.
   * Note also that the values are in column major order.
   */
  slot: number;
}

/** The common interface that all controls for editing uniforms must implement. */
export interface UniformControl {
  uniform: Uniform;

  /** Copies the values of the control into the uniforms array. */
  applyUniformValues(uniforms: number[]): void;

  /** Copies the values from the uniforms array into the control. */
  restoreUniformValues(uniforms: number[]): void;

  /** Function to call on every requestAnimationFrame. Only called if needsRAF() returns true. */
  onRAF(): void;

  /** Returns true if this controls needs to update on every requestAnimationFrame, such as uniform-time-sk. */
  needsRAF(): boolean;
}
