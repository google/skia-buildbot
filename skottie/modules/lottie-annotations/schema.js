// The schema exported by this module (as SCHEMA) has the following structure:
// {
//   key1: Annotator,
//   key2: Annotator,
//   ...
//   keyN: Annotator,
// }
// The above structure is known as a AnnotationMap.

// where each key is a key that could exist in the lottie object
// and Annotator is an Object that looks like:
// {
//    _annotation: String|Function(val) the annotation for this node,
//                 or a function that returns a dynamic annotation based
//                 on the value of the node. For example, this allows
//                 the blend mode or 'bm' key to have an annotation of
//                 'normal' if the associated value is 0, or something else
//                 for a non-zero value.
//    _is_array: [optional] Boolean if this node represents an array.
//    _children: [optional] AnnotationMap of any child keys.
// }
// or Annotator is a Function(val) that returns the above Object based
// on the value of the node. For example, this allows for a different
// set of annotations if a transform property is animated or not.

// This Schema is not complete, but full enough to show off the dynamic
// behavior that is involved in the lottie format.


// A little helper function to create an object with _annotation
// containing the value (either String or function(value) => String)
function leaf(v) {
  return {
    _annotation: v,
  }
}

const LAYERS = {
   '0': 'Precomposed',
   '1': 'Solid',
   '2': 'Image',
   '3': 'Null',
   '4': 'Shape',
   '5': 'Text',
  '13': 'Camera',
}

const BLEND_MODE = {
  '0': 'normal',
  '1': 'multiply',
  '2': 'screen',
  '3': 'overlay',
  '4': 'darken',
  '5': 'lighten',
  '6': 'colorDodge',
  '7': 'colorBurn',
  '8': 'hardLight',
  '9': 'overlay',
  '10': 'difference',
  '11': 'exclusion',
  '12': 'hue',
  '13': 'saturation',
  '14': 'color',
  '15': 'luminosity',
  '16': 'add',
}

const flatColor = {
  'a': leaf('Is animated flag'), // should be 0
  'ix': leaf('Index. Used for expressions'),
  'x': leaf('AE expression that modifies the value'),
  'k': leaf('Components (R, G, B, A)'),
}

const fillShape = {
  'c': {
    '_annotation': 'Fill Color',
    '_children': flatColor,  // TODO(kjlubick): Does this need to be keyframed?
  },
  'hd': leaf('Hidden'),
  'mn': leaf('Match Name. Used for expressions'),
  'nm': leaf('Name. Used for expressions'),
  'o': {
    '_annotation': 'Opacity',
    '_children': {
      'a': leaf('Is animated flag'),
      'ix': leaf('Index. Used for expression'),
      'x': leaf('AE expression that modifies the value'),
      'k': leaf('Opacity Value'),
    },
  },
  'r': leaf('Unknown'),
  'ty': leaf('Shape Type - fill'),
}

const mergeShape = {
  'hd': leaf('Hidden'),
  'mm': leaf('Unknown'),
  'mn': leaf('Match Name. Used for expressions'),
  'nm': leaf('Name. Used for expressions'),
  'ty': leaf('Shape Type - merge'),
}

const genericShape = {
  'ind': leaf('Unknown'),
  'hd': leaf('Hidden'),
  'd': leaf('Direction of how the shape is drawn'),
  'ix': leaf('Index. Used for expressions'),
  'mn': leaf('Match Name. Used for expressions'),
  'nm': leaf('Name. Used for expressions'),
  'np': leaf('Group number of properties. Used for expressions'),
  'ty': leaf('Shape Type - Shape Content'),

  'ks': {
    '_annotation': 'Shape Vertices',
    // TODO(kjlubick): flesh this out more
  },
}

const shapeGroup = {
  'cix': leaf('Unknown'),
  'hd': leaf('Hidden'),
  'ix': leaf('Index. Used for expressions'),
  'mn': leaf('Match Name. Used for expressions'),
  'nm': leaf('Name. Used for expressions'),
  'np': leaf('Group number of properties. Used for expressions'),
  'ty': leaf('Shape Type - Shape Group'),

  'it': {
    '_annotation': 'List of items in group',
    '_is_array': true,
    '_children': shapeAnnotator,
  }
}

const transformShape = {
  'nm': leaf('Name. Used for expressions'),
  'ty': leaf('Shape Type - Transform'),
  // TODO(kjlubick)
}

function shapeAnnotator(val) {
  switch(val.ty) {
    case 'fl':
      return fillShape;
    case 'gr':
      return shapeGroup;
    case 'mm':
      return mergeShape;
    case 'sh':
      return genericShape;
    case 'tr':
      return transformShape;
    default:
      return leaf('UNKNOWN shape type');
  }
}

const flatCoordinate = {
  'a': leaf('Is animated flag'), // should be 0
  'ix': leaf('Index. Used for expressions'),
  'x': leaf('AE expression that modifies the value'),
  'k': leaf('Coordinates (X, Y, Z)'),
}

const animatedCoordinate = {
  'a': leaf('Is animated flag'), // should be 1
  'ix': leaf('Index. Used for expressions'),
  'x': leaf('AE expression that modifies the value'),
  'k': {
    _annotation: 'Keyframes',
    '_is_array': true,
    '_children': {
      'e': leaf('End value of keyframe segment'),
      'i': leaf('Bezier curve interpolation in value'),
      'n': leaf('Bezier curve name. Used for caching'),
      'o': leaf('Bezier curve interpolation out value'),
      's': leaf('Start value of keyframe segment'),
      't': leaf('start time of keyframe'),
    }
  }
}

function coordinateAnnotator(val) {
  if (val.a == '1') {
    return animatedCoordinate;
  }
  return flatCoordinate;
}

const transformAnnotator = {
  'a': {
    '_annotation': 'Anchor Point',
    '_children': coordinateAnnotator, // or keyframed version
  },
  'p': {
    '_annotation': 'Position',
    '_children': coordinateAnnotator, // or keyframed version
  },
  'px': leaf('Position X'), // Likely used instead of 'p'
  'py': leaf('Position Y'),
  'o': {
    '_annotation': 'Opacity',
    '_children': {
      'a': leaf('Is animated flag'),
      'ix': leaf('Index. Used for expression'),
      'x': leaf('AE expression that modifies the value'),
      'k': leaf('Opacity Value'),
    },
  },
  'r': {
    '_annotation': 'Rotation',
    '_children': coordinateAnnotator,
  },
  's': {
    '_annotation': 'Scale',
    '_children': coordinateAnnotator,
  },
}

// TODO(kjlubick): break this apart to allow for nested objects
// Most of this data comes from https://github.com/airbnb/lottie-web/blob/699eb219f32a80718e8dc57ba18848fe2250d165/docs/json/animation.json
// or was other-wise reverse-engineered.
export const SCHEMA = {
  'ddd': leaf('3d layer flag'),
  'fr': leaf('Frame Rate (hz)'),
  'h': leaf('Animation Height'),
  'ip': leaf('In Point of the Time Ruler. Sets the initial Frame of the animation'),
  'nm': leaf('Animation Name'),
  'op': leaf('Out Point of the Time Ruler. Sets the final Frame of the animation'),
  'v': leaf('Version of extension that created this animation'), // TODO(kjlubick): Do we need different schemas for different versions?
  'w': leaf('Animation Width'),

  'assets': {
    '_annotation': 'source items that can be used in multiple places. E.g. Images'
  },
  'chars': {
    '_annotation': 'source characters for text layers'
  },
  'layers': {
    '_annotation': 'List of Composition Layers',
    '_is_array': true,
    '_children': {
      'ao': leaf('Auto-Orient along path flag'),
      'ip': leaf('In Point of layer. Sets the initial frame of the layer'),
      'op': leaf('Out Point of layer. Sets the final frame of the layer'),
      'st': leaf('Start Time of layer. Sets the start time of the layer'),
      'sr': leaf('Layer Time Stretching (multiplier)'),
      'ddd': leaf('3d layer flag'),
      'ind': leaf('Layer index in AE. Used for parenting and expressions'),
      'nm': leaf('Layer Name. Used for expressions'),

      'bm': leaf((text) => {
        let s = BLEND_MODE[text] || 'UNKNOWN/INVALID';
        return `Blend Mode: ${s}`
      }),
      'ty': leaf((text) => {
        let s = LAYERS[text] || 'UNKNOWN/INVALID';
        return `Type of Layer: ${s}`
      }),

      'ks': {
        '_annotation': 'Transform properties',
        '_children': transformAnnotator,
      },

      'shapes': {
        '_annotation': 'List of Shapes',
        '_is_array': true,
        '_children': shapeAnnotator,
      }
    }
  }
}
