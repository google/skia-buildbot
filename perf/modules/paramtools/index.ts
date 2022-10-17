// Contains mirror functions to /infra/go/paramtools. See that module for more
// details.
//
// All the validation is done on the server, so these functions do less checking
// on the validity of structured keys and Params.
import { Params, ParamSet } from '../json';

/** Create a structured key from a Params. */
export function makeKey(params: Params): string {
  if (Object.keys(params).length === 0) {
    throw new Error('Params must have at least one entry');
  }
  const keys = Object.keys(params).sort();
  return `,${keys.map((key) => `${key}=${params[key]}`).join(',')},`;
}

/** Parse a structured key into a Params. */
export function fromKey(structuredKey: string): Params {
  const ret: Params = {};
  structuredKey.split(',').forEach((keyValue) => {
    if (!keyValue) {
      return;
    }
    const [key, value] = keyValue.split('=');
    ret[key] = value;
  });
  return ret;
}

/** Add the given Params to the ParamSet. */
export function addParamsToParamSet(ps: ParamSet, p: Params): void {
  Object.entries(p).forEach((keyValue) => {
    const [key, value] = keyValue;
    let values = ps[key];
    if (!values) {
      values = [];
    }
    if (values.indexOf(value) === -1) {
      values.push(value);
    }
    ps[key] = values;
  });
}

export function paramsToParamSet(p: Params): ParamSet {
  const ret: ParamSet = { };

  Object.entries(p).forEach((value: [string, string]) => {
    ret[value[0]] = [value[1]];
  });
  return ret;
}
