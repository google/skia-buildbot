// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/** @module common-sk/modules/stateReflector */
import * as query from './query';
import * as object from './object';
import { DomReady } from './dom';
import { HintableObject } from './hintable';

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
 * @param getState - Function that returns an object representing the state
 *     we want reflected to the URL.
 *
 * @param setState(o) - Function to call when the URL has changed and the state
 *     object needs to be updated. The object 'o' doesn't need to be copied
 *     as it is a fresh object.
 *
 * @returns A function to call when state has changed and needs to be reflected
 *   to the URL.
 */
export function stateReflector(
  getState: () => HintableObject,
  setState: (o: HintableObject) => void
): () => void {
  // The default state of the stateHolder. Used to calculate diffs to state.
  const defaultState = object.deepCopy(getState());

  // Have we done an initial read from the the existing query params.
  let loaded = false;

  // stateFromURL should be called when the URL has changed, it updates
  // the state via setState() and triggers the callback.
  const stateFromURL = () => {
    loaded = true;
    const delta = query.toObject(window.location.search.slice(1), defaultState);
    setState(object.applyDelta(delta, defaultState));
  };

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

    const new_state = object.getDelta(getState(), defaultState);
    const old_state = query.toObject(
      window.location.search.slice(1),
      defaultState
    );

    const new_delta = object.getDelta(new_state, old_state);
    const old_delta = object.getDelta(old_state, new_state);

    // Don't push to state if the current URL and the URL to be pushed are equivalent.
    if (
      Object.keys(new_delta).length > 0 ||
      Object.keys(old_delta).length > 0
    ) {
      const q = query.fromObject(new_state);
      history.pushState(
        null,
        '',
        `${window.location.origin + window.location.pathname}?${q}`
      );
    }
  };
}
