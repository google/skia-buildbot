import { $, $$ } from 'common-sk/modules/dom'

import { SCHEMA } from './schema'

let currentJSON = {};

// Call setupListeners with the container element of the JSON editor
// after it has been created with new JSONEditor(container, options).
// setupListeners adds listeners to respond to the user interacting
// with the editor (e.g. expanding/contracting menus).
export function setupListeners(container) {
  // Add to first element child of container, that is, the actual
  // editor HTML.
  container.firstElementChild.addEventListener('click', () => {
    // When an object or array is expanded, new rows are created
    // (which don't have annotations), so
    // There's no good event for "thing was expanded", so
    // we just wait for any clicks and check again.
    reannotate(container);
  });
}

// Call onUserEdit after an event has been received that the user
// has modified the JSON. See annoations-demo.html for more.
export function onUserEdit(container, json) {
  if (!json) {
    throw 'The updated JSON must be given for accurate annotations';
  }
  $('.dynamic_annotation', container).forEach((e) => {
    e.remove();
  });
  reannotate(container, json);
}

// Call reannotate once after the JSON editor has been initially
// set. If setupListeners was used, then clients should only have
// to call reannoate once for the initial annotations.
// It is required to pass in the same JSON object that was given
// to the editor on this initial call. The JSON then only need be
// given if it has changed. The passed in JSON will not be modified
// and thus need not be copied before passing in.
export function reannotate(container, json) {
  if (!json && !currentJSON) {
    throw 'The initial JSON must be given for accurate annotations';
  }
  if (json) {
    currentJSON = json;
  }
  // .jsoneditor-values is (confusingly) the wrapper around the key:value pair.
  // Iterate through all of them and go inside to see if there are any keys
  // (e.g. jsoneditor-field) that could possibly be annotated.
  let rows = $('.jsoneditor-values', container);
  rows.forEach((row) => {
    let key = $$('.jsoneditor-field', row);
    if (key) {
      // parent is where we want to add an extra <td> with our annotation.
      let parent = key.closest('tr');
      // Don't re-annotate something with an annotation.
      if (!$$('.annotation', parent)) {
        let [comment, isDynamic] = getAnnotation(key);
        if (comment) {
          let newAnno = document.createElement('td');
          newAnno.innerHTML = `<div class='jsoneditor-value annotation'>// ${comment}</div>`;
          if (isDynamic) {
            newAnno.classList.add('dynamic_annotation');
          }
          parent.appendChild(newAnno);
        }
      }
    }
  });
}

// This resolves the given key with the given AnnotationMap
// (i.e. getting the AnnotationMap from the passed in function if necessary)
// and then returns [Annotator, JSON] or [null, null] if there
// is no more annotation down this path.
function unwrap(annoMap, key, json) {
    let newAnno = null;
    if (typeof annoMap === 'function') {
      newAnno = annoMap(json)[key]
    } else {
      newAnno = annoMap[key];
    }
    if (newAnno) {
      return [newAnno, json[key]]
    }
    return [null, null];
}

// Returns the Annotator and the JSON for a given HTML node as an array.
// The node is expected to be associated with a key rendered by
// jseditor, which tends to be "div.jsoneditor-field".
// This will use recursion to find a path to the root (which aligns with
//  the SCHEMA object) and then backtrack down through SCHEMA to get
// the appropriate Annotator and JSON.
// See schema.js for information about Annotator and SCHEMA.
// The JSON returned will be the subtree which corresponds to the given node.
function getAnnotator(node) {
  // jsoneditor presents the data as a whole bunch of <tr>, so essentially
  // flattening the data. This makes finding the parent element a bit tricky
  // because we can't just look for the row above us that can be expanded and
  // call it a day. We have to check to see if that expandable node is a sibling
  // and the most reliable way to do that is to look for how much it is indented,
  // relying on the fact that jsoneditor sets the margin-left to be monotonically
  // increasing values as layers of indents go up and up.
  let values = node.closest('table.jsoneditor-values');
  if (!values) {
    // Can't find wrapper for this node.
    console.error('Did not expect to get here. Bailing out.');
    return [null, null];
  }
  let thisIndent = parseInt(values.style['margin-left']);
  // jsoneditor doesn't give us a good way
  let thisKey = node.innerText;

  // Now that we know our indent, let's start iterating over the rows above
  // this node looking for a possible parent, that is, anything that can be expanded.
  let row = values.closest('tr');
  let possibleParent = row.previousSibling;
  while (true) {
    while (possibleParent && !possibleParent.classList.contains('jsoneditor-expandable')) {
      possibleParent = possibleParent.previousSibling;
    }
    if (!possibleParent || !possibleParent.previousSibling) {
      // recursion base case, we are at the root of the HTML and therefore,
      // the root of the JSON.
      return unwrap(SCHEMA, thisKey, currentJSON);
    }
    // we might have found our parent. Look for the value and compare the margin-left.
    // .jsoneditor-values is (confusingly) the wrapper around the key:value pair
    // and is indented according to the nestedness.
    let possibleParentValue = $$('.jsoneditor-values', possibleParent);
    if (!possibleParentValue) {
      // can't get parent's wrapper?
      console.error('Did not expect to get here. Bailing out.');
      return [null, null];
    }
    let possibleParentIndent = parseInt(possibleParentValue.style['margin-left']);
    if (possibleParentIndent < thisIndent) {
      // We have found our parent. However, we don't know where this node
      // or the parent node sits with respect to the root. Recursion will
      // tell us that.
      let parentKey = $$('.jsoneditor-field', possibleParentValue);
      // parentKey is the HTML node referring to the key of the parent
      // (or null if this node is in an array);
      if (!parentKey) {
        parentKey = $$('.jsoneditor-readonly', possibleParentValue);
        if (!parentKey) {
          // parent isn't an object or an array?
          console.error('Did not expect to get here. Bailing out.');
          return [null, null];
        }
      }
      let [parentAnnotation, parentJSON] = getAnnotator(parentKey);
      if (!parentAnnotation) {
        return [null, null];
      }
      // Arrays just have one _children object that all of the children use, so
      // short-circuit to that.
      if (parentAnnotation._is_array) {
        // Arrays get tricky because there's an extra level
        // e.g. arr[0].obj has an extra step of the [0] part.
        // To keep things clean, we return the _children AnnotationMap (not
        // the promised Annotator) and know that the caller will be
        // able to properly handle this (since they have the key).
        return [parentAnnotation._children, parentJSON[parseInt(thisKey)]];
      }

      if (parentAnnotation._children) {
        // If there's a _children object, attempt to find the annotation
        // for this node in there.
        return unwrap(parentAnnotation._children, thisKey, parentJSON);
      }
      // Subtle note, parentAnnotation is an AnnotationMap,
      // not an Annotator (see the _is_array logic above)
      return unwrap(parentAnnotation, thisKey, parentJSON)
    }
    // Nope, possibleParent was actually a sibling (they have the same indent)
    // try the next one up
    possibleParent = possibleParent.previousSibling;
  }
}

// getAnnotation returns a two element array of [String, Boolean]
// for a given HTML node. The node is expected to be associated
// with a key rendered by jseditor
// The first return value is the text that annotates the node
// and the second value is if the text is dynamic and should
// be updated if the JSON gets edited.
function getAnnotation(node) {
  let [anno, value] = getAnnotator(node);
  if (anno && anno._annotation) {
    anno = anno._annotation;
    // anno is either a String or a function that returns a String
    // based on the value of the given key. Call it if necessary.
    if (anno instanceof Function) {
      return [anno(value), true];
    }
    return [anno, false];
  }
  return [null, false];
}
