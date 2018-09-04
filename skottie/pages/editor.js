import './editor.css'
import { $, $$ } from 'common-sk/modules/dom'

const container = document.getElementById('jsoneditor');

let initialRender = false;

const options = {
  sortObjectKeys: true,
  onChange: () => {
    if (!initialRender) {
      return;
    }
    console.log('user edited things');
    $('.dynamic_annotation', container).forEach((e) => {
      e.remove();
    });
    INPUT_JSON = editor.get();
    reannotate();
  }
};

const editor = new JSONEditor(container, options);

// set json
// This is the gear animation
let INPUT_JSON = {'v':'5.1.20','fr':60,'ip':0,'op':300,'w':128,'h':128,'nm':'Comp 1','ddd':0,'assets':[],'layers':[{'ddd':0,'ind':1,'ty':4,'nm':'GearHollowAltWeb Outlines','sr':1,'ks':{'o':{'a':0,'k':100,'ix':11},'r':{'a':1,'k':[{'i':{'x':[0.833],'y':[0.833]},'o':{'x':[0.167],'y':[0.167]},'n':['0p833_0p833_0p167_0p167'],'t':0,'s':[0],'e':[360]},{'t':299}],'ix':10},'p':{'a':0,'k':[64,64,0],'ix':2},'a':{'a':0,'k':[320,320,0],'ix':1},'s':{'a':0,'k':[11.562,11.562,100],'ix':6}},'ao':0,'shapes':[{'ty':'gr','it':[{'ind':0,'ty':'sh','ix':1,'ks':{'a':0,'k':{'i':[[8.101,-15.5],[10.699,0],[0,-52.9],[-3.5,-10.1],[16.699,-5.2],[36.1,0],[16.4,-31.2],[15.5,8.1],[0,10.699],[52.9,0],[10.1,-3.5],[5.2,16.699],[0,36.099],[31.1,16.4],[-8.1,15.5],[-10.7,0],[0,52.9],[3.5,10.1],[-16.7,5.2],[-36.1,0],[-16.4,31.2],[-15.5,-8.1],[0,-10.7],[-52.9,0],[-10.101,3.5],[-5.1,-16.6],[0,-36.1],[-31.1,-16.4]],'o':[[-10,-3.5],[-52.901,0],[0,10.699],[-15.5,8.1],[-16.301,-31.2],[-36,0],[-16.7,-5.101],[3.5,-10],[0,-52.9],[-10.7,0],[-8.1,-15.5],[31.2,-16.301],[0,-36.1],[5.1,-16.7],[10,3.5],[52.9,0],[0,-10.7],[15.5,-8.1],[16.3,31.2],[36.1,0],[16.6,5.1],[-3.5,10],[0,52.9],[10.7,0],[8.099,15.5],[-31.2,16.4],[0,36],[-5.1,16.7]],'v':[[255.3,133.25],[223.901,127.95],[127.901,223.95],[133.2,255.35],[84.901,275.25],[0,223.95],[-84.9,275.25],[-133.3,255.35],[-128,223.95],[-224,127.95],[-255.4,133.25],[-275.3,84.95],[-224,0.05],[-275.2,-84.95],[-255.3,-133.25],[-223.9,-127.95],[-127.9,-223.95],[-133.2,-255.35],[-84.9,-275.25],[0,-223.95],[85,-275.25],[133.3,-255.35],[128,-223.95],[224,-127.95],[255.401,-133.25],[275.3,-84.95],[224,0.05],[275.2,84.85]],'c':true},'ix':2},'nm':'Path 1','mn':'ADBE Vector Shape - Group','hd':false},{'ind':1,'ty':'sh','ix':2,'ks':{'a':0,'k':{'i':[[7.9,2.701],[0,27.5],[-26.1,8.8],[2.1,8.1],[15.1,25.8],[5.5,0],[2.4,-1.4],[10.7,0],[0,35.3],[-5.1,9.3],[7.399,4.4],[28.8,7.4],[1.3,0],[2.201,-6.7],[27.5,0],[8.7,26.1],[6.7,0],[1.4,-0.3],[25.8,-15.1],[-4.2,-7.6],[0,-10.7],[35.3,0],[9.3,5.1],[2.6,0],[3,-5],[7.4,-28.8],[-7.9,-2.6],[0,-27.5],[26.1,-8.699],[-2.1,-8.101],[-15.1,-25.8],[-5.5,0],[-2.4,1.399],[-10.7,0],[0,-35.301],[5.1,-9.3],[-7.4,-4.401],[-28.8,-7.4],[-1.3,0],[-2.3,6.599],[-27.5,0],[-8.699,-26.1],[-6.7,0],[-1.4,0.3],[-25.8,15.099],[4.201,7.599],[0,10.699],[-35.301,0],[-9.3,-5.101],[-2.6,0],[-3,5],[-7.401,28.8]],'o':[[-26.1,-8.799],[0,-27.5],[7.9,-2.7],[-7.4,-28.7],[-3,-5.1],[-2.6,0],[-9.3,5.1],[-35.3,0],[0,-10.7],[4.101,-7.6],[-25.701,-15.1],[-1.3,-0.3],[-6.7,0],[-8.799,26.1],[-27.5,0],[-2.3,-6.6],[-1.3,0],[-28.7,7.4],[-7.5,4.4],[5.1,9.3],[0,35.3],[-10.7,0],[-2.4,-1.3],[-5.5,0],[-15.1,25.7],[-2.1,8.1],[26,8.8],[0,27.5],[-7.9,2.701],[7.4,28.7],[3,5.1],[2.6,0],[9.3,-5.101],[35.3,0],[0,10.699],[-4.1,7.599],[25.7,15.099],[1.3,0.3],[6.7,0],[8.8,-26.1],[27.5,0],[2.301,6.599],[1.3,0],[28.701,-7.4],[7.5,-4.401],[-5.1,-9.3],[0,-35.301],[10.699,0],[2.401,1.3],[5.5,0],[15.1,-25.701],[2.099,-8]],'v':[[299.6,60.649],[256,0.05],[299.6,-60.65],[310,-79.75],[276.1,-161.85],[262.3,-169.75],[254.6,-167.75],[224,-159.95],[160,-223.95],[167.8,-254.55],[161.901,-276.05],[79.8,-309.95],[75.8,-310.45],[60.7,-299.55],[0,-255.95],[-60.6,-299.55],[-75.7,-310.45],[-79.7,-309.95],[-161.8,-276.05],[-167.7,-254.55],[-159.9,-223.95],[-223.9,-159.95],[-254.5,-167.75],[-262.2,-169.65],[-276,-161.85],[-309.9,-79.75],[-299.5,-60.65],[-256,0.05],[-299.6,60.649],[-310,79.75],[-276.1,161.85],[-262.3,169.75],[-254.6,167.75],[-224,159.95],[-160,223.95],[-167.8,254.55],[-161.9,276.05],[-79.8,309.95],[-75.8,310.45],[-60.6,299.55],[0,255.95],[60.6,299.55],[75.7,310.45],[79.7,309.95],[161.8,276.05],[167.7,254.55],[159.901,223.95],[223.901,159.95],[254.5,167.75],[262.2,169.649],[276,161.85],[309.901,79.75]],'c':true},'ix':2},'nm':'Path 2','mn':'ADBE Vector Shape - Group','hd':false},{'ty':'mm','mm':1,'nm':'Merge Paths 1','mn':'ADBE Vector Filter - Merge','hd':false},{'ty':'fl','c':{'a':0,'k':[0,0,0,1],'ix':4},'o':{'a':0,'k':100,'ix':5},'r':1,'nm':'Fill 1','mn':'ADBE Vector Graphic - Fill','hd':false},{'ty':'tr','p':{'a':0,'k':[320,319.95],'ix':2},'a':{'a':0,'k':[0,0],'ix':1},'s':{'a':0,'k':[100,100],'ix':3},'r':{'a':0,'k':0,'ix':6},'o':{'a':0,'k':100,'ix':7},'sk':{'a':0,'k':0,'ix':4},'sa':{'a':0,'k':0,'ix':5},'nm':'Transform'}],'nm':'Group 1','np':4,'cix':2,'ix':1,'mn':'ADBE Vector Group','hd':false},{'ty':'gr','it':[{'ind':0,'ty':'sh','ix':1,'ks':{'a':0,'k':{'i':[[52.9,0],[0,52.9],[-52.9,0],[0,-52.9]],'o':[[-52.9,0],[0,-52.9],[52.9,0],[0,52.9]],'v':[[0,96],[-96,0],[0,-96],[96,0]],'c':true},'ix':2},'nm':'Path 1','mn':'ADBE Vector Shape - Group','hd':false},{'ind':1,'ty':'sh','ix':2,'ks':{'a':0,'k':{'i':[[70.6,0],[0,-70.6],[-70.6,0],[0,70.6]],'o':[[-70.6,0],[0,70.6],[70.6,0],[0,-70.6]],'v':[[0,-128],[-128,0],[0,128],[128,0]],'c':true},'ix':2},'nm':'Path 2','mn':'ADBE Vector Shape - Group','hd':false},{'ty':'mm','mm':1,'nm':'Merge Paths 1','mn':'ADBE Vector Filter - Merge','hd':false},{'ty':'fl','c':{'a':0,'k':[0,0,0,1],'ix':4},'o':{'a':0,'k':100,'ix':5},'r':1,'nm':'Fill 1','mn':'ADBE Vector Graphic - Fill','hd':false},{'ty':'tr','p':{'a':0,'k':[320,320],'ix':2},'a':{'a':0,'k':[0,0],'ix':1},'s':{'a':0,'k':[100,100],'ix':3},'r':{'a':0,'k':0,'ix':6},'o':{'a':0,'k':100,'ix':7},'sk':{'a':0,'k':0,'ix':4},'sa':{'a':0,'k':0,'ix':5},'nm':'Transform'}],'nm':'Group 2','np':4,'cix':2,'ix':2,'mn':'ADBE Vector Group','hd':false}],'ip':0,'op':300,'st':0,'bm':0}],'markers':[]};
editor.set(INPUT_JSON);
editor.expandAll(); //FIXME remove

// A little helper function to create an object with _annotation
// containing the value (either String or function(value) => String)
function leaf(v) {
  return {
    _annotation: v,
  }
}

const SHAPES = {
  '0': 'Precomputed',
  '1': 'Solid',
  '2': 'Image',
  '3': 'Null',
  '4': 'Shape',
  '5': 'Text',
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
}

const shapeAnnotator = {

}

const flatCoordinate = {
  'a': leaf('Is animated flag'), // should be 0
  'ix': leaf('Index. Used for expression'),
  'x': leaf('AE expression that modifies the value'),
  'k': leaf('Coordinates (X, Y, Z)'),
}

const animatedCoordinate = {
  'a': leaf('Is animated flag'), // should be 0
  'ix': leaf('Index. Used for expression'),
  'x': leaf('AE expression that modifies the value'),
  'k': leaf('Keyframes'),
}

function coordinateAnnotator(val) {
  if (val.a == '1') {
    return animatedCoordinate;
  }
  return flatCoordinate
}

const transformAnnotator = {
  'a': {
    '_annotation': 'Anchor Point',
    'children': coordinateAnnotator, // or keyframed version
  },
  'p': {
    '_annotation': 'Position',
    'children': coordinateAnnotator, // or keyframed version
  },
  'px': leaf('Position X'), // Likely used instead of 'p'
  'py': leaf('Position Y'),
  'o': {
    '_annotation': 'Opacity',
    'children': {
      'a': leaf('Is animated flag'),
      'ix': leaf('Index. Used for expression'),
      'x': leaf('AE expression that modifies the value'),
      'k': leaf('Opacity Value'),
    },
  },
  'r': {
    '_annotation': 'Rotation',
    'children': coordinateAnnotator,
  },
  's': {
    '_annotation': 'Scale',
    'children': coordinateAnnotator,
  },
}

// TODO(kjlubick): break this apart to allow for nested objects
// Most of this data comes from https://github.com/airbnb/lottie-web/blob/699eb219f32a80718e8dc57ba18848fe2250d165/docs/json/animation.json
// or was other-wise reverse-engineered.
let annotations = {
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
    'children': {
      'ao': leaf('Auto-Orient along path flag'),
      'ip': leaf('In Point of layer. Sets the initial frame of the layer'),
      'op': leaf('Out Point of layer. Sets the final frame of the layer'),
      'st': leaf('Start Time of layer. Sets the start time of the layer'),
      'sr': leaf('Layer Time Stretching (multiplier)'),
      'ddd': leaf('3d layer flag'),
      'ind': leaf('Layer index in AE. Used for parenting and expressions'),
      'nm': leaf('Layer Name. Used for expressions'),

      'bm':leaf((text) => {
        let s = BLEND_MODE[text] || 'UNKNOWN/INVALID';
        return `Blend Mode: ${s}`
      }),
      'ty': leaf((text) => {
        let s = SHAPES[text] || 'UNKNOWN/INVALID';
        return `Type of Layer: ${s}`
      }),

      'ks': {
        '_annotation': 'Transform properties',
        'children': transformAnnotator,
      },
      'shapes': {
        '_annotation': 'List of Shapes',
        '_is_array': true,
        'children': shapeAnnotator,
      }
    }
  }
}

// Get the annotation object and the JSON for a given node. This will use recursion
// to find a path to the parent (which uses the annotations object)
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
      return w(annotations, thisKey, INPUT_JSON);
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
window.getAnnotationObj = getAnnotationObj; // FIXME for debugging

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

function reannotate() {
  console.time('reannotate');
  let rows = $('.jsoneditor-values');
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

container.addEventListener('click', () => {
  // There's no good event for "thing was expanded", so
  // we just wait for any clicks and check again.
  reannotate();
});
reannotate();
initialRender = true;