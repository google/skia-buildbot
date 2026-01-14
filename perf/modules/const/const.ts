/** Constants used across the UI of Perf. */

/** MISSING_DATA_SENTINEL is the value the backend uses to indicate that a
 *  sample is missing from a trace. This must be set to the same value as the
 *  const MissingDataSentinel in //go/vec32/vec.
 *
 *  JSON doesn't support NaN or +/- Inf, so we need a valid float32 to signal
 *  missing data that also has a compact JSON representation.
 */
export const MISSING_DATA_SENTINEL = 1e32;

export const MISSING_VALUE_SENTINEL = '__missing__';
