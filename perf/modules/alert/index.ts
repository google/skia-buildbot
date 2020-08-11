import { Alert } from '../json';

/**
 * validate returns the empty string if the Alert is valid, otherwise it
 * returns a message explaining why the Alert is invalid.
 */
export function validate(alert: Alert): string {
  if (!alert.query) {
    return 'An alert must have a non-empty query.';
  }
  return '';
}
