/** @module common-sk/modules/stateReflector */
import * as query from './query.js'
import * as object from './object.js'
import { DomReady } from './dom.js'

/** Track the state of an object and reflect it to and from the URL.
 *
 * @example
 *
 * // If an element has a private variable _state:
 * this._state = {"foo": "bar", "count": 7}
 *
 * // then in the connectedCallback() call:
 * this._stateHasChanged = stateReflector(
 *   () => this._state,
 *   (state) => {
 *     this._state = state;
 *     this._render();
 *   }
 * );
 *
 * // And then any time the app changes the value of _state:
 * this._stateHasChanged();
 *
 * @param {function} getState - Function that returns an object representing the state
 *     we want reflected to the URL.
 *
 * @param {function} setState(o) - Function to call when the URL has changed and the state
 *     object needs to be updated. The object 'o' doesn't need to be copied
 *     as it is a fresh object.
 *
 * @returns {function} A function to call when state has changed and needs to be reflected
 *   to the URL.
 */
export function stateReflector(getState, setState) {
  // The default state of the stateHolder. Used to calculate diffs to state.
  let defaultState = object.deepCopy(getState());

  // Have we done an initial read from the the existing query params.
  let loaded = false;

  // stateFromURL should be called when the URL has changed, it updates
  // the state via setState() and triggers the callback.
  const stateFromURL = () => {
    loaded = true;
    let delta = query.toObject(window.location.search.slice(1), defaultState);
    setState(object.applyDelta(delta, defaultState));
  }

  // When we are loaded we should update the state from the URL.
  DomReady.then(stateFromURL);

  // Every popstate event should also update the state.
  window.addEventListener('popstate', stateFromURL);

  // Return a function to call when the state has changed to force reflection into the URL.
  return () => {
    // Don't overwrite the query params until we have done the initial load from them.
    if (!loaded) {
      return;
    }
    let q = query.fromObject(object.getDelta(getState(), defaultState));
    history.pushState(null, '', window.location.origin + window.location.pathname + '?' +  q);
  };
}

