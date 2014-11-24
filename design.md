This is a short lived design doc on harmonizing Polymer and common.js across all
projects.

The goal is to have common.js and all commonly used local web components
available for everyone to use and to have a base Gruntfile or includable
node modules that have commonly used commands.

Ideas
=====
First, put commonly used local web components into /res/imp/\*.html.

And put common.js in /res/js/common.js.

Each project has an elements.html that is vulcanized.

Each project has a core.js that is a minified set of JS files needed
for the app, including webcomponents.js, common.js, flot, etc.

The core.js can be modified per project.

The core.js can be built minified or non-minified.

elements.html can be built minified or non-minified.

