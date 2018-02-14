import * as query from './query.js'
import * as object from './object.js'
import { DomReady } from './dom.js'

// Track the state of an object and reflect it to and from the URL.
//
// stateHolder - An object with a property 'state' where the state to be reflected
//        into the URL is stored. We need the level of indirection because
//        JS doesn't have pointers.
//
//        The 'state' must be on Object and all the values in the Object
//        must be Number, String, Boolean, Object, or Array of String.
//        Doesn't handle NaN, null, or undefined.
//
// cb   - A callback of the form function() that is called when state has been
//        changed by a change in the URL.
//
// Returns
//        A function to call when state has changed and needs to be reflected
//        to the URL.
export function stateReflector(stateHolder, cb) {
  // The default state of the stateHolder. Used to calculate diffs to state.
  let defaultState = object.deepCopy(stateHolder.state);

  // The last state of the stateHolder. Used to determine if the stateHolder state has changed recently.
  let lastState = object.deepCopy(stateHolder.state);

  // stateFromURL should be called when the URL has changed, it updates
  // the stateHolder.state and triggers the callback.
  const stateFromURL = () => {
    let delta = query.toObject(window.location.search.slice(1), defaultState);
    stateHolder.state = object.applyDelta(delta, defaultState);
    lastState = object.deepCopy(stateHolder.state);
    cb();
  }

  // When we are loaded we should update the state from the URL.
  DomReady.then(stateFromURL);

  // Every popstate event should also update the state.
  window.addEventListener('popstate', stateFromURL);

  // Return a function to call when the state has changed to force reflection into the URL.
  return () => {
    if (Object.keys(object.getDelta(lastState, stateHolder.state)).length > 0) {
      lastState = object.deepCopy(stateHolder.state);
      let q = query.fromObject(object.getDelta(stateHolder.state, defaultState));
      history.pushState(null, "", window.location.origin + window.location.pathname + "?" +  q);
    }
  };
}

