// Copied from https://lottie.github.io/lottie-spec/specs/schema/

// prettier-ignore
export const lottieSchema =
{
    "$schema": "https://json-schema.org/draft/2020-12/schema",
    "$id": "https://lottie.github.io/lottie-spec/1.0/specs/schema/",
    "$ref": "#/$defs/composition/animation",
    "$version": 10000,
    "$defs": {
        "assets": {
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
            },
            "asset": {
                "type": "object",
                "title": "Asset",
                "allOf": [
                    {
                        "$ref": "#/$defs/helpers/visual-object"
                    },
                    {
                        "type": "object",
                        "properties": {
                            "id": {
                                "title": "ID",
                                "description": "Unique identifier used by layers when referencing this asset",
                                "type": "string"
                            }
                        },
                        "required": [
                            "id"
                        ]
                    }
                ]
            },
            "image": {
                "type": "object",
                "title": "Image",
                "description": "Asset containing an image that can be referenced by layers.",
                "allOf": [
                    {
                        "$ref": "#/$defs/assets/asset"
                    },
                    {
                        "$ref": "#/$defs/helpers/slottable-object"
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
                                "type": "string"
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
                        "allOf": [
                            {
                                "if": {
                                    "properties": {
                                        "e": {
                                            "const": 1
                                        }
                                    },
                                    "required": [
                                        "e"
                                    ]
                                },
                                "then": {
                                    "properties": {
                                        "p": {
                                            "$ref": "#/$defs/values/data-url"
                                        }
                                    }
                                }
                            }
                        ],
                        "if": {
                            "required": [
                                "sid"
                            ]
                        },
                        "else": {
                            "required": [
                                "w",
                                "h",
                                "p"
                            ]
                        }
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
            }
        },
        "composition": {
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
                            "ver": {
                                "title": "Specification Version",
                                "description": "Specification version this Lottie is targeting. This is a 6 digit number with version components encoded as `MMmmpp`, with `MM` being major version, `mm` being minor and `pp` being patch.",
                                "type": "integer",
                                "minimum": 10000
                            },
                            "fr": {
                                "title": "Framerate",
                                "description": "Framerate in frames per second",
                                "type": "number",
                                "exclusiveMinimum": 0
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
                                "type": "integer",
                                "minimum": 0
                            },
                            "h": {
                                "title": "Height",
                                "description": "Height of the animation",
                                "type": "integer",
                                "minimum": 0
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
                            },
                            "slots": {
                                "title": "Slots",
                                "description": "Dictionary of slot ids that will replace matching properties.",
                                "type": "object",
                                "additionalProperties": {
                                    "$ref": "#/$defs/helpers/slot"
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
            },
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
            }
        },
        "constants": {
            "gradient-type": {
                "type": "integer",
                "title": "Gradient Type",
                "description": "Whether a Gradient is a linear or radial.",
                "oneOf": [
                    {
                        "title": "Linear",
                        "description": "Colors transition in a single linear direction.",
                        "const": 1
                    },
                    {
                        "title": "Radial",
                        "description": "Colors transition outward from a center point.",
                        "const": 2
                    }
                ]
            },
            "line-cap": {
                "type": "integer",
                "title": "Line Cap",
                "description": "Style at the end of a stoked line",
                "oneOf": [
                    {
                        "title": "Butt",
                        "const": 1
                    },
                    {
                        "title": "Round",
                        "const": 2
                    },
                    {
                        "title": "Square",
                        "const": 3
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
            "matte-mode": {
                "type": "integer",
                "title": "Matte Mode",
                "description": "How a layer should mask another layer",
                "oneOf": [
                    {
                        "title": "Normal",
                        "description": "The layer is not used as a track matte",
                        "const": 0
                    },
                    {
                        "title": "Alpha",
                        "description": "The masked layer opacity is modulated by the track matte layer opacity",
                        "const": 1
                    },
                    {
                        "title": "Inverted Alpha",
                        "description": "The masked layer opacity is modulated by the inverted track matte layer opacity",
                        "const": 2
                    },
                    {
                        "title": "Luma",
                        "description": "The masked layer opacity is modulated by the track matte layer luminance",
                        "const": 3
                    },
                    {
                        "title": "Inverted Luma",
                        "description": "The masked layer opacity is modulated by the inverted track matte layer luminance",
                        "const": 4
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
            },
            "mask-mode": {
                "type": "string",
                "title": "Mask Mode",
                "description": "Describes how a mask interacts (blends) with the preceding masks in the stack.",
                "oneOf": [
                    {
                        "title": "None",
                        "const": "n",
                        "description": "The mask is ignored."
                    },
                    {
                        "title": "Add",
                        "const": "a",
                        "description": "Mask coverage is added (Normal blending)."
                    },
                    {
                        "title": "Subtract",
                        "const": "s",
                        "description": "Mask coverage is subtracted (Subtract blending)."
                    },
                    {
                        "title": "Intersect",
                        "const": "i",
                        "description": "Mask coverage is intersected (Source-In blending)."
                    }
                ]
            },
            "line-join": {
                "type": "integer",
                "title": "Line Join",
                "description": "Style at a sharp corner of a stoked line",
                "oneOf": [
                    {
                        "title": "Miter",
                        "const": 1
                    },
                    {
                        "title": "Round",
                        "const": 2
                    },
                    {
                        "title": "Bevel",
                        "const": 3
                    }
                ]
            },
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
            "stroke-dash-type": {
                "type": "string",
                "title": "Stroke Dash Type",
                "description": "Type of a dash item in a stroked line",
                "oneOf": [
                    {
                        "title": "Dash",
                        "const": "d"
                    },
                    {
                        "title": "Gap",
                        "const": "g"
                    },
                    {
                        "title": "Offset",
                        "const": "o"
                    }
                ]
            }
        },
        "helpers": {
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
            },
            "slottable-object": {
                "type": "object",
                "title": "Slottable Object",
                "description": "Object that may have its value replaced with a slot value",
                "properties": {
                    "sid": {
                        "title": "Slot Id",
                        "description": "Identifier to look up the slot",
                        "type": "string"
                    }
                }
            },
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
                                "$ref": "#/$defs/properties/splittable-position-property"
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
            "mask": {
                "type": "object",
                "title": "Mask",
                "description": "Mask for layer content.",
                "allOf": [
                    {
                        "properties": {
                            "mode": {
                                "title": "Mode",
                                "$ref": "#/$defs/constants/mask-mode",
                                "default": "i"
                            },
                            "o": {
                                "title": "Opacity",
                                "description": "Mask opacity, as a percentage [0..100].",
                                "$ref": "#/$defs/properties/scalar-property",
                                "default": 100
                            },
                            "pt": {
                                "title": "Shape",
                                "description": "Mask shape",
                                "$ref": "#/$defs/properties/bezier-property"
                            }
                        },
                        "required": [
                            "pt"
                        ]
                    }
                ]
            },
            "slot": {
                "type": "object",
                "title": "Slot",
                "description": "Defines a property value that will be set to all matched properties",
                "properties": {
                    "p": {
                        "title": "Property Value",
                        "description": "Property Value"
                    }
                },
                "required": [
                    "p"
                ]
            },
            "slottable-property": {
                "type": "object",
                "title": "Slottable Property",
                "description": "Property that may have its value replaced with a slot value",
                "allOf": [
                    {
                        "$ref": "#/$defs/helpers/slottable-object"
                    }
                ],
                "if": {
                    "required": [
                        "sid"
                    ]
                },
                "else": {
                    "required": [
                        "a",
                        "k"
                    ]
                }
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
                                "description": "Human readable name, as seen from editors and the like",
                                "type": "string"
                            }
                        },
                        "required": []
                    }
                ]
            }
        },
        "layers": {
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
                    },
                    {
                        "$ref": "#/$defs/layers/unknown-layer"
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
                            },
                            "sr": {
                                "title": "Time Stretch",
                                "type": "number",
                                "default": 1
                            },
                            "st": {
                                "title": "Start Time",
                                "type": "number",
                                "default": 0
                            },
                            "tm": {
                                "title": "Time Remap",
                                "description": "Timeline remap function (frame index -> time in seconds)",
                                "$ref": "#/$defs/properties/scalar-property"
                            }
                        },
                        "required": [
                            "ty",
                            "refId"
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
            "unknown-layer": {
                "type": "object",
                "title": "Unknown layer types",
                "description": "Unknown layer types. Types not defined by the specification are still allowed.",
                "properties": {
                    "ty": {
                        "not": {
                            "$comment": "enum list is dynamically generated",
                            "enum": [
                                0,
                                2,
                                3,
                                1,
                                4
                            ]
                        }
                    }
                }
            },
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
                                "description": "If 1, the layer will rotate itself to match its animated position path"
                            },
                            "tt": {
                                "title": "Matte Mode",
                                "$ref": "#/$defs/constants/matte-mode",
                                "description": "Defines the track matte mode for the layer"
                            },
                            "tp": {
                                "title": "Matte Parent",
                                "type": "integer",
                                "description": "Index of the layer used as matte, if omitted assume the layer above the current one"
                            },
                            "masksProperties": {
                                "title": "Masks",
                                "description": "Optional array of masks for the layer.",
                                "type": "array",
                                "items": {
                                    "$ref": "#/$defs/helpers/mask"
                                }
                            }
                        },
                        "required": [
                            "ks"
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
                            "ip": {
                                "title": "In Point",
                                "description": "Frame when the layer becomes visible",
                                "type": "number"
                            },
                            "op": {
                                "title": "Out Point",
                                "description": "Frame when the layer becomes invisible",
                                "type": "number"
                            }
                        },
                        "required": [
                            "ty",
                            "ip",
                            "op"
                        ]
                    }
                ]
            }
        },
        "properties": {
            "vector-property": {
                "type": "object",
                "title": "Vector Property",
                "description": "An animatable property that holds an array of numbers",
                "allOf": [
                    {
                        "$ref": "#/$defs/helpers/slottable-property"
                    }
                ],
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
            "gradient-keyframe": {
                "type": "object",
                "title": "Gradient Keyframe",
                "allOf": [
                    {
                        "$ref": "#/$defs/properties/base-keyframe"
                    },
                    {
                        "properties": {
                            "s": {
                                "title": "Value",
                                "description": "Value at this keyframe.",
                                "$ref": "#/$defs/values/gradient"
                            }
                        }
                    }
                ],
                "required": [
                    "s"
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
                                "title": "Value",
                                "description": "Static Value",
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
            "scalar-property": {
                "type": "object",
                "title": "Scalar Property",
                "description": "An animatable property that holds a float",
                "allOf": [
                    {
                        "$ref": "#/$defs/helpers/slottable-property"
                    }
                ],
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
                ]
            },
            "gradient-property": {
                "type": "object",
                "title": "Gradient Property",
                "description": "An animatable property that holds a Gradient",
                "properties": {
                    "p": {
                        "title": "Color stop count",
                        "type": "number"
                    },
                    "k": {
                        "type": "object",
                        "title": "Gradient stops",
                        "description": "Animatable vector representing the gradient stops",
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
                                        "$ref": "#/$defs/values/gradient"
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
                                            "$ref": "#/$defs/properties/gradient-keyframe"
                                        }
                                    }
                                }
                            }
                        ],
                        "required": [
                            "a",
                            "k"
                        ]
                    }
                }
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
                                    "default": 0
                                },
                                "minItems": 1
                            },
                            {
                                "type": "number",
                                "default": 0
                            }
                        ]
                    }
                },
                "required": [
                    "x",
                    "y"
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
                    }
                ],
                "required": [
                    "t"
                ]
            },
            "split-position": {
                "type": "object",
                "title": "Split Position",
                "description": "An animatable position where x and y are definied and animated separately.",
                "properties": {
                    "s": {
                        "title": "Split",
                        "description": "Whether the position has split values",
                        "type": "boolean",
                        "const": true
                    },
                    "x": {
                        "title": "X Position",
                        "description": "X Position",
                        "$ref": "#/$defs/properties/scalar-property"
                    },
                    "y": {
                        "title": "Y Position",
                        "description": "Y Position",
                        "$ref": "#/$defs/properties/scalar-property"
                    }
                },
                "required": [
                    "s",
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
                                "type": "array",
                                "items": {
                                    "$ref": "#/$defs/values/bezier"
                                },
                                "minItems": 1,
                                "maxItems": 1
                            }
                        }
                    }
                ],
                "required": [
                    "s"
                ]
            },
            "position-property": {
                "type": "object",
                "title": "Position Property",
                "description": "An animatable property to represent a position in space",
                "allOf": [
                    {
                        "$ref": "#/$defs/helpers/slottable-property"
                    }
                ],
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
            "color-property": {
                "type": "object",
                "title": "Color Property",
                "description": "An animatable property that holds a Color",
                "allOf": [
                    {
                        "$ref": "#/$defs/helpers/slottable-property"
                    }
                ],
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
                ]
            },
            "splittable-position-property": {
                "type": "object",
                "title": "Splittable Position Property",
                "description": "An animatable position where position values may be defined and animated separately.",
                "oneOf": [
                    {
                        "$comment": "Grouped XY position coordinates",
                        "$ref": "#/$defs/properties/position-property",
                        "properties": {
                            "s": {
                                "title": "Split",
                                "description": "Whether the position has split values",
                                "type": "boolean",
                                "const": false
                            }
                        }
                    },
                    {
                        "$comment": "Split XY position coordinates",
                        "$ref": "#/$defs/properties/split-position"
                    }
                ]
            }
        },
        "shapes": {
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
                        "$ref": "#/$defs/shapes/gradient-fill"
                    },
                    {
                        "$ref": "#/$defs/shapes/gradient-stroke"
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
                        "$ref": "#/$defs/shapes/stroke"
                    },
                    {
                        "$ref": "#/$defs/shapes/transform"
                    },
                    {
                        "$ref": "#/$defs/shapes/trim-path"
                    },
                    {
                        "$ref": "#/$defs/shapes/unknown-shape"
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
            "gradient-stroke": {
                "type": "object",
                "title": "Gradient Stroke",
                "description": "Gradient stroke",
                "allOf": [
                    {
                        "$ref": "#/$defs/shapes/shape-style"
                    },
                    {
                        "$ref": "#/$defs/shapes/base-stroke"
                    },
                    {
                        "$ref": "#/$defs/shapes/base-gradient"
                    },
                    {
                        "type": "object",
                        "properties": {
                            "ty": {
                                "title": "Shape Type",
                                "type": "string",
                                "const": "gs"
                            }
                        },
                        "required": [
                            "ty"
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
            "base-gradient": {
                "type": "object",
                "title": "Base Gradient",
                "description": "Common properties for gradients",
                "allOf": [
                    {
                        "type": "object",
                        "properties": {
                            "g": {
                                "title": "Colors",
                                "description": "Gradient colors",
                                "$ref": "#/$defs/properties/gradient-property"
                            },
                            "s": {
                                "title": "Start Point",
                                "description": "Starting point for the gradient",
                                "$ref": "#/$defs/properties/position-property"
                            },
                            "e": {
                                "title": "End Point",
                                "description": "End point for the gradient",
                                "$ref": "#/$defs/properties/position-property"
                            },
                            "t": {
                                "title": "Gradient Type",
                                "description": "Type of the gradient",
                                "$ref": "#/$defs/constants/gradient-type"
                            },
                            "h": {
                                "title": "Highlight Length",
                                "description": "Highlight Length, as a percentage between `s` and `e`",
                                "$ref": "#/$defs/properties/scalar-property"
                            },
                            "a": {
                                "title": "Highlight Angle",
                                "description": "Highlight Angle in clockwise degrees, relative to the direction from `s` to `e`",
                                "$ref": "#/$defs/properties/scalar-property"
                            }
                        },
                        "required": [
                            "s",
                            "e",
                            "g",
                            "t"
                        ]
                    }
                ]
            },
            "graphic-element": {
                "type": "object",
                "title": "Graphic Element",
                "description": "Element used to display vector data in a shape layer",
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
            "stroke-dash": {
                "type": "object",
                "title": "Stroke Dash",
                "description": "An item used to described the dash pattern in a stroked path",
                "allOf": [
                    {
                        "$ref": "#/$defs/helpers/visual-object"
                    },
                    {
                        "type": "object",
                        "properties": {
                            "n": {
                                "title": "Dash Type",
                                "$ref": "#/$defs/constants/stroke-dash-type",
                                "default": "d"
                            },
                            "v": {
                                "title": "Length",
                                "description": "Length of the dash",
                                "$ref": "#/$defs/properties/scalar-property"
                            }
                        },
                        "required": []
                    }
                ]
            },
            "gradient-fill": {
                "type": "object",
                "title": "Gradient",
                "description": "Gradient fill color",
                "allOf": [
                    {
                        "$ref": "#/$defs/shapes/shape-style"
                    },
                    {
                        "$ref": "#/$defs/shapes/base-gradient"
                    },
                    {
                        "type": "object",
                        "properties": {
                            "ty": {
                                "title": "Shape Type",
                                "type": "string",
                                "const": "gf"
                            },
                            "r": {
                                "title": "Fill Rule",
                                "$ref": "#/$defs/constants/fill-rule"
                            }
                        },
                        "required": [
                            "ty"
                        ]
                    }
                ]
            },
            "base-stroke": {
                "type": "object",
                "title": "Base Stroke",
                "description": "Common properties for stroke styles",
                "allOf": [
                    {
                        "type": "object",
                        "properties": {
                            "lc": {
                                "title": "Line Cap",
                                "$ref": "#/$defs/constants/line-cap",
                                "default": 2
                            },
                            "lj": {
                                "title": "Line Join",
                                "$ref": "#/$defs/constants/line-join",
                                "default": 2
                            },
                            "ml": {
                                "title": "Miter Limit",
                                "type": "number",
                                "default": 0
                            },
                            "ml2": {
                                "title": "Miter Limit",
                                "description": "Animatable alternative to ml",
                                "$ref": "#/$defs/properties/scalar-property"
                            },
                            "w": {
                                "title": "Width",
                                "description": "Stroke width",
                                "$ref": "#/$defs/properties/scalar-property"
                            },
                            "d": {
                                "title": "Dashes",
                                "description": "Dashed line definition",
                                "type": "array",
                                "items": {
                                    "$ref": "#/$defs/shapes/stroke-dash"
                                }
                            }
                        },
                        "required": [
                            "w"
                        ]
                    }
                ]
            },
            "unknown-shape": {
                "type": "object",
                "title": "Unknown shape types",
                "description": "Unknown shape types. Types not defined by the specification are still allowed.",
                "properties": {
                    "ty": {
                        "not": {
                            "$comment": "enum list is dynamically generated",
                            "enum": [
                                "el",
                                "fl",
                                "gf",
                                "gs",
                                "gr",
                                "sh",
                                "sr",
                                "rc",
                                "st",
                                "tr",
                                "tm"
                            ]
                        }
                    }
                }
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
            "stroke": {
                "type": "object",
                "title": "Stroke",
                "description": "Solid stroke",
                "allOf": [
                    {
                        "$ref": "#/$defs/shapes/shape-style"
                    },
                    {
                        "$ref": "#/$defs/shapes/base-stroke"
                    },
                    {
                        "type": "object",
                        "properties": {
                            "ty": {
                                "title": "Shape Type",
                                "type": "string",
                                "const": "st"
                            },
                            "c": {
                                "title": "Color",
                                "description": "Stroke color",
                                "$ref": "#/$defs/properties/color-property"
                            }
                        },
                        "required": [
                            "ty",
                            "c"
                        ]
                    }
                ]
            }
        },
        "values": {
            "hexcolor": {
                "type": "string",
                "title": "Hex Color",
                "description": "Color value in hexadecimal format, with two digits per component ('#RRGGBB')",
                "pattern": "^#([a-fA-F0-9]{6})$",
                "examples": [
                    "#FF00AA"
                ]
            },
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
            "data-url": {
                "type": "string",
                "title": "Data URL",
                "description": "An embedded data object",
                "pattern": "^data:([\\w/]+)(;base64)?,(.+)$"
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
            "vector": {
                "type": "array",
                "title": "Vector",
                "description": "An array of numbers",
                "items": {
                    "type": "number"
                }
            },
            "gradient": {
                "type": "array",
                "title": "Gradient",
                "description": "A flat list of color stops followed by optional transparency stops. A color stop is [offset, red, green, blue]. A transparency stop is [offset, transparency]. All values are between 0 and 1",
                "items": {
                    "type": "number",
                    "minimum": 0,
                    "maximum": 1
                }
            }
        }
    }
}
