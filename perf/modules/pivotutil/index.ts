import { pivot } from '../json/all';

/** Maps pivot.Operations to human readable names. */
export const operationDescriptions: {[key in pivot.Operation]: string} = {
  sum: 'Sum',
  avg: 'Mean',
  geo: 'Geometric Mean',
  std: 'Standard Deviation',
  count: 'Count',
  min: 'Minimum',
  max: 'Maximum',
};

/** Returns a non-empty string with the error message if the pivot.Request is
 * invalid.
 */
export function validatePivotRequest(req: pivot.Request | null): string {
  if (!req) {
    return 'Pivot request is null.';
  }
  if (!req.group_by || req.group_by.length === 0) {
    return 'Pivot must have at least one GroupBy.';
  }
  return '';
}

/** Returns a non-empty string with the error message if the pivot.Request is
 * invalid or would not result in a pivot table, i.e. it would only result in a
 * pivot plot as there are no summary operations to turn traces into summary
 * values.
 */
export function validateAsPivotTable(req: pivot.Request | null): string {
  const invalid = validatePivotRequest(req);
  if (invalid) {
    return invalid;
  }
  if (!req!.summary || req!.summary.length === 0) {
    return 'Must have at least one Summary operation.';
  }
  return '';
}
