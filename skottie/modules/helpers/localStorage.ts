// This module adds a few helper methods around the native localStorage API
// to make it easier to store and retrieve objects.
// Besides wrapping the localStorage native getItem and setItem,
// it also provides methods to get a fully serialized object
// and to write or read attributes from that object.

const localStorage = window.localStorage || {};

const _localStorage = {
  get: (key: string): string|null => localStorage.getItem(key),
  set: (key: string, value: string): void => localStorage.setItem(key, value),
  // Gets a serialized object from localStorage and parses it
  getObject: (key: string): Record<string, unknown> => {
    try {
      const v = _localStorage.get(key);
      if (!v) {
        return {};
      }
      return JSON.parse(v);
    } catch (err) {
      console.error('err', err);
      return {};
    }
  },
  // Serializes and object and stores it on localStorage
  setObject: (key: string, object: Record<string, unknown>): void => {
    try {
      const serializedData = JSON.stringify(object);
      _localStorage.set(key, serializedData);
    } catch (err) {
      console.log(err);
    }
  },
  // Gets a value from a serialized object.
  // If the attribute does not exist, it returns the defaultValue passed as argument.
  getValueFromObject: (objectKey: string, key: string, defaultValue: unknown) => {
    const object = _localStorage.getObject(objectKey);
    return object[key] !== undefined ? object[key] : defaultValue;
  },
  // Sets a value on an object and serializes it to save it on the localStorage.
  setValueInObject: (objectKey: string, key: string, value: unknown): void => {
    const gifData = _localStorage.getObject(objectKey);
    gifData[key] = value;
    _localStorage.setObject(objectKey, gifData);
  },
};

export default _localStorage;
