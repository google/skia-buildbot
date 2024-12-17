// prettier-ignore
export const lottiePerformanceWarningSchema =
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
            ]
          }
        }
      }
    },
    "features": {
      "layers": {
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
      },
      "shapes": {
        "types": {
          "warn-stroke": {
            "feature-code": "shape-stroke",
            "feature-details": "may cause framerate issues if animated",
            "not": {
              "type": "object",
              "properties": {
                "ty": {
                  "const": "st"
                }
              }
            }
          },
          "warn-gradient-stroke": {
            "feature-code": "shape-stroke-gradient",
            "feature-details": "may cause framerate issues if animated",
            "not": {
              "type": "object",
              "properties": {
                "ty": {
                  "const": "gs"
                }
              }
            }
          },
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
              "$ref": "#/$defs/features/shapes/types/warn-stroke"
            },
            {
              "$ref": "#/$defs/features/shapes/types/warn-gradient-stroke"
            },
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
