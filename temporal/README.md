# temporal

Multi-stage Dockerfile build for [temporal](https://github.com/temporalio/temporal).

Note we use a trusted Go image to do the building
and then copy the built executable into our smaller
basealpine image.

Also note that all images are referred to by hash
so we get repeatability.