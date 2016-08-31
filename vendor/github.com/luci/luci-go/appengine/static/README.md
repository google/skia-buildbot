# appengine/static/

Exposes common static files under `/static/common/`.

## Bootstrap

In your app root run:

    ln -s ../../static/dispatch.yaml
    mkdir static
    ln -s ../../../static/common static/
    ln -s ../../../static/module-static.yaml static/
