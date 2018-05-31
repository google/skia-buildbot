const minify = require('../loader.js');

describe('html-template-minifier-webpack', function() {

    describe('single template behavior', function(){
        // This helper creates a valid JS template for more easily
        // testing short snippets of code
        function wrapTemplate(s) {
            return 'const template=html`'+s+'`;';
        }

        it('removes spaces', function(){
            let input = wrapTemplate(`
    <div>       blarg
    </div>`);
            let expectedOutput = wrapTemplate(' <div> blarg </div>');
            let output = minify(input);
            expect(output).toBe(expectedOutput);
        })

        it('removes comments', function(){
            let input = wrapTemplate('<div><!-- inside --> outside</div>');
            let expectedOutput = wrapTemplate('<div> outside</div>');
            let output = minify(input);
            expect(output).toBe(expectedOutput);
        })

        it('removes comments and spaces at the same time', function(){
            let input = wrapTemplate(`<div>
                                        <!-- inside --> outside
                                      </div>`);
            // TODO(maybe do multiple iterations to squeeze a few more spaces out)
            // when there are comments and spaces together.
            let expectedOutput = wrapTemplate('<div>  outside </div>');
            let output = minify(input);
            expect(output).toBe(expectedOutput);
        })

        it('ignores things in <pre> tags', function(){
            let input = wrapTemplate(`<pre>
                                        <!-- inside --> outside
                                      </pre>`);
            let output = minify(input);
            // unchanged
            expect(output).toBe(input);
        })

        it('ignores things in ${} bracket', function(){
            let input = wrapTemplate("<div ${window ? 'foo  ': '  bar'}>" +
"                                       ${'  <!-- idk comments--> '}" +
"                                     </div>");
            let output = minify(input);
            // The pre-minification turns the <!-- into \x3c!--
            let expectedOutput = wrapTemplate('<div ${window?"foo  ":"  bar"}> ${"  \\x3c!-- idk comments--\\x3e "} </div>');
            expect(output).toBe(expectedOutput);
        })

        it('handles nested <pre> and ${} tags', function(){
            // Don't nest <pre> inside of other <pre> or ${} inside of other ${}.
            // That isn't well supported (and is also likely ugly to read).
            let input = wrapTemplate('<pre ${window?"foo  ":"  bar"}>' +
'                                       ${console.log("    ")}' +
'                                     </pre>');
            let output = minify(input);
            // unchanged
            expect(output).toBe(input);
        })

    });

    describe('multiple template behavior', function(){
        const input =          "const a=html`  <div>   alpha    </div>  `,"+
                                     "b=html`  <div><!-- idk comments--></div>  `;";

        const expectedOutput = "const a=html` <div> alpha </div> `,"+
                                     "b=html` <div></div> `;";

        it('can properly minify multiple individual templates', function() {
            let output = minify(input);
            // unchanged
            expect(output).toBe(expectedOutput);
        })
    });

});