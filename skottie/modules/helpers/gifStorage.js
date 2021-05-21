// This module allows the setting/retrieval of gif-specific fields from the local storage

import localStorage from './localStorage';

const storageKey = 'skottie-gif-storage';

const gifStorage = {
  set: (key, value) => localStorage.setValueInObject(storageKey, key, value),
  get: (key, defaultValue) => localStorage.getValueFromObject(storageKey, key, defaultValue),
};

export default gifStorage;
