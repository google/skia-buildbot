const localStorage = window.localStorage || {};

const _localStorage = {
  get: (key) => localStorage.getItem(key),
  set: (key, value) => localStorage.setItem(key, value),
  getObject: (key) => {
    try {
      return JSON.parse(_localStorage.get(key)) || {};
    } catch (err) {
      console.log('err', err);
      return {};
    }
  },
  setObject: (key, object) => {
    try {
      const serializedData = JSON.stringify(object);
      _localStorage.set(key, serializedData);
    } catch (err) {
      console.log(err);
    }
  },
  getValueFromObject: (objectKey, key, defaultValue) => {
    const object = _localStorage.getObject(objectKey);
    return object[key] !== undefined ? object[key] : defaultValue;
  },
  setValueInObject: (objectKey, key, value) => {
    const gifData = _localStorage.getObject(objectKey);
    gifData[key] = value;
    _localStorage.setObject(objectKey, gifData);
  },
};

export default _localStorage;
