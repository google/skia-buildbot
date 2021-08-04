// This module allows the setting/retrieval of gif-specific fields from the local storage

import localStorage from './localStorage';

const storageKey = 'skottie-gif-storage';

const gifStorage = {
  set: (key: string, value: unknown): void => localStorage.setValueInObject(storageKey, key, value),
  get: (key: string, defaultValue: unknown): unknown => localStorage.getValueFromObject(storageKey, key, defaultValue),
};

export default gifStorage;
