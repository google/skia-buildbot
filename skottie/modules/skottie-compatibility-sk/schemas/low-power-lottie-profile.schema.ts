// prettier-ignore
export const lowPowerLottieProfileSchema = {
  'type': 'object',
  '$ref': '#/$defs/animation',
  '$defs': {
    'composition': {
      'type': 'object',
      'properties': {
        'layers': {
          'type': 'array',
          'items': {
            'allOf': [
              {'$ref': '#/$defs/features/layers/only-supported-shapes'},
              {'$ref': '#/$defs/features/layers/types/no-image-layer'},
              {'$ref': '#/$defs/features/layers/types/no-text-layer'},
              {'$ref': '#/$defs/features/layers/properties/no-effects'},
              {'$ref': '#/$defs/features/layers/properties/no-time-remap'},
              {'$ref': '#/$defs/features/layers/properties/no-time-stretch'},
              {'$ref': '#/$defs/features/layers/properties/no-masks'},
              {'$ref': '#/$defs/features/layers/properties/no-blend-modes'},
              {'$ref': '#/$defs/features/layers/properties/no-layer-styles'},
            ],
          },
        },
      },
    },
    'features': {
      'layers': {
        'types': {
          'no-image-layer': {
            'type': 'object',
            'feature-code': 'layer-image',
            'not': {'properties': {'ty': {'const': 2}}},
          },
          'no-text-layer': {
            'type': 'object',
            'feature-code': 'layer-text',
            'not': {'properties': {'ty': {'const': 5}}},
          },
        },
        'properties': {
          'no-layer-styles': {
            'feature-code': 'styles',
            'type': 'object',
            'properties': {
              'sy': false,
            },
          },
          'no-time-stretch': {
            'feature-code': 'property-time-stretch',
            'type': 'object',
            'properties': {
              'sr': {
                'const': 1,
              },
            },
          },
          'no-time-remap': {
            'feature-code': 'property-timeremap',
            'type': 'object',
            'properties': {
              'tm': false,
            },
          },
          'no-effects': {
            'feature-code': 'effects',
            'type': 'object',
            'properties': {
              'ef': false,
            },
          },
          'no-masks': {
            'feature-code': 'mask',
            'type': 'object',
            'properties': {
              'masksProperties': false,
            },
          },
          'no-blend-modes': {
            'feature-code': 'blend-mode',
            'type': 'object',
            'properties': {
              'bm': {
                'const': 0,
              },
            },
          },
        },
        'only-supported-shapes': {
          'oneOf': [
            {'$ref': '#/$defs/features/non-shape-layer'},
            {'$ref': '#/$defs/features/shape-layer'},
          ],
        },
      },
      'shapes': {
        'types': {
          'no-pucker-and-bloat': {
            'feature-code': 'shape-pucker-and-bloat',
            'not': {
              'type': 'object',
              'properties': {
                'ty': {'const': 'pb'},
              },
            },
          },
          'no-polystar': {
            'feature-code': 'shape-polystar',
            'not': {
              'type': 'object',
              'properties': {
                'ty': {'const': 'sr'},
              },
            },
          },
          'no-stroke': {
            'feature-code': 'shape-stroke',
            'not': {
              'type': 'object',
              'properties': {
                'ty': {'const': 'st'},
              },
            },
          },
          'no-gradient-fill': {
            'feature-code': 'shape-fill-gradient',
            'not': {
              'type': 'object',
              'properties': {
                'ty': {'const': 'gf'},
              },
            },
          },
          'no-gradient-stroke': {
            'feature-code': 'shape-stroke-gradient',
            'not': {
              'type': 'object',
              'properties': {
                'ty': {'const': 'gs'},
              },
            },
          },
          'no-path': {
            'feature-code': 'shape-path',
            'not': {
              'type': 'object',
              'properties': {
                'ty': {'const': 'sh'},
              },
            },
          },
        },
        'all': {
          'oneOf': [
            {
              '$ref': '#/$defs/features/shapes/group',
            },
            {
              '$ref': '#/$defs/features/shapes/non-group',
            },
          ],
          'allOf': [
            {'$ref': '#/$defs/features/shapes/types/no-pucker-and-bloat'},
            {'$ref': '#/$defs/features/shapes/types/no-polystar'},
            {'$ref': '#/$defs/features/shapes/types/no-stroke'},
            {'$ref': '#/$defs/features/shapes/types/no-gradient-stroke'},
          ],
        },
        'group': {
          'type': 'object',
          'properties': {
            'ty': {
              'const': 'gr',
            },
            'it': {
              'type': 'array',
              'items': {
                '$ref': '#/$defs/features/shapes/all',
              },
            },
          },
        },
        'non-group': {
          'type': 'object',
          'properties': {
            'ty': {
              'not': {'const': 'gr'},
            },
          },
        },
      },
      'non-shape-layer': {
        'type': 'object',
        'properties': {
          'ty': {
            'type': 'integer',
            'not': {'const': 4},
          },
        },
      },
      'shape-layer': {
        'type': 'object',
        'properties': {
          'ty': {
            'const': 4,
          },
          'shapes': {
            'type': 'array',
            'items': {
              '$ref': '#/$defs/features/shapes/all',
            },
          },
        },
      },
    },
      'animation': {
        '$ref': '#/$defs/composition',
        'properties': {
          'assets': {
            'type': 'array',
            'items': {
              '$ref': '#/$defs/composition',
            }
          }
        }
      },
  },
};
