// prettier-ignore
export const lowPowerLottieProfileSchema =
{
  "type": "object",
  "$ref": "#/$defs/animation",
  "$defs": {
    "composition": {
      "type": "object",
      "properties": {
        "layers": {
          "type": "array",
          "items": {
            "allOf": [
              {
                "$ref": "#/$defs/features/layers/only-supported-shapes"
              },
              {
                "$ref": "#/$defs/features/effects/only-supported-effects"
              },
              {
                "$ref": "#/$defs/features/layers/only-supported-layer-transforms"
              },
              {
                "$ref": "#/$defs/features/layers/types/no-image-layer"
              },
              {
                "$ref": "#/$defs/features/layers/types/no-text-layer"
              },
              {
                "$ref": "#/$defs/features/layers/properties/no-effects"
              },
              {
                "$ref": "#/$defs/features/layers/properties/no-masks"
              },
              {
                "$ref": "#/$defs/features/layers/properties/no-track-matte"
              },
              {
                "$ref": "#/$defs/features/layers/properties/no-blend-modes"
              },
              {
                "$ref": "#/$defs/features/layers/properties/no-layer-styles"
              }
            ]
          }
        }
      }
    },
    "features": {
      "transforms": {
        "only-supported-transforms": {
          "allOf": [
            {"$ref": "#/$defs/features/transforms/no-skew"},
            {"$ref": "#/$defs/features/transforms/no-skew-axis"}
          ]
        },
        "no-skew": {
          "feature-code": "transform-skew",
          "type": "object",
          "properties": {
            "sk": {
              "type": "object",
              "properties": {
                "k": {
                  "type": "number",
                  "const": 0
                }
              }
            }
          }
        },
        "no-skew-axis": {
          "feature-code": "transform-skew-axis",
          "type": "object",
          "properties": {
            "sa": {
              "type": "object",
              "properties": {
                "k": {
                  "type": "number",
                  "const": 0
                }
              }
            }
          }
        }
      },
      "effects": {
        "only-supported-effects": {
          "type": "object",
          "properties": {
            "ef": {
              "type": "array",
              "items": {
                "allOf": [
                  {
                    "$ref": "#/$defs/features/effects/no-tint"
                  },
                  {
                    "$ref": "#/$defs/features/effects/no-fill"
                  },
                  {
                    "$ref": "#/$defs/features/effects/no-stroke"
                  },
                  {
                    "$ref": "#/$defs/features/effects/no-tritone"
                  },
                  {
                    "$ref": "#/$defs/features/effects/no-pro-levels"
                  },
                  {
                    "$ref": "#/$defs/features/effects/no-drop-shadow"
                  },
                  {
                    "$ref": "#/$defs/features/effects/no-radial-wipe"
                  },
                  {
                    "$ref": "#/$defs/features/effects/no-displacement-map"
                  },
                  {
                    "$ref": "#/$defs/features/effects/no-matte3"
                  },
                  {
                    "$ref": "#/$defs/features/effects/no-gaussian-blur"
                  },
                  {
                    "$ref": "#/$defs/features/effects/no-twirl"
                  },
                  {
                    "$ref": "#/$defs/features/effects/no-mesh-warp"
                  },
                  {
                    "$ref": "#/$defs/features/effects/no-wavy"
                  },
                  {
                    "$ref": "#/$defs/features/effects/no-spherize"
                  },
                  {
                    "$ref": "#/$defs/features/effects/no-puppet"
                  }
                ]
              }
            }
          }
        },
        "no-tint": {
          "feature-code": "effect-tint",
          "type": "object",
          "properties": {
            "ty": {
              "not": {
                "const": 20
              }
            }
          }
        },
        "no-fill": {
          "feature-code": "effect-fill",
          "type": "object",
          "properties": {
            "ty": {
              "not": {
                "const": 21
              }
            }
          }
        },
        "no-stroke": {
          "feature-code": "effect-stroke",
          "type": "object",
          "properties": {
            "ty": {
              "not": {
                "const": 22
              }
            }
          }
        },
        "no-tritone": {
          "feature-code": "effect-tritone",
          "type": "object",
          "properties": {
            "ty": {
              "not": {
                "const": 23
              }
            }
          }
        },
        "no-pro-levels": {
          "feature-code": "effect-pro-levels",
          "$comment": "Not on canilottie",
          "feature-link": "effects",
          "type": "object",
          "properties": {
            "ty": {
              "not": {
                "const": 24
              }
            }
          }
        },
        "no-drop-shadow": {
          "feature-code": "effect-drop-shadow",
          "type": "object",
          "properties": {
            "ty": {
              "not": {
                "const": 25
              }
            }
          }
        },
        "no-radial-wipe": {
          "feature-code": "effect-radial-wipe",
          "type": "object",
          "properties": {
            "ty": {
              "not": {
                "const": 26
              }
            }
          }
        },
        "no-displacement-map": {
          "feature-code": "effect-displacement-map",
          "type": "object",
          "properties": {
            "ty": {
              "not": {
                "const": 27
              }
            }
          }
        },
        "no-matte3": {
          "feature-code": "effect-matte3",
          "$comment": "Not on canilottie",
          "feature-link": "effects",
          "type": "object",
          "properties": {
            "ty": {
              "not": {
                "const": 28
              }
            }
          }
        },
        "no-gaussian-blur": {
          "feature-code": "effect-gaussian-blur",
          "type": "object",
          "properties": {
            "ty": {
              "not": {
                "const": 29
              }
            }
          }
        },
        "no-twirl": {
          "feature-code": "effect-twirl",
          "$comment": "Not on canilottie",
          "feature-link": "effects",
          "type": "object",
          "properties": {
            "ty": {
              "not": {
                "const": 30
              }
            }
          }
        },
        "no-mesh-warp": {
          "feature-code": "effect-mesh-warp",
          "$comment": "Not on canilottie",
          "feature-link": "effects",
          "type": "object",
          "properties": {
            "ty": {
              "not": {
                "const": 31
              }
            }
          }
        },
        "no-wavy": {
          "feature-code": "effect-wavy",
          "$comment": "Not on canilottie",
          "feature-link": "effects",
          "type": "object",
          "properties": {
            "ty": {
              "not": {
                "const": 32
              }
            }
          }
        },
        "no-spherize": {
          "feature-code": "effect-spherize",
          "feature-link": "spherize-effect",
          "type": "object",
          "properties": {
            "ty": {
              "not": {
                "const": 33
              }
            }
          }
        },
        "no-puppet": {
          "feature-code": "effect-puppet",
          "$comment": "Not on canilottie",
          "feature-link": "effects",
          "type": "object",
          "properties": {
            "ty": {
              "not": {
                "const": 34
              }
            }
          }
        }
      },
      "layers": {
        "types": {
          "no-image-layer": {
            "type": "object",
            "feature-code": "layer-image",
            "not": {
              "properties": {
                "ty": {
                  "const": 2
                }
              }
            }
          },
          "no-text-layer": {
            "type": "object",
            "feature-code": "layer-text",
            "not": {
              "properties": {
                "ty": {
                  "const": 5
                }
              }
            }
          }
        },
        "properties": {
          "no-layer-styles": {
            "feature-code": "styles",
            "type": "object",
            "properties": {
              "sy": false
            }
          },
          "no-effects": {
            "feature-code": "effects",
            "type": "object",
            "properties": {
              "ef": false
            }
          },
          "no-masks": {
            "feature-code": "mask",
            "type": "object",
            "properties": {
              "masksProperties": false
            }
          },
          "no-track-matte": {
            "feature-code": "property-matte-mask",
            "type": "object",
            "properties": {
              "tt": {
                "const": 0
              }
            }
          },
          "no-blend-modes": {
            "feature-code": "blend-mode",
            "type": "object",
            "properties": {
              "bm": {
                "const": 0
              }
            }
          }
        },
        "only-supported-shapes": {
          "oneOf": [
            {
              "$ref": "#/$defs/features/non-shape-layer"
            },
            {
              "$ref": "#/$defs/features/shape-layer"
            }
          ]
        },
        "only-supported-layer-transforms": {
          "type": "object",
          "properties": {
            "ks": {
              "$ref": "#/$defs/features/transforms/only-supported-transforms"
            }
          }
        }
      },
      "shapes": {
        "types": {
          "no-pucker-and-bloat": {
            "feature-code": "shape-pucker-and-bloat",
            "not": {
              "type": "object",
              "properties": {
                "ty": {
                  "const": "pb"
                }
              }
            }
          },
          "no-polystar": {
            "feature-code": "shape-polystar",
            "not": {
              "type": "object",
              "properties": {
                "ty": {
                  "const": "sr"
                }
              }
            }
          },
          "no-repeater": {
            "feature-code": "shape-repeater",
            "not": {
              "type": "object",
              "properties": {
                "ty": {
                  "const": "rp"
                }
              }
            }
          },
          "no-merge-path": {
            "feature-code": "shape-merge-path",
            "not": {
              "type": "object",
              "properties": {
                "ty": {
                  "const": "mm"
                }
              }
            }
          },
          "no-gradient-fill-radial-highlight": {
            "feature-code": "shape-fill-gradient-radial-highlight",
            "feature-link": "shape-fill-gradient",
            "not": {
              "type": "object",
              "properties": {
                "ty": {
                  "const": "gf"
                },
                "t": {
                  "const": 2
                },
                "h": {
                  "not": {
                    "type": "object",
                    "properties": {
                      "k": {
                        "type": "number",
                        "const": 0
                      }
                    }
                  }
                }
              }
            }
          },
          "no-gradient-stroke-radial-highlight": {
            "feature-code": "shape-stroke-gradient-radial-highlight",
            "feature-link": "shape-stroke-gradient",
            "not": {
              "type": "object",
              "properties": {
                "ty": {
                  "const": "gs"
                },
                "t": {
                  "const": 2
                },
                "h": {
                  "not": {
                    "type": "object",
                    "properties": {
                      "k": {
                        "type": "number",
                        "const": 0
                      }
                    }
                  }
                }
              }
            }
          },
          "no-stroke-dash": {
            "feature-code": "shape-stroke-dash",
            "feature-link": "shape-stroke",
            "not": {
              "required": ["d"],
              "properties": {
                "d": {
                  "type": "array",
                  "minItems": 1
                },
                "ty": {
                  "const": "st"
                }
              }
            }
          },
          "no-gradient-stroke-dash": {
            "feature-code": "shape-stroke-gradient-dash",
            "feature-link": "shape-stroke-gradient",
            "not": {
              "required": ["d"],
              "properties": {
                "d": {
                  "type": "array",
                  "minItems": 1
                },
                "ty": {
                  "const": "gs"
                }
              }
            }
          },
          "no-path": {
            "feature-code": "shape-path",
            "not": {
              "type": "object",
              "properties": {
                "ty": {
                  "const": "sh"
                }
              }
            }
          },
          "only-supported-shape-transforms": {
            "if": {
              "type": "object",
              "properties": {
                "ty": {
                  "const": "tr"
                }
              }
            },
            "then": {
              "$ref": "#/$defs/features/transforms/only-supported-transforms"
            }
          }
        },
        "all": {
          "oneOf": [
            {
              "$ref": "#/$defs/features/shapes/group"
            },
            {
              "$ref": "#/$defs/features/shapes/non-group"
            }
          ],
          "allOf": [
            {
              "$ref": "#/$defs/features/shapes/types/no-pucker-and-bloat"
            },
            {
              "$ref": "#/$defs/features/shapes/types/no-polystar"
            },
            {
              "$ref": "#/$defs/features/shapes/types/no-merge-path"
            },
            {
              "$ref": "#/$defs/features/shapes/types/no-repeater"
            },
            {
              "$ref": "#/$defs/features/shapes/types/no-gradient-fill-radial-highlight"
            },
            {
              "$ref": "#/$defs/features/shapes/types/no-gradient-stroke-radial-highlight"
            },
            {
              "$ref": "#/$defs/features/shapes/types/no-stroke-dash"
            },
            {
              "$ref": "#/$defs/features/shapes/types/no-gradient-stroke-dash"
            },
            {
              "$ref": "#/$defs/features/shapes/types/only-supported-shape-transforms"
            }
          ]
        },
        "group": {
          "type": "object",
          "properties": {
            "ty": {
              "const": "gr"
            },
            "it": {
              "type": "array",
              "items": {
                "$ref": "#/$defs/features/shapes/all"
              }
            }
          }
        },
        "non-group": {
          "type": "object",
          "properties": {
            "ty": {
              "not": {
                "const": "gr"
              }
            }
          }
        }
      },
      "non-shape-layer": {
        "type": "object",
        "properties": {
          "ty": {
            "type": "integer",
            "not": {
              "const": 4
            }
          }
        }
      },
      "shape-layer": {
        "type": "object",
        "properties": {
          "ty": {
            "const": 4
          },
          "shapes": {
            "type": "array",
            "items": {
              "$ref": "#/$defs/features/shapes/all"
            }
          }
        }
      }
    },
    "animation": {
      "$ref": "#/$defs/composition",
      "type": "object",
      "properties": {
        "assets": {
          "type": "array",
          "items": {
            "$ref": "#/$defs/composition"
          }
        }
      }
    }
  }
}
