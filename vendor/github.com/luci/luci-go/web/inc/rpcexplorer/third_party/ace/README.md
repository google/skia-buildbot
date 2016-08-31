Ace Editor, incomplete

* Version: v.1.2.3
* Date: 2016-01-17
* Commit hash: e94cb3c7ffccfb2e565cc857f07a988c0989e527
* Originals: https://github.com/ajaxorg/ace-builds/tree/e94cb3c7ffccfb2e565cc857f07a988c0989e527/src-min-noconflict

## Why not bower?

The ace-builds bower package contains .go and .c files in the same directory
and pcg complains about it. There is no a simple way to exclude them, neither
in pcg nor bower. Also ace-builds is 41M while we need only 392K out of it.
