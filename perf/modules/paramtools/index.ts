// Contains mirror functions to /infra/go/paramtools. See that module for more
// details.
//
// All the validation is done on the server, so these functions do less checking
// on the validity of structured keys and Params.
import { Params, ParamSet, ReadOnlyParamSet } from '../json';

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

/** Checks that the trace id isn't a calculation or special_* trace. */
export function validKey(key: string): boolean {
  return key.startsWith(',') && key.endsWith(',');
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
  const ret: ParamSet = {};

  Object.entries(p).forEach((value: [string, string]) => {
    ret[value[0]] = [value[1]];
  });
  return ret;
}

/** addParamSet adds the ParamSet or ReadOnlyParamSet to this ParamSet. */
export function addParamSet(
  p: ParamSet,
  ps: ParamSet | ReadOnlyParamSet
): void {
  for (const [k, arr] of Object.entries(ps)) {
    if (!p[k]) {
      p[k] = arr.slice(0);
    } else {
      for (const v of arr) {
        if (!p[k].includes(v)) {
          p[k].push(v);
        }
      }
    }
  }
}
