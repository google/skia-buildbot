import { $, $$ } from 'common-sk/modules/dom'

import { SCHEMA } from './schema'

let currentJSON = {};

export function setupListeners(container, json) {
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

export function onUserEdit(container, json) {
  if (!json) {
    throw 'The updated JSON must be given for accurate annotations';
  }
  $('.dynamic_annotation', container).forEach((e) => {
    e.remove();
  });
  reannotate(container, json);
}

export function reannotate(container, json) {
  if (!json && !currentJSON) {
    throw 'The initial JSON must be given for accurate annotations';
  }
  if (json) {
    currentJSON = json;
  }
  console.time('reannotate');
  let rows = $('.jsoneditor-values', container);
  rows.forEach((row) => {
    let key = $$('.jsoneditor-field', row);
    if (key) {
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
  console.timeEnd('reannotate');
}

// Get the annotation object and the JSON for a given node. This will use recursion
// to find a path to the parent (which uses the SCHEMA object)
// and then backtrack finding the last leaf object.
// annotation object looks like:
// {
//    _annotation: 'text about this node',
//    children: [optional] Object that maps keys -> annotation object
//              where each key is a possible child.
// }
// The JSON returned will be the subtree which corresponds to the given node.
function getAnnotationObj(node) {
  // jsoneditor presents the data as a whole bunch of <tr>, so essentially
  // flattening the data. This makes finding the parent element a bit tricky
  // because we can't just look for the row above us that can be expanded and
  // call it a day. We have to check to see if that expandable node is a sibling
  // and the most reliable way to do that is to look for how much it is indented,
  // relying on the fact that jsoneditor sets the margin-left to be monotonically
  // increasing values as layers of indents go up and up.
  let values = node.closest('table.jsoneditor-values');
  let thisKey = node.innerText;
  if (!values) {
    console.error('did not expect to get here.  Bailing out')
    return [null, null];
  }
  let thisIndent = parseInt(values.style['margin-left']);

  // The source object can have a function that takes the JSON as an argument
  // and returns the expected annotation object. This wrapper function makes the
  // appropriate call if necessary.
  const w = (anno, key, j) => {
    let newAnno = null;
    if (typeof anno === 'function') {
      newAnno = anno(j)[key]
    } else {
      newAnno = anno[key];
    }
    if (newAnno) {
      return [newAnno, j[key]]
    }
    return [null, null];
  }

  let row = values.closest('tr');
  let possibleParent = row.previousSibling;
  while (true) {
    while (possibleParent && !possibleParent.classList.contains('jsoneditor-expandable')) {
      possibleParent = possibleParent.previousSibling;
    }
    if (!possibleParent || !possibleParent.previousSibling) {
      // bail out, we are at the top.  Might also need to check for "lottie" special key
      return w(SCHEMA, thisKey, currentJSON);
    }
    // we might have found our parent. Look for the value and check the margin-left.
    let possibleParentValue = $$('.jsoneditor-values', possibleParent);
    if (!possibleParentValue) {
      console.error('did not expect to get here.  Bailing out')
      return [null, null];
    }
    let possibleParentIndent = parseInt(possibleParentValue.style['margin-left']);
    if (possibleParentIndent < thisIndent) {
      let parentKey = $$('.jsoneditor-field', possibleParentValue);
      if (parentKey) {
        let [parentAnnotation, parentJSON] = getAnnotationObj(parentKey);
        //console.log(parentJSON);
        if (!parentAnnotation) {
          return [null, null];
        }
        // Arrays just have one children object that all of them use, so
        // short-circuit to that.
        if (parentAnnotation._is_array) {
          return [parentAnnotation.children, parentJSON[parseInt(thisKey)]];
        }
        if (parentAnnotation.children) {
          // If there's a children object, go look in there.
          return w(parentAnnotation.children, thisKey, parentJSON);
        }
        return [null, null];
      } else {
        // possibleParent might be an array
        parentKey = $$('.jsoneditor-readonly', possibleParentValue);
        if (!parentKey) {
          console.error('parent is neither object, nor array (?)');
          return [null, null];
        }
        let [parentAnnotation, parentJSON] = getAnnotationObj(parentKey);
        //console.log(parentJSON);
        if (!parentAnnotation) {
          return [null, null];
        }
        return w(parentAnnotation, thisKey, parentJSON);
      }
    }
    // Nope, possibleParent was actually a sibling (they have the same indent)
    // try the next one up
    possibleParent = possibleParent.previousSibling;
  }
}

function getAnnotation(node) {
  let [anno, value] = getAnnotationObj(node);
  if (anno && anno._annotation) {
    anno = anno._annotation;
    if (anno instanceof Function) {
      return [anno(value), true];
    }
    return [anno, false];
  }
  return [null, false];
}
