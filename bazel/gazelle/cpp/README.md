# Gazelle extension for C++ code

This extension generates `generated_cc_atom` rules, which take in exactly one C++ header or source
file and list its dependencies (usually other header files). This rule type is a very thin
wrapper around `cc_library`, mainly named this way in order to visually distinguish automatically
generated rules from manually-created ones.

Generating these "atom" rules is not sufficient to make a library or executable buildable and
linkable, but it should make defining `cc_library`, `cc_test`, and `cc_binary` rules easier, in
that rule writers do not have to manually compute the dependencies and transitive dependencies.

## See also
 - [The Skia Infra Gazelle extension](https://skia.googlesource.com/buildbot/+/b91509df3c3b71b9c9fb5a225edf574ca940b039/bazel/gazelle/frontend/README.md)
