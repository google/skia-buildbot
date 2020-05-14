Javascript Style Guide
===========================

These rules apply to pure JS, as well as JSA inside of Polymer.
Follow the [Google Style Guide](https://google.github.io/styleguide/javascriptguide.xml)
as a baseline, except as follows below.

- We don't use the Closure Compiler or any type comments related to it.
- Use double quotes.
- We support only the "Evergreen Browsers". IE 9 and below are not supported.


Polymer Style Guide
===================

As a baseline, be consistent with standard Polymer style as used by Polymer elements themselves.
Some exceptions and clarifications are listed below.

AJAX requests
-------------

Instead of `<iron-ajax>`, prefer using the family of sk.request methods, found in
[common.js](https://skia.googlesource.com/buildbot/+show/d3624df97a7422c542a739f36668f4831a2cda0b/res/js/common.js)

**Rationale:** It is easier to debug procedural code over the declarative element.
iron-ajax requires looking between template and the JS in the element declaration, which is sometimes a lot of scrolling.
It is easier to read when the logic is all in one place.
Some legacy code may use iron-ajax, but do not follow that example.


Behaviors
---------

Behaviors should exist one per file, with the file name ending in "-behaviors.html".
If a Behavior is not used by any templating logic, consider implementing the logic in
pure JS, using a namespace as appropriate.

**Rationale:** If methods are not used for templating, pure JS is more portable.


Elements
--------

Elements should exist one per file, with the file name being the same as the element.
If an Element has a helper element that should not be used alone, it may be included
in the same file.
Example: [details-summary.html](https://skia.googlesource.com/buildbot/+show/d3624df97a7422c542a739f36668f4831a2cda0b/res/imp/details-summary.html)

Naming Things
-------------

Properties should be named using one word (if possible) or using snake_case.
Private properties and methods should be prefixed with an underscore
(e.g. _super_private).

**Rationale:** The names are consistent in Javascript and HTML land.
Polymer will automatically convert camelCase into sausage-case otherwise, which
leads to hilarity and/or tears.


Python Style Guide
==================

Python code follows the [Google Python Style Guide](https://google.github.io/styleguide/pyguide.html).