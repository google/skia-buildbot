// Contains mirror functions to /infra/go/query.go for structured keys, see that
// module for more details.
//
// All the validation is done on the server, so these functions do not do any
// checking on the validity of structured keys.
import { Params } from '../json';

export function makeKey(params: Params): string {
  if (Object.keys(params).length === 0) {
    throw new Error('Params must have at least one entry');
  }
  const keys = Object.keys(params).sort();
  return `,${keys.map((key) => `${key}=${params[key]}`).join(',')},`;
}
