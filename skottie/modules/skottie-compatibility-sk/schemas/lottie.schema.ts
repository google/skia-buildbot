// Copied from https://lottie.github.io/lottie-spec/specs/schema/

// prettier-ignore
export const schema = {
    "$schema": "https://json-schema.org/draft/2020-12/schema",
    "$id": "https://lottie.github.io/lottie-spec/specs/schema/",
    "$ref": "#/$defs/composition/animation",
    "$defs": {
        "assets": {
            "image": {
                "type": "object",
                "title": "Image",
                "description": "Asset containing an image that can be referenced by layers.",
                "allOf": [
                    {
                        "$ref": "#/$defs/assets/asset"
                    },
                    {
                        "type": "object",
                        "properties": {
                            "w": {
                                "title": "Width",
                                "description": "Width of the image",
                                "type": "number"
                            },
                            "h": {
                                "title": "Height",
                                "description": "Height of the image",
                                "type": "number"
                            },
                            "p": {
                                "title": "File Name",
                                "description": "Name of the image file or a data url",
                                "oneOf": [
                                    {
                                        "type": "string"
                                    },
                                    {
                                        "$ref": "#/$defs/values/data-url"
                                    }
                                ]
                            },
                            "u": {
                                "title": "File Path",
                                "description": "Path to the image file",
                                "type": "string"
                            },
                            "e": {
                                "title": "Embedded",
                                "description": "If '1', 'p' is a Data URL",
                                "$ref": "#/$defs/values/int-boolean"
                            }
                        },
                        "required": [
                            "w",
                            "h",
                            "p"
                        ]
                    }
                ]
            },
            "all-assets": {
                "oneOf": [
                    {
                        "$ref": "#/$defs/assets/precomposition"
                    },
                    {
                        "$ref": "#/$defs/assets/image"
                    }
                ]
            },
            "asset": {
                "type": "object",
                "title": "Asset",
                "allOf": [
                    {
                        "type": "object",
                        "properties": {
                            "id": {
                                "title": "ID",
                                "description": "Unique identifier used by layers when referencing this asset",
                                "type": "string",
                                "default": ""
                            },
                            "nm": {
                                "title": "Name",
                                "description": "Human readable name",
                                "type": "string"
                            }
                        },
                        "required": [
                            "id"
                        ]
                    }
                ]
            },
            "precomposition": {
                "type": "object",
                "title": "Precomposition",
                "description": "Asset containing a composition that can be referenced by layers.",
                "allOf": [
                    {
                        "$ref": "#/$defs/assets/asset"
                    },
                    {
                        "$ref": "#/$defs/composition/composition"
                    }
                ]
            }
        },
        "composition": {
            "composition": {
                "type": "object",
                "title": "Composition",
                "description": "An object that contains a list of layers",
                "properties": {
                    "layers": {
                        "title": "Layers",
                        "type": "array",
                        "items": {
                            "$ref": "#/$defs/layers/all-layers"
                        }
                    }
                },
                "required": [
                    "layers"
                ]
            },
            "animation": {
                "type": "object",
                "title": "Animation",
                "description": "Top level object, describing the animation",
                "allOf": [
                    {
                        "$ref": "#/$defs/helpers/visual-object"
                    },
                    {
                        "type": "object",
                        "properties": {
                            "fr": {
                                "title": "Framerate",
                                "description": "Framerate in frames per second",
                                "type": "number"
                            },
                            "ip": {
                                "title": "In Point",
                                "description": "Frame the animation starts at (usually 0)",
                                "type": "number"
                            },
                            "op": {
                                "title": "Out Point",
                                "description": "Frame the animation stops/loops at, which makes this the duration in frames when `ip` is 0",
                                "type": "number"
                            },
                            "w": {
                                "title": "Width",
                                "description": "Width of the animation",
                                "type": "integer"
                            },
                            "h": {
                                "title": "Height",
                                "description": "Height of the animation",
                                "type": "integer"
                            },
                            "assets": {
                                "title": "Assets",
                                "type": "array",
                                "description": "List of assets that can be referenced by layers",
                                "items": {
                                    "$ref": "#/$defs/assets/all-assets"
                                }
                            },
                            "markers": {
                                "title": "Markers",
                                "description": "Markers defining named sections of the composition.",
                                "type": "array",
                                "items": {
                                    "$ref": "#/$defs/helpers/marker"
                                }
                            }
                        },
                        "required": [
                            "w",
                            "h",
                            "fr",
                            "op",
                            "ip"
                        ]
                    },
                    {
                        "$ref": "#/$defs/composition/composition"
                    }
                ]
            }
        },
        "constants": {
            "shape-direction": {
                "type": "integer",
                "title": "Shape Direction",
                "description": "Drawing direction of the shape curve, useful for trim path",
                "oneOf": [
                    {
                        "title": "Normal",
                        "description": "Usually clockwise",
                        "const": 1
                    },
                    {
                        "title": "Reversed",
                        "description": "Usually counter clockwise",
                        "const": 3
                    }
                ]
            },
            "star-type": {
                "type": "integer",
                "title": "Star Type",
                "description": "Whether a PolyStar is a star or a polygon",
                "oneOf": [
                    {
                        "title": "Star",
                        "const": 1
                    },
                    {
                        "title": "Polygon",
                        "const": 2
                    }
                ]
            },
            "fill-rule": {
                "type": "integer",
                "title": "Fill Rule",
                "description": "Rule used to handle multiple shapes rendered with the same fill object",
                "oneOf": [
                    {
                        "title": "Non Zero",
                        "description": "Everything is colored (You can think of this as an OR)",
                        "const": 1
                    },
                    {
                        "title": "Even Odd",
                        "description": "Colored based on intersections and path direction, can be used to create \"holes\"",
                        "const": 2
                    }
                ]
            },
            "trim-multiple-shapes": {
                "type": "integer",
                "title": "Trim Multiple Shapes",
                "description": "How to handle multiple shapes in trim path",
                "oneOf": [
                    {
                        "title": "Parallel",
                        "description": "All shapes apply the trim at the same time",
                        "const": 1
                    },
                    {
                        "title": "Sequential",
                        "description": "Shapes are considered as a continuous sequence",
                        "const": 2
                    }
                ]
            }
        },
        "helpers": {
            "transform": {
                "type": "object",
                "title": "Transform",
                "description": "Layer transform",
                "allOf": [
                    {
                        "properties": {
                            "a": {
                                "title": "Anchor Point",
                                "description": "Anchor point: a position (relative to its parent) around which transformations are applied (ie: center for rotation / scale)",
                                "$ref": "#/$defs/properties/position-property"
                            },
                            "p": {
                                "title": "Position",
                                "description": "Position / Translation",
                                "$ref": "#/$defs/properties/position-property"
                            },
                            "r": {
                                "title": "Rotation",
                                "description": "Rotation in degrees, clockwise",
                                "$ref": "#/$defs/properties/scalar-property"
                            },
                            "s": {
                                "title": "Scale",
                                "description": "Scale factor, `[100, 100]` for no scaling",
                                "$ref": "#/$defs/properties/vector-property"
                            },
                            "o": {
                                "title": "Opacity",
                                "$ref": "#/$defs/properties/scalar-property"
                            },
                            "sk": {
                                "title": "Skew",
                                "description": "Skew amount as an angle in degrees",
                                "$ref": "#/$defs/properties/scalar-property"
                            },
                            "sa": {
                                "title": "Skew Axis",
                                "description": "Direction along which skew is applied, in degrees (`0` skews along the X axis, `90` along the Y axis)",
                                "$ref": "#/$defs/properties/scalar-property"
                            }
                        }
                    }
                ]
            },
            "visual-object": {
                "type": "object",
                "title": "Visual Object",
                "description": "",
                "allOf": [
                    {
                        "type": "object",
                        "properties": {
                            "nm": {
                                "title": "Name",
                                "description": "Name, as seen from editors and the like",
                                "type": "string"
                            }
                        },
                        "required": []
                    }
                ]
            },
            "marker": {
                "type": "object",
                "title": "Marker",
                "description": "Defines named portions of the composition.",
                "properties": {
                    "cm": {
                        "title": "Comment",
                        "type": "string"
                    },
                    "tm": {
                        "title": "Time",
                        "type": "number"
                    },
                    "dr": {
                        "title": "Duration",
                        "type": "number"
                    }
                }
            }
        },
        "layers": {
            "visual-layer": {
                "type": "object",
                "title": "Visual Layer",
                "description": "Layer used to affect visual elements",
                "allOf": [
                    {
                        "$ref": "#/$defs/layers/layer"
                    },
                    {
                        "type": "object",
                        "properties": {
                            "ks": {
                                "title": "Transform",
                                "description": "Layer transform",
                                "$ref": "#/$defs/helpers/transform"
                            },
                            "ao": {
                                "title": "Auto Orient",
                                "$ref": "#/$defs/values/int-boolean",
                                "default": 0,
                                "description": "If 1, The layer will rotate itself to match its animated position path"
                            }
                        },
                        "required": [
                            "ks"
                        ]
                    }
                ]
            },
            "shape-layer": {
                "type": "object",
                "title": "Shape Layer",
                "description": "Layer containing Shapes",
                "allOf": [
                    {
                        "$ref": "#/$defs/layers/visual-layer"
                    },
                    {
                        "type": "object",
                        "properties": {
                            "ty": {
                                "title": "Type",
                                "description": "Layer type",
                                "type": "integer",
                                "const": 4
                            },
                            "shapes": {
                                "title": "Shapes",
                                "type": "array",
                                "items": {
                                    "$ref": "#/$defs/shapes/all-graphic-elements"
                                }
                            }
                        },
                        "required": [
                            "ty",
                            "shapes"
                        ]
                    }
                ]
            },
            "layer": {
                "type": "object",
                "title": "Layer",
                "description": "Common properties for all layers",
                "allOf": [
                    {
                        "$ref": "#/$defs/helpers/visual-object"
                    },
                    {
                        "type": "object",
                        "properties": {
                            "hd": {
                                "title": "Hidden",
                                "description": "Whether the layer is hidden",
                                "type": "boolean"
                            },
                            "ty": {
                                "title": "Type",
                                "description": "Layer Type",
                                "type": "integer"
                            },
                            "ind": {
                                "title": "Index",
                                "type": "integer",
                                "description": "Index that can be used for parenting and referenced in expressions"
                            },
                            "parent": {
                                "title": "Parent Index",
                                "description": "Must be the `ind` property of another layer",
                                "type": "integer"
                            },
                            "sr": {
                                "title": "Time Stretch",
                                "type": "number",
                                "default": 1
                            },
                            "ip": {
                                "title": "In Point",
                                "description": "Frame when the layer becomes visible",
                                "type": "number"
                            },
                            "op": {
                                "title": "Out Point",
                                "description": "Frame when the layer becomes invisible",
                                "type": "number"
                            },
                            "st": {
                                "title": "Start Time",
                                "type": "number",
                                "default": 0
                            }
                        },
                        "required": [
                            "ty",
                            "st",
                            "ip",
                            "op"
                        ]
                    }
                ]
            },
            "null-layer": {
                "type": "object",
                "title": "Null Layer",
                "description": "Layer with no data, useful to group layers together",
                "allOf": [
                    {
                        "$ref": "#/$defs/layers/visual-layer"
                    },
                    {
                        "type": "object",
                        "properties": {
                            "ty": {
                                "title": "Type",
                                "description": "Layer type",
                                "type": "integer",
                                "const": 3
                            }
                        },
                        "required": [
                            "ty"
                        ]
                    }
                ]
            },
            "solid-layer": {
                "type": "object",
                "title": "Solid Layer",
                "description": "Solid color, rectangle-shaped layer",
                "allOf": [
                    {
                        "$ref": "#/$defs/layers/visual-layer"
                    },
                    {
                        "type": "object",
                        "properties": {
                            "ty": {
                                "title": "Type",
                                "description": "Layer type",
                                "type": "integer",
                                "const": 1
                            },
                            "sw": {
                                "title": "Width",
                                "description": "Solid rectangle width",
                                "type": "integer"
                            },
                            "sh": {
                                "title": "Height",
                                "description": "Solid rectangle height",
                                "type": "integer"
                            },
                            "sc": {
                                "title": "Color",
                                "description": "Solid fill color",
                                "$ref": "#/$defs/values/hexcolor"
                            }
                        },
                        "required": [
                            "ty",
                            "sw",
                            "sh",
                            "sc"
                        ]
                    }
                ]
            },
            "all-layers": {
                "oneOf": [
                    {
                        "$ref": "#/$defs/layers/precomposition-layer"
                    },
                    {
                        "$ref": "#/$defs/layers/image-layer"
                    },
                    {
                        "$ref": "#/$defs/layers/null-layer"
                    },
                    {
                        "$ref": "#/$defs/layers/solid-layer"
                    },
                    {
                        "$ref": "#/$defs/layers/shape-layer"
                    }
                ]
            },
            "image-layer": {
                "type": "object",
                "title": "Image Layer",
                "description": "Layer containing an image",
                "allOf": [
                    {
                        "$ref": "#/$defs/layers/visual-layer"
                    },
                    {
                        "type": "object",
                        "properties": {
                            "ty": {
                                "title": "Type",
                                "description": "Layer type",
                                "type": "integer",
                                "const": 2
                            },
                            "refId": {
                                "title": "Reference Id",
                                "description": "ID of the image as specified in the assets",
                                "type": "string"
                            }
                        },
                        "required": [
                            "ty",
                            "refId"
                        ]
                    }
                ]
            },
            "precomposition-layer": {
                "type": "object",
                "title": "Precomposition Layer",
                "description": "Layer that renders a Precomposition asset",
                "allOf": [
                    {
                        "$ref": "#/$defs/layers/visual-layer"
                    },
                    {
                        "type": "object",
                        "properties": {
                            "ty": {
                                "title": "Type",
                                "description": "Layer type",
                                "type": "integer",
                                "const": 0
                            },
                            "refId": {
                                "title": "Reference Id",
                                "description": "ID of the precomp as specified in the assets",
                                "type": "string"
                            },
                            "w": {
                                "title": "Width",
                                "description": "Width of the clipping rect",
                                "type": "integer"
                            },
                            "h": {
                                "title": "Height",
                                "description": "Height of the clipping rect",
                                "type": "integer"
                            }
                        },
                        "required": [
                            "ty",
                            "refId"
                        ]
                    }
                ]
            }
        },
        "properties": {
            "color-property": {
                "type": "object",
                "title": "Color Property",
                "description": "An animatable property that holds a Color",
                "oneOf": [
                    {
                        "$comment": "Not animated",
                        "properties": {
                            "a": {
                                "title": "Animated",
                                "description": "Whether the property is animated",
                                "$ref": "#/$defs/values/int-boolean",
                                "const": 0
                            },
                            "k": {
                                "title": "Static value",
                                "$ref": "#/$defs/values/color"
                            }
                        }
                    },
                    {
                        "$comment": "Animated",
                        "properties": {
                            "a": {
                                "title": "Animated",
                                "description": "Whether the property is animated",
                                "$ref": "#/$defs/values/int-boolean",
                                "const": 1
                            },
                            "k": {
                                "type": "array",
                                "title": "Keyframes",
                                "description": "Array of keyframes",
                                "items": {
                                    "$ref": "#/$defs/properties/color-keyframe"
                                }
                            }
                        }
                    }
                ],
                "required": [
                    "a",
                    "k"
                ]
            },
            "scalar-property": {
                "type": "object",
                "title": "Scalar Property",
                "description": "An animatable property that holds a float",
                "oneOf": [
                    {
                        "$comment": "Not animated",
                        "properties": {
                            "a": {
                                "title": "Animated",
                                "description": "Whether the property is animated",
                                "$ref": "#/$defs/values/int-boolean",
                                "const": 0
                            },
                            "k": {
                                "title": "Static value",
                                "type": "number"
                            }
                        }
                    },
                    {
                        "$comment": "Animated",
                        "properties": {
                            "a": {
                                "title": "Animated",
                                "description": "Whether the property is animated",
                                "$ref": "#/$defs/values/int-boolean",
                                "const": 1
                            },
                            "k": {
                                "type": "array",
                                "title": "Keyframes",
                                "description": "Array of keyframes",
                                "items": {
                                    "$ref": "#/$defs/properties/vector-keyframe"
                                }
                            }
                        }
                    }
                ],
                "required": [
                    "a",
                    "k"
                ]
            },
            "color-keyframe": {
                "type": "object",
                "title": "Color Keyframe",
                "allOf": [
                    {
                        "$ref": "#/$defs/properties/base-keyframe"
                    },
                    {
                        "properties": {
                            "s": {
                                "title": "Value",
                                "description": "Value at this keyframe.",
                                "$ref": "#/$defs/values/color"
                            }
                        }
                    }
                ],
                "required": [
                    "s"
                ]
            },
            "easing-handle": {
                "type": "object",
                "title": "Keyframe Easing",
                "description": "Bezier handle for keyframe interpolation",
                "properties": {
                    "x": {
                        "title": "X",
                        "description": "Time component:\n0 means start time of the keyframe,\n1 means time of the next keyframe.",
                        "oneOf": [
                            {
                                "type": "array",
                                "$ref": "#/$defs/values/vector",
                                "items": {
                                    "type": "number",
                                    "default": 0,
                                    "minimum": 0,
                                    "maximum": 1
                                },
                                "minItems": 1
                            },
                            {
                                "type": "number",
                                "default": 0,
                                "minimum": 0,
                                "maximum": 1
                            }
                        ]
                    },
                    "y": {
                        "title": "Y",
                        "description": "Value interpolation component:\n0 means start value of the keyframe,\n1 means value at the next keyframe.",
                        "oneOf": [
                            {
                                "type": "array",
                                "$ref": "#/$defs/values/vector",
                                "items": {
                                    "type": "number",
                                    "default": 0,
                                    "minimum": 0,
                                    "maximum": 1
                                },
                                "minItems": 1
                            },
                            {
                                "type": "number",
                                "default": 0,
                                "minimum": 0,
                                "maximum": 1
                            }
                        ]
                    }
                },
                "required": [
                    "x",
                    "y"
                ]
            },
            "bezier-keyframe": {
                "type": "object",
                "title": "Shape Keyframe",
                "allOf": [
                    {
                        "$ref": "#/$defs/properties/base-keyframe"
                    },
                    {
                        "properties": {
                            "s": {
                                "title": "Value",
                                "description": "Value at this keyframe.",
                                "$ref": "#/$defs/values/bezier"
                            }
                        }
                    }
                ],
                "required": [
                    "s"
                ]
            },
            "vector-property": {
                "type": "object",
                "title": "Vector Property",
                "description": "An animatable property that holds an array of numbers",
                "oneOf": [
                    {
                        "$comment": "Not animated",
                        "properties": {
                            "a": {
                                "title": "Animated",
                                "description": "Whether the property is animated",
                                "$ref": "#/$defs/values/int-boolean",
                                "const": 0
                            },
                            "k": {
                                "title": "Value",
                                "description": "Static Value",
                                "$ref": "#/$defs/values/vector"
                            }
                        }
                    },
                    {
                        "$comment": "Animated",
                        "properties": {
                            "a": {
                                "title": "Animated",
                                "description": "Whether the property is animated",
                                "$ref": "#/$defs/values/int-boolean",
                                "const": 1
                            },
                            "k": {
                                "type": "array",
                                "title": "Keyframes",
                                "description": "Array of keyframes",
                                "items": {
                                    "$ref": "#/$defs/properties/vector-keyframe"
                                }
                            }
                        }
                    }
                ],
                "required": [
                    "a",
                    "k"
                ]
            },
            "base-keyframe": {
                "type": "object",
                "title": "Base Keyframe",
                "description": "A Keyframes specifies the value at a specific time and the interpolation function to reach the next keyframe.",
                "allOf": [
                    {
                        "properties": {
                            "t": {
                                "title": "Time",
                                "description": "Frame number",
                                "type": "number",
                                "default": 0
                            },
                            "h": {
                                "title": "Hold",
                                "$ref": "#/$defs/values/int-boolean",
                                "default": 0
                            },
                            "i": {
                                "title": "In Tangent",
                                "description": "Easing tangent going into the next keyframe",
                                "$ref": "#/$defs/properties/easing-handle"
                            },
                            "o": {
                                "title": "Out Tangent",
                                "description": "Easing tangent leaving the current keyframe",
                                "$ref": "#/$defs/properties/easing-handle"
                            }
                        }
                    },
                    {
                        "if": {
                            "oneOf": [
                                {
                                    "properties": {
                                        "h": {
                                            "const": 0
                                        }
                                    }
                                },
                                {
                                    "not": {
                                        "required": [
                                            "h"
                                        ]
                                    }
                                }
                            ]
                        },
                        "then": {
                            "required": [
                                "i",
                                "o"
                            ]
                        }
                    }
                ],
                "required": [
                    "t"
                ]
            },
            "bezier-property": {
                "type": "object",
                "title": "Bezier Property",
                "description": "An animatable property that holds a Bezier shape",
                "oneOf": [
                    {
                        "$comment": "Not animated",
                        "properties": {
                            "a": {
                                "title": "Animated",
                                "description": "Whether the property is animated",
                                "$ref": "#/$defs/values/int-boolean",
                                "const": 0
                            },
                            "k": {
                                "title": "Static value",
                                "$ref": "#/$defs/values/bezier"
                            }
                        }
                    },
                    {
                        "$comment": "Animated",
                        "properties": {
                            "a": {
                                "title": "Animated",
                                "description": "Whether the property is animated",
                                "$ref": "#/$defs/values/int-boolean",
                                "const": 1
                            },
                            "k": {
                                "type": "array",
                                "title": "Keyframes",
                                "description": "Array of keyframes",
                                "items": {
                                    "$ref": "#/$defs/properties/bezier-keyframe"
                                }
                            }
                        }
                    }
                ],
                "required": [
                    "a",
                    "k"
                ]
            },
            "position-property": {
                "type": "object",
                "title": "Position Property",
                "description": "An animatable property to represent a position in space",
                "oneOf": [
                    {
                        "$comment": "Not animated",
                        "properties": {
                            "a": {
                                "title": "Animated",
                                "description": "Whether the property is animated",
                                "$ref": "#/$defs/values/int-boolean",
                                "const": 0
                            },
                            "k": {
                                "title": "Static value",
                                "$ref": "#/$defs/values/vector"
                            }
                        }
                    },
                    {
                        "$comment": "Animated",
                        "properties": {
                            "a": {
                                "title": "Animated",
                                "description": "Whether the property is animated",
                                "$ref": "#/$defs/values/int-boolean",
                                "const": 1
                            },
                            "k": {
                                "type": "array",
                                "title": "Keyframes",
                                "description": "Array of keyframes",
                                "items": {
                                    "$ref": "#/$defs/properties/position-keyframe"
                                }
                            }
                        }
                    }
                ],
                "required": [
                    "a",
                    "k"
                ]
            },
            "vector-keyframe": {
                "type": "object",
                "title": "Vector Keyframe",
                "allOf": [
                    {
                        "$ref": "#/$defs/properties/base-keyframe"
                    },
                    {
                        "properties": {
                            "s": {
                                "title": "Value",
                                "description": "Value at this keyframe.",
                                "$ref": "#/$defs/values/vector"
                            }
                        }
                    }
                ],
                "required": [
                    "s"
                ]
            },
            "position-keyframe": {
                "type": "object",
                "title": "Position Keyframe",
                "allOf": [
                    {
                        "$ref": "#/$defs/properties/vector-keyframe"
                    },
                    {
                        "properties": {
                            "ti": {
                                "title": "Value In Tangent",
                                "description": "Tangent for values (eg: moving position around a curved path)",
                                "$ref": "#/$defs/values/vector"
                            },
                            "to": {
                                "title": "Value Out Tangent",
                                "description": "Tangent for values (eg: moving position around a curved path)",
                                "$ref": "#/$defs/values/vector"
                            }
                        }
                    }
                ]
            }
        },
        "shapes": {
            "shape-style": {
                "type": "object",
                "title": "Shape Style",
                "description": "Describes the visual appearance (like fill and stroke) of neighbouring shapes",
                "allOf": [
                    {
                        "$ref": "#/$defs/shapes/graphic-element"
                    },
                    {
                        "type": "object",
                        "properties": {
                            "o": {
                                "title": "Opacity",
                                "description": "Opacity, 100 means fully opaque",
                                "$ref": "#/$defs/properties/scalar-property"
                            }
                        },
                        "required": [
                            "o"
                        ]
                    }
                ]
            },
            "polystar": {
                "type": "object",
                "title": "PolyStar",
                "description": "Star or regular polygon",
                "allOf": [
                    {
                        "$ref": "#/$defs/shapes/shape"
                    },
                    {
                        "type": "object",
                        "properties": {
                            "ty": {
                                "title": "Shape Type",
                                "type": "string",
                                "const": "sr"
                            },
                            "p": {
                                "title": "Position",
                                "$ref": "#/$defs/properties/position-property"
                            },
                            "or": {
                                "title": "Outer Radius",
                                "$ref": "#/$defs/properties/scalar-property"
                            },
                            "os": {
                                "title": "Outer Roundness",
                                "description": "Outer Roundness as a percentage",
                                "$ref": "#/$defs/properties/scalar-property"
                            },
                            "r": {
                                "title": "Rotation",
                                "description": "Rotation, clockwise in degrees",
                                "$ref": "#/$defs/properties/scalar-property"
                            },
                            "pt": {
                                "title": "Points",
                                "$ref": "#/$defs/properties/scalar-property"
                            },
                            "sy": {
                                "title": "Star Type",
                                "$ref": "#/$defs/constants/star-type",
                                "default": 1
                            },
                            "ir": {
                                "title": "Inner Radius",
                                "$ref": "#/$defs/properties/scalar-property"
                            },
                            "is": {
                                "title": "Inner Roundness",
                                "description": "Inner Roundness as a percentage",
                                "$ref": "#/$defs/properties/scalar-property"
                            }
                        },
                        "required": [
                            "ty",
                            "or",
                            "os",
                            "pt",
                            "p",
                            "r"
                        ]
                    },
                    {
                        "if": {
                            "properties": {
                                "sy": {
                                    "const": 1
                                }
                            }
                        },
                        "then": {
                            "required": [
                                "ir",
                                "is"
                            ]
                        }
                    }
                ]
            },
            "path": {
                "type": "object",
                "title": "Path",
                "description": "Custom Bezier shape",
                "allOf": [
                    {
                        "$ref": "#/$defs/shapes/shape"
                    },
                    {
                        "type": "object",
                        "properties": {
                            "ty": {
                                "title": "Shape Type",
                                "type": "string",
                                "const": "sh"
                            },
                            "ks": {
                                "title": "Shape",
                                "description": "Bezier path",
                                "$ref": "#/$defs/properties/bezier-property"
                            }
                        },
                        "required": [
                            "ty",
                            "ks"
                        ]
                    }
                ]
            },
            "trim-path": {
                "type": "object",
                "title": "Trim Path",
                "description": "Trims shapes into a segment",
                "allOf": [
                    {
                        "$ref": "#/$defs/shapes/modifier"
                    },
                    {
                        "type": "object",
                        "properties": {
                            "ty": {
                                "title": "Shape Type",
                                "type": "string",
                                "const": "tm"
                            },
                            "s": {
                                "title": "Start",
                                "description": "Segment start",
                                "$ref": "#/$defs/properties/scalar-property"
                            },
                            "e": {
                                "title": "End",
                                "description": "Segment end",
                                "$ref": "#/$defs/properties/scalar-property"
                            },
                            "o": {
                                "title": "Offset",
                                "$ref": "#/$defs/properties/scalar-property"
                            },
                            "m": {
                                "title": "Multiple",
                                "description": "How to treat multiple copies",
                                "$ref": "#/$defs/constants/trim-multiple-shapes"
                            }
                        },
                        "required": [
                            "ty",
                            "o",
                            "s",
                            "e"
                        ]
                    }
                ]
            },
            "rectangle": {
                "type": "object",
                "title": "Rectangle",
                "description": "A simple rectangle shape",
                "allOf": [
                    {
                        "$ref": "#/$defs/shapes/shape"
                    },
                    {
                        "type": "object",
                        "properties": {
                            "ty": {
                                "title": "Shape Type",
                                "type": "string",
                                "const": "rc"
                            },
                            "p": {
                                "title": "Position",
                                "description": "Center of the rectangle",
                                "$ref": "#/$defs/properties/position-property"
                            },
                            "s": {
                                "title": "Size",
                                "$ref": "#/$defs/properties/vector-property"
                            },
                            "r": {
                                "title": "Rounded",
                                "description": "Rounded corners radius",
                                "$ref": "#/$defs/properties/scalar-property"
                            }
                        },
                        "required": [
                            "ty",
                            "s",
                            "p"
                        ]
                    }
                ]
            },
            "shape": {
                "type": "object",
                "title": "Shape",
                "description": "Drawable shape, defines the actual shape but not the style",
                "allOf": [
                    {
                        "$ref": "#/$defs/shapes/graphic-element"
                    },
                    {
                        "type": "object",
                        "properties": {
                            "d": {
                                "title": "Direction",
                                "description": "Direction the shape is drawn as, mostly relevant when using trim path",
                                "$ref": "#/$defs/constants/shape-direction"
                            }
                        }
                    }
                ]
            },
            "transform": {
                "type": "object",
                "title": "Transform Shape",
                "description": "Group transform",
                "allOf": [
                    {
                        "$ref": "#/$defs/shapes/graphic-element"
                    },
                    {
                        "$ref": "#/$defs/helpers/transform"
                    },
                    {
                        "type": "object",
                        "properties": {
                            "ty": {
                                "title": "Shape Type",
                                "type": "string",
                                "const": "tr"
                            }
                        },
                        "required": [
                            "ty"
                        ]
                    }
                ]
            },
            "graphic-element": {
                "type": "object",
                "title": "Graphic Element",
                "description": "Element used to display vector daya in a shape layer",
                "allOf": [
                    {
                        "$ref": "#/$defs/helpers/visual-object"
                    },
                    {
                        "type": "object",
                        "properties": {
                            "hd": {
                                "title": "Hidden",
                                "description": "Whether the shape is hidden",
                                "type": "boolean"
                            },
                            "ty": {
                                "title": "Shape Type",
                                "type": "string"
                            }
                        },
                        "required": [
                            "ty"
                        ]
                    }
                ]
            },
            "group": {
                "type": "object",
                "title": "Group",
                "description": "Shape Element that can contain other shapes",
                "allOf": [
                    {
                        "$ref": "#/$defs/shapes/graphic-element"
                    },
                    {
                        "type": "object",
                        "properties": {
                            "ty": {
                                "title": "Shape Type",
                                "type": "string",
                                "const": "gr"
                            },
                            "np": {
                                "title": "Number Of Properties",
                                "type": "number"
                            },
                            "it": {
                                "title": "Shapes",
                                "type": "array",
                                "items": {
                                    "$ref": "#/$defs/shapes/all-graphic-elements"
                                }
                            }
                        },
                        "required": [
                            "ty"
                        ]
                    }
                ]
            },
            "fill": {
                "type": "object",
                "title": "Fill",
                "description": "Solid fill color",
                "allOf": [
                    {
                        "$ref": "#/$defs/shapes/shape-style"
                    },
                    {
                        "type": "object",
                        "properties": {
                            "ty": {
                                "title": "Shape Type",
                                "type": "string",
                                "const": "fl"
                            },
                            "c": {
                                "title": "Color",
                                "$ref": "#/$defs/properties/color-property"
                            },
                            "r": {
                                "title": "Fill Rule",
                                "$ref": "#/$defs/constants/fill-rule"
                            }
                        },
                        "required": [
                            "ty",
                            "c"
                        ]
                    }
                ]
            },
            "all-graphic-elements": {
                "$comment": "List of valid shapes",
                "oneOf": [
                    {
                        "$ref": "#/$defs/shapes/ellipse"
                    },
                    {
                        "$ref": "#/$defs/shapes/fill"
                    },
                    {
                        "$ref": "#/$defs/shapes/group"
                    },
                    {
                        "$ref": "#/$defs/shapes/path"
                    },
                    {
                        "$ref": "#/$defs/shapes/polystar"
                    },
                    {
                        "$ref": "#/$defs/shapes/rectangle"
                    },
                    {
                        "$ref": "#/$defs/shapes/transform"
                    },
                    {
                        "$ref": "#/$defs/shapes/trim-path"
                    }
                ]
            },
            "modifier": {
                "type": "object",
                "title": "Modifier",
                "description": "Modifiers change the bezier curves of neighbouring shapes",
                "allOf": [
                    {
                        "$ref": "#/$defs/shapes/graphic-element"
                    }
                ]
            },
            "ellipse": {
                "type": "object",
                "title": "Ellipse",
                "description": "Ellipse shape",
                "allOf": [
                    {
                        "$ref": "#/$defs/shapes/shape"
                    },
                    {
                        "type": "object",
                        "properties": {
                            "ty": {
                                "title": "Shape Type",
                                "type": "string",
                                "const": "el"
                            },
                            "p": {
                                "title": "Position",
                                "$ref": "#/$defs/properties/position-property"
                            },
                            "s": {
                                "title": "Size",
                                "$ref": "#/$defs/properties/vector-property"
                            }
                        },
                        "required": [
                            "ty",
                            "s",
                            "p"
                        ]
                    }
                ]
            }
        },
        "values": {
            "bezier": {
                "type": "object",
                "title": "Bezier",
                "description": "Cubic polybezier",
                "properties": {
                    "c": {
                        "title": "Closed",
                        "type": "boolean",
                        "default": false
                    },
                    "i": {
                        "title": "In Tangents",
                        "type": "array",
                        "description": "Array of points, each point is an array of coordinates.\nThese points are along the `in` tangents relative to the corresponding `v`.",
                        "items": {
                            "$ref": "#/$defs/values/vector",
                            "default": []
                        }
                    },
                    "o": {
                        "title": "Out Tangents",
                        "type": "array",
                        "description": "Array of points, each point is an array of coordinates.\nThese points are along the `out` tangents relative to the corresponding `v`.",
                        "items": {
                            "$ref": "#/$defs/values/vector",
                            "default": []
                        }
                    },
                    "v": {
                        "title": "Vertices",
                        "description": "Array of points, each point is an array of coordinates.\nThese points are along the bezier path",
                        "type": "array",
                        "items": {
                            "$ref": "#/$defs/values/vector",
                            "default": []
                        }
                    }
                },
                "required": [
                    "i",
                    "v",
                    "o"
                ]
            },
            "vector": {
                "type": "array",
                "title": "Vector",
                "description": "An array of numbers",
                "items": {
                    "type": "number"
                }
            },
            "color": {
                "type": "array",
                "title": "Color",
                "description": "Color as a [r, g, b] array with values in [0, 1]",
                "items": {
                    "type": "number",
                    "minimum": 0,
                    "maximum": 1
                },
                "minItems": 3,
                "maxItems": 4
            },
            "int-boolean": {
                "type": "integer",
                "title": "Integer Boolean",
                "description": "Represents boolean values as an integer. `0` is false, `1` is true.",
                "default": 0,
                "examples": [
                    0
                ],
                "enum": [
                    0,
                    1
                ],
                "oneOf": [
                    {
                        "title": "True",
                        "const": 1
                    },
                    {
                        "title": "False",
                        "const": 0
                    }
                ]
            },
            "data-url": {
                "type": "string",
                "title": "Data URL",
                "description": "An embedded data object",
                "pattern": "^data:([\\w/]+)(;base64)?,(.+)$"
            },
            "hexcolor": {
                "type": "string",
                "title": "Hex Color",
                "description": "Color value in hexadecimal format, with two digits per component ('#RRGGBB')",
                "pattern": "^#([a-fA-F0-9]{6})$",
                "examples": [
                    "#FF00AA"
                ]
            }
        }
    }
};
