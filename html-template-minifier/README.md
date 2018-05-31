html-template-minifier-webpack
==============================

A webpack plugin for (simply) minifiying HTML templates in JS files,
especially HTML templates that might use lit-html or other similar
templating libraries.

Why not use something like [html-minifier](https://github.com/kangax/html-minifier)?
Unfortunately, those html-minifiers tend to require the HTML to be
well formed and having {{ }} in `<tag>`s tend to make them not work.

The minifier currently applies a few simple rules, like removing comments
and replacing many spaces with a single space.  It ignores anything that
would be in ${}, as that is JS that is getting executed.

It is not robust against antagonistic input, but if your templates are
confusing enough to throw it off, they likely could be refactored to
be cleaner (or there is a minification bug).

Usage
-----


Run `npm install html-template-minifier-webpack --save-dev` then,
add a js-based rule to your webpack config's `rules`, for example:

    export.module.rules.push({
        test: /.js$/,
        use: 'html-template-minifier-webpack',
    });


Known pitfalls
--------------

Nested `<pre>` or `${}` blocks can confuse the simplistic pattern matching
that is currently used.  For example, avoid the following in your templates:

    <pre>
        Nested <pre> <!-- Won't work--></pre>
    </pre>

    <div input=${console.log(`${'too complicated'}`)}>