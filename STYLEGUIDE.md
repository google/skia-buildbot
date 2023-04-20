## Prettier

Code formatting for TS, JS, CSS, and SCSS files is enforced using
[prettier](https://prettier.io/).

To reformat all code run:

```
make prettier
```

The easiest way to avoid issues is to have prettier format your code on save
from your editor of choice. For VS Code that would be to install the [Prettier
extension](https://marketplace.visualstudio.com/items?itemName=esbenp.prettier-vscode).

## Elements

Elements should exist one per file, with the file name being the same as the element.
If an Element has a helper element that should not be used alone, it may be included
in the same file.

Create new custom elements using [new_element](./new_element/). The command
should be run above your `modules` sub-directory.

```
 bazelisk run //new_element:new_element "--run_under=cd $PWD &&" -- --element-name=<element name> --app-name=<app name>
```

# Python Style Guide

Python code follows the [Google Python Style Guide](https://google.github.io/styleguide/pyguide.html).
