# Gazelle extension for Skia Infrastructure front-end code

This Gazelle extension generates Bazel build targets for front-end code (TypeScript, Sass, HTML)
using the rules defined in `//infra-sk/index.bzl`. Specifically, it generates the following kinds of
targets:

 - `ts_library`
 - `karma_test`
 - `nodejs_test`
 - `sass_library`
 - `sk_element`
 - `sk_page`
 - `sk_element_demo_page_server`
 - `sk_element_puppeteer_test`

## Glossary

Normally, we use the word "rule" to refer to Bazel rule and macro definitions, e.g.:

```python
def ts_library(name, srcs, ...):
    ...
```

And we use the word "target" to refer to a specific instance of a rule or macro, e.g.:

```python
ts_library(
    name = "my_lib",
    srcs = ["my_lib.ts"],
)
```

However, the Gazelle API uses the words "rule kind" to refer to what we normally call "rule", and
"rule" to refer to what we normally call "target". This Gazelle extension uses said words in the
same fashion as the Gazelle API to avoid confusion.

## How Gazelle extensions work (high level overview)

This section describes how a typical Gazelle extension works. This particular Gazelle extension
differs in that it uses a custom rule index to resolve dependencies between rules. These differences
are pointed out where necessary.

A Gazelle extension is essentially a `go_library` with a function named `NewLanguage` that provides
an implementation of the
[`language.Language`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/language#Language) Go
interface. This interface provides hooks for generating rules, parsing configuration directives, and
resolving imports to Bazel labels.

Gazelle extensions work in (roughly) three steps, each one corresponding to one method in the
`language.Language` interface:

 1. Index the imports that existing Bazel rules may provide (i.e. what existing rules are we working
    with?).
 2. Generate or update rules in each target directory (i.e. what rules do we need to make/update?).
 3. Resolve the dependencies of any generated or updated rules (i.e. populate the `deps` arguments
    of the rules we made/updated in step 2 with rules from steps 1 and 2).

When the Gazelle binary runs, it will call the `language.Language` interface methods corresponding
to each step in the above order.

For a more in-depth overview, please see
https://github.com/bazelbuild/bazel-gazelle/blob/3fccaeca6a77cc41adcb90c4c8ce0af5c49d2c9d/merger/merger.go#L19.

### Step 1: Index imports

This step takes place in the implementation of the `Imports` method of the `language.Language`
interface (defined in the
[`resolve.Resolver`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle@v0.23.0/resolve#Resolver)
interface, which `language.Language` embeds).

An import is the path of an "import" statement in a programming language. For example, the path of
the following TypeScript import statement is `measurements/units/international`:

```typescript
import { length as meter } from 'measurements/units/international';
```

`Imports` takes as a parameter a Bazel rule (represented as a
[`rule.Rule`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle@v0.23.0/rule#Rule) struct) and
returns the set of imports in the underlying programming language that the rule may provide
(represented as a slice of
[`resolve.ImportSpec`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle@v0.23.0/resolve#ImportSpec)
structs).

As an example, suppose that `Imports` is passed a `rule.Rule` struct that represents the following
`ts_library` rule, defined in a hypothetical `//measurements/units` Bazel package:

```python
# //measurements/units/BUILD.bazel

ts_library(
    name = "units",
    srcs = [
        "customary.ts",
        "imperial.ts",
        "international.ts",
    ],
)
```

In this example, `Imports` should return the following imports:

 - `measurements/units/customary`
 - `measurements/units/imperial`
 - `measurements/units/international`

Note that the imports returned by `Imports` are based exclusively on the file names of the rule's
sources (`srcs` attribute). At no point does `Imports` inspect the contents of the source files.

In step 1, Gazelle invokes `Imports` once for each Bazel rule in the workspace. Gazelle uses the
returned imports to build a
[`resolve.RuleIndex`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle@v0.23.0/resolve#RuleIndex)
struct which maps imports to the rules that might provide them (see e.g. its
[`FindRulesByImport`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle@v0.23.0/resolve#RuleIndex.FindRulesByImport)
method.)

In step 2, Gazelle invokes `Imports` again for each rule generated or updated by the Gazelle
extension, in order to make sure the `resolve.RuleIndex` reflects the changes made by the extension.

In step 3, the `resolve.RuleIndex` is used to resolve the `deps` argument of each rule generated or
updated by the extension.

#### How this Gazelle extension differs

While Gazelle extensions typically rely on the `resolve.RuleIndex` to resolve dependencies, this
particular Gazelle extension uses a custom rule index due to limitations with the
`resolve.RuleIndex` struct. Our implementation of the `Imports` method populates the custom rule
index with all the information required by this extension, and always returns an empty
`resolve.ImportSpec` slice. This results in an empty `resolve.RuleIndex`, but that is OK because we
never use it. In step 3, method `Resolve` will query the custom rule index to resolve any
dependencies between rules, ignoring the `resolve.RuleIndex` built by Gazelle.

### Step 2: Generate or update rules

This step takes place in the implementation of the `GenerateRules` method of the `language.Language`
interface.

`GenerateRules` takes a directory as an argument, and generates rules from source files found in the
directory. It returns a
[`language.GenerateResult`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle@v0.23.0/language#GenerateResult)
struct with the following contents:

 - A list of rules generated from the source files in the directory, represented as `rule.Rule`
   structs (field
   [`Gen`](https://github.com/bazelbuild/bazel-gazelle/blob/e9091445339de2ba7c01c3561f751b64a7fab4a5/language/lang.go#L139)).
 - A list of empty rules, that is, existing rules (defined in the directory's `BUILD.bazel` file)
   that no longer can be built, e.g. because their source files have been deleted (field
   [`Empty`](https://github.com/bazelbuild/bazel-gazelle/blob/e9091445339de2ba7c01c3561f751b64a7fab4a5/language/lang.go#L144)).
 - A list of imports parsed from the source files of each generated rule (field
   [`Imports`](https://github.com/bazelbuild/bazel-gazelle/blob/e9091445339de2ba7c01c3561f751b64a7fab4a5/language/lang.go#L150)).

As an example, suppose that directory `//measurements/conversions` has files `conversions.ts` and
`conversions_test.ts` with the following contents:

```typescript
// //measurements/conversions/conversions.ts

import { mass as lb } from 'measurements/units/customary';
import { mass as kg } from 'measurements/units/international';

export const lbsToKg = (lbs: number) => `${lbs} ${lb} is equal to ${lbs * 0.453592} ${kg}`
```

```typescript
// //measurements/conversions/conversions_test.ts

import { lbsToKg } from './conversions';

describe('conversions', () => {
  it('should convert pounds to kilograms', () => {
    expect(lbsToKg(1)).to.equal('1 pound (lb) is equal to 0.45392 kilogram (kg)');
  });
});
```

In this example, `GenerateRules` should generate the following rules:

```python
# //measurements/conversions/BUILD.bazel

ts_library(
    name = "conversions",
    srcs = ["conversions.ts"],
    # Note that no "deps" argument is generated in this step. Step 3 populates the "deps" argument.
)

karma_test(
    name = "conversions_test",
    srcs = ["conversions_test.ts"],
    # Note that no "deps" argument is generated in this step. Step 3 populates the "deps" argument.
)
```

Field `Imports` of the returned `language.GenerateResult` should be populated with the following
imports, grouped by rule:

- `conversions`: `measurements/units/customary`, `measurements/units/international`.
- `conversions_test`: `./conversions`.

The above imports must be parsed from the sources of each rule. Gazelle extensions may use a parser
for the programming language of the source files, regular expressions, or any other suitable
technique.

As mentioned in step 1, Gazelle will call the `Imports` method with each rule returned by
`GenerateRules` in order to keep the rule index up-to-date.

### Step 3: Resolve dependencies

This step takes place in the implementation of the `Resolve` method of the `language.Language`
interface (defined in the `resolve.Resolver` interface, which `language.Language` embeds).

`Resolve` takes as arguments a (rule, imports) pair returned by `GenerateRules` in step 2, and
populates the `deps` argument of the rule. It does so by querying the `resolve.RuleIndex` for the
rules that provide each import.

Gazelle invokes `Resolve` once for each rule returned by `GenerateRules`.

The example rules from step 2 might look as follows after having their `deps` arguments populated by
`Resolve`:

```python
# //measurements/conversions/BUILD.bazel

ts_library(
    name = "conversions",
    srcs = ["conversions.ts"],
    deps = ["//measurements/units:units"],
)

karma_test(
    name = "conversions_test",
    srcs = ["conversions_test.ts"],
    deps = [":conversions"],
)

```

#### How this Gazelle extension differs

As mentioned in step 1, this particular Gazelle extension ignores Gazelle's `resolve.RuleIndex`, and
uses a custom rule index instead, which is populated in the `Imports` method. Our implementation of
the `Resolve` method uses said custom rule index to resolve dependencies between rules.

## How to add support for additional rule kinds

Support for new rule kinds (e.g. `foo_library`, `bar_binary`, etc.) can be added in three steps:

1. Update method `GenerateRules` to generate, update and delete rules of the new kind, and parse any
   imports present in their source files.

2. Update method `Resolve` to resolve the `deps` argument of rules of the new kind, if necessary.

3. Any rule kinds generated by this Gazelle extension must be included in the return values of
   methods
   [`KnownDirectives`](https://github.com/bazelbuild/bazel-gazelle/blob/e9091445339de2ba7c01c3561f751b64a7fab4a5/config/config.go#L169),
   [`Loads`](https://github.com/bazelbuild/bazel-gazelle/blob/e9091445339de2ba7c01c3561f751b64a7fab4a5/language/lang.go#L76)
   and
   [`Kinds`](https://github.com/bazelbuild/bazel-gazelle/blob/e9091445339de2ba7c01c3561f751b64a7fab4a5/language/lang.go#L71).

## Additional readings

 - Extending Gazelle: https://github.com/bazelbuild/bazel-gazelle/blob/master/extend.rst.
 - Sample Gazelle extension by Ecosia: https://github.com/jasongwartz/bazel_rules_nodejs_contrib.
 - Tentative Gazelle extension for Sass: https://github.com/bazelbuild/rules_sass/pull/75/files.
