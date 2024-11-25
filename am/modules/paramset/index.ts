import { Params, ParamSet } from '../json';

/**
 * Add some params to a ParamSet.
 *
 * @param paramset - Key-value pairs where keys are strings and values are arrays of strings.
 * @param params - Key-value pairs where both keys and values are strings.
 * @param ignored - List of keys to ignore.
 */
export function add(paramset: ParamSet, params: Params, ignored: string[] = []): void {
  Object.keys(params).forEach((key) => {
    if (ignored.includes(key)) {
      return;
    }
    const value = params[key];
    const values = paramset[key] || [];
    if (!values.includes(value)) {
      values.push(value);
      paramset[key] = values;
    }
  });
}

/**
 * Determines if the params given match the ParamSet.
 *
 * @param paramset - Key-value pairs where keys are strings and values are arrays of strings
 *   or regexes (see skbug.com/9587).
 * @param params - Key-value pairs where both keys and values are strings.
 *
 * @return True if every key in ParamSet is present in 'params' and
 *   the value seen in params is in the ParamSet values or it matches one of
 *   the regexes.
 */
export function match(paramset: ParamSet, params: Params): boolean {
  const keys = Object.keys(paramset);
  for (let i = 0; i < keys.length; i++) {
    const key = keys[i];
    if (paramset[key]!.includes(params[key])) {
      continue;
    }
    const values = paramset[key] || [];
    let valMatched = false;
    for (let j = 0; j < values.length; j++) {
      const re = new RegExp(`^${values[j]}$`);
      if (re.test(params[key])) {
        valMatched = true;
        break;
      }
    }
    if (!valMatched) {
      return false;
    }
  }
  return true;
}
