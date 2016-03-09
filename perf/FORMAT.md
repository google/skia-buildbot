The Skia Perf Format
====================

The Skia Perf Format is a JSON file that looks like:

```
{
    "gitHash": "fe4a4029a080bc955e9588d05a6cd9eb490845d4", 
    "key": {
        "arch": "x86", 
        "gpu": "GTX660", 
        "model": "ShuttleA", 
        "os": "Ubuntu12"
    }, 
    "options": {
        "system": "UNIX"
    }, 
    "results": {
        "ChunkAlloc_PushPop_640_480": {
            "nonrendering": {
                "min_ms": 0.01485466666666667, 
                "options": {
                    "source_type": "bench"
                } 
            }
        }, 
        "DeferredSurfaceCopy_discardable_640_480": {
            "565": {
                "min_ms": 2.215988, 
                "options": {
                    "source_type": "bench"
                } 
            }, 
    ...
```

  * gitHash - The git hash of the build this was tested at.
  * key - A map of key, value pairs that make up the key that uniquely
         define test results. Note that "config" and "test" are also
         added to the key.
  * options - A map of key, value pairs that are stored with all
         the test results, but don't become part of the key.
  * results - A map of "test" name to the tests results. Note
         that test name is part of the key. Each key in the result
         is mapped to "config" in the key.


```javascript
{
    "gitHash": "fe4a4029a080bc955e9588d05a6cd9eb490845d4",
    "key": {
        "arch": "x86",
        "gpu": "GTX660",
        "model": "ShuttleA",
        "os": "Ubuntu12"
    },
    "options": {
        "system": "UNIX"
    },
    "results": {
        "ChunkAlloc_PushPop_640_480": {  // Added to key as "test".
            "nonrendering": {            // Added to key as "config".
                "ms": 0.0148546,         // Added to key as "sub_result".
            }
        },
        "DeferredSurfaceCopy_discardable_640_480": {
            "565": {
                "ms": 2.215,
            },
            "8888": {
                "ms": 2.223606,
            },
            "gpu": {
							  // You can have as many measurements as you want
								// per test+config, just name them uniquely:
                "wall_time_ms": 0.11,
                "gpu_time_ms": 0.87,
                "options": {                   // Recorded as part of the result,
                                               // but not part of the key.
                    "GL_RENDERER": "GeForce GTX 660/PCIe/SSE2",
                    "GL_SHADING_LANGUAGE_VERSION": "4.40 NVIDIA via Cg compiler",
                    "GL_VENDOR": "NVIDIA Corporation",
                    "GL_VERSION": "4.4.0 NVIDIA 331.49",
                    "source_type": "bench"
                }
            }
        },
    ...
```

To keep the key names stable, all the key names that make up the key
are sorted and then the values are concatenated in that order. For
example, the key for "ms": 2.223606 would be made up of:

    "arch": "x86"
    "config": "8888"
    "gpu": "GTX660"
    "model": "ShuttleA"
    "os": "Ubuntu12"
    "test": "DeferredSurfaceCopy_discardable_640_480"

And so the key for that trace would be:

    x86:8888:GTX660:ShuttleA:Ubuntu12:DeferredSurfaceCopy_discardable_640_480:ms

Note that "ms" becomes part of the key, but is always appended
to the end of the key and not sorted as "sub_result".

Key value pair charactes should come from [0-9a-zA-Z\_], particularly
note no spaces or ":" characters.
