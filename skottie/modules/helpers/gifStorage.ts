// This module allows the setting/retrieval of gif-specific fields from the local storage

import localStorage from './localStorage';

const storageKey = 'skottie-gif-storage';

const gifStorage = {
  set: <T>(key: string, value: T): void => localStorage.setValueInObject(storageKey, key, value),
  get: <T>(key: string, defaultValue: T): T => localStorage.getValueFromObject(storageKey, key, defaultValue),
};

export default gifStorage;
