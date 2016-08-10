A set of common JS functions used across Skia Infrastructure projects.

It contains three global objects, $$, $$$, and sk.

Some functionalilty uses Promises and as such may require a Promise polyfill
if your target browsers don't support Promises, such as IE 11. See
http://caniuse.com/#feat=promises.

To run the tests run

    make test