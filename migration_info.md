# Migration info
Command for local testing:
```
bazel build --enable_bzlmod --noenable_workspace //go/sql/pool:pool
```
## Direct dependencies:
* com_github_jackc_pgx_v4
* com_github_jackc_pgconn
* io_bazel_rules_go
## Migration of `bazel_gazelle`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository bazel_gazelle instantiated at:
  /home/jcgregorio/goldmine/WORKSPACE:61:13: in <toplevel>
Repository rule http_archive defined at:
  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>

```

#### Definition
```python
load("@@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
http_archive(
  name = "bazel_gazelle",
  urls = [
    "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.33.0/bazel-gazelle-v0.33.0.tar.gz",
    "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.33.0/bazel-gazelle-v0.33.0.tar.gz"
  ],
  sha256 = "d3fa66a39028e97d76f9e2db8f1b0c11c099e8e01bf363a923074784e451f809",
)
```
**Tip**: URLs usually show which version was used.
</details>

___
Found partially name matches in BCR: `gazelle`

## Migration of `io_bazel_rules_go`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository io_bazel_rules_go instantiated at:
  /home/jcgregorio/goldmine/WORKSPACE:52:13: in <toplevel>
Repository rule http_archive defined at:
  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>

```

#### Definition
```python
load("@@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
http_archive(
  name = "io_bazel_rules_go",
  urls = [
    "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.42.0/rules_go-v0.42.0.zip",
    "https://github.com/bazelbuild/rules_go/releases/download/v0.42.0/rules_go-v0.42.0.zip"
  ],
  sha256 = "91585017debb61982f7054c9688857a2ad1fd823fc3f9cb05048b0025c47d023",
)
```
**Tip**: URLs usually show which version was used.
</details>

___
Found partially name matches in BCR: `rules_go`

It has been introduced as a Bazel module:

	bazel_dep(name = "rules_go", version = "0.57.0", repo_name = "io_bazel_rules_go")
## Migration of `aspect_rules_ts`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository aspect_rules_ts instantiated at:
  /home/jcgregorio/goldmine/WORKSPACE:240:13: in <toplevel>
Repository rule http_archive defined at:
  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>

```

#### Definition
```python
load("@@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
http_archive(
  name = "aspect_rules_ts",
  urls = [
    "https://storage.googleapis.com/skia-world-readable/bazel/bd3e7b17e677d2b8ba1bac3862f0f238ab16edb3e43fb0f0b9308649ea58a2ad.tar.gz",
    "https://github.com/aspect-build/rules_ts/releases/download/v2.1.0/rules_ts-v2.1.0.tar.gz"
  ],
  sha256 = "bd3e7b17e677d2b8ba1bac3862f0f238ab16edb3e43fb0f0b9308649ea58a2ad",
  strip_prefix = "rules_ts-2.1.0",
)
```
**Tip**: URLs usually show which version was used.
</details>

___
Found perfect name match in BCR: `aspect_rules_ts`

It has been introduced as a Bazel module:

	bazel_dep(name = "aspect_rules_ts", version = "3.7.0")
## Migration of `bazel_gazelle`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository bazel_gazelle instantiated at:
  /home/jcgregorio/goldmine/WORKSPACE:61:13: in <toplevel>
Repository rule http_archive defined at:
  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>

```

#### Definition
```python
load("@@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
http_archive(
  name = "bazel_gazelle",
  urls = [
    "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.33.0/bazel-gazelle-v0.33.0.tar.gz",
    "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.33.0/bazel-gazelle-v0.33.0.tar.gz"
  ],
  sha256 = "d3fa66a39028e97d76f9e2db8f1b0c11c099e8e01bf363a923074784e451f809",
)
```
**Tip**: URLs usually show which version was used.
</details>

___
Found partially name matches in BCR: `gazelle`

It has been introduced as a Bazel module:

	bazel_dep(name = "gazelle", version = "0.45.0", repo_name = "bazel_gazelle")
## Migration of `com_github_jackc_pgconn`:
It has been introduced as a go module with the help of `go.mod`:

```
go_deps.from_file(go_mod = "//:go.mod")
go_sdk.from_file(go_mod = "//:go.mod")
```
## Migration of `platforms`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository platforms instantiated at:
  /DEFAULT.WORKSPACE.SUFFIX:147:6: in <toplevel>
  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe
Repository rule local_repository defined at:
  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/local.bzl:64:35: in <toplevel>

```

#### Definition
```python
load("@@bazel_tools//tools/build_defs/repo:local.bzl", "local_repository")
local_repository(
  name = "platforms",
  path = "/home/jcgregorio/.cache/bazel/_bazel_jcgregorio/install/5309d864f9edb3a2e8380ffc84e6b95c/platforms",
)
```
**Tip**: URLs usually show which version was used.
</details>

___
Found perfect name match in BCR: `platforms`

It has been introduced as a Bazel module:

	bazel_dep(name = "platforms", version = "1.0.0")
## Migration of `io_bazel_rules_docker`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository io_bazel_rules_docker instantiated at:
  /home/jcgregorio/goldmine/WORKSPACE:306:13: in <toplevel>
Repository rule http_archive defined at:
  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>

```

#### Definition
```python
load("@@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
http_archive(
  name = "io_bazel_rules_docker",
  urls = [
    "https://storage.googleapis.com/skia-world-readable/bazel/27d53c1d646fc9537a70427ad7b034734d08a9c38924cc6357cc973fed300820.tar.gz",
    "https://github.com/bazelbuild/rules_docker/releases/download/v0.24.0/rules_docker-v0.24.0.tar.gz"
  ],
  sha256 = "27d53c1d646fc9537a70427ad7b034734d08a9c38924cc6357cc973fed300820",
  strip_prefix = "rules_docker-0.24.0",
)
```
**Tip**: URLs usually show which version was used.
</details>

___
	It is not found in BCR. 

	It has been introduced with `use_repo_rule`:

## Migration of `rules_pkg`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository rules_pkg instantiated at:
  /home/jcgregorio/goldmine/WORKSPACE:289:13: in <toplevel>
Repository rule http_archive defined at:
  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>

```

#### Definition
```python
load("@@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
http_archive(
  name = "rules_pkg",
  urls = [
    "https://mirror.bazel.build/github.com/bazelbuild/rules_pkg/releases/download/0.4.0/rules_pkg-0.4.0.tar.gz",
    "https://github.com/bazelbuild/rules_pkg/releases/download/0.4.0/rules_pkg-0.4.0.tar.gz"
  ],
  sha256 = "038f1caa773a7e35b3663865ffb003169c6a71dc995e39bf4815792f385d837d",
)
```
**Tip**: URLs usually show which version was used.
</details>

___
Found perfect name match in BCR: `rules_pkg`

It has been introduced as a Bazel module:

	bazel_dep(name = "rules_pkg", version = "1.1.0")
Repository definition for `git_amd64_linux` is not found in ./resolved_deps.py file, please add `--force/-f` flag to force update it.
Repository definition for `cockroachdb_linux` is not found in ./resolved_deps.py file, please add `--force/-f` flag to force update it.
## Migration of `bazel_skylib`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository bazel_skylib instantiated at:
  /home/jcgregorio/goldmine/WORKSPACE:77:22: in <toplevel>
  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/io_bazel_rules_go/go/private/repositories.bzl:50:12: in go_rules_dependencies
  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/io_bazel_rules_go/go/private/repositories.bzl:288:18: in _maybe
Repository rule http_archive defined at:
  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>

```

#### Definition
```python
load("@@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
http_archive(
  name = "bazel_skylib",
  urls = [
    "https://mirror.bazel.build/github.com/bazelbuild/bazel-skylib/releases/download/1.4.2/bazel-skylib-1.4.2.tar.gz",
    "https://github.com/bazelbuild/bazel-skylib/releases/download/1.4.2/bazel-skylib-1.4.2.tar.gz"
  ],
  sha256 = "66ffd9315665bfaafc96b52278f57c7e2dd09f5ede279ea6d39b2be471e7e3aa",
  strip_prefix = "",
)
```
**Tip**: URLs usually show which version was used.
</details>

___
Found perfect name match in BCR: `bazel_skylib`

Found partially name matches in BCR: `bazel_skylib_gazelle_plugin`

It has been introduced as a Bazel module:

	bazel_dep(name = "bazel_skylib", version = "1.8.1")
## Migration of `aspect_rules_js`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository aspect_rules_js instantiated at:
  /home/jcgregorio/goldmine/WORKSPACE:198:13: in <toplevel>
Repository rule http_archive defined at:
  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>

```

#### Definition
```python
load("@@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
http_archive(
  name = "aspect_rules_js",
  urls = [
    "https://storage.googleapis.com/skia-world-readable/bazel/76a04ef2120ee00231d85d1ff012ede23963733339ad8db81f590791a031f643.tar.gz",
    "https://github.com/aspect-build/rules_js/releases/download/v1.34.1/rules_js-v1.34.1.tar.gz"
  ],
  sha256 = "76a04ef2120ee00231d85d1ff012ede23963733339ad8db81f590791a031f643",
  strip_prefix = "rules_js-1.34.1",
)
```
**Tip**: URLs usually show which version was used.
</details>

___
Found perfect name match in BCR: `aspect_rules_js`

It has been introduced as a Bazel module:

	bazel_dep(name = "aspect_rules_js", version = "2.5.0")
## Migration of `npm`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository npm instantiated at:
  /home/jcgregorio/goldmine/WORKSPACE:221:19: in <toplevel>
  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_rules_js/npm/repositories.bzl:12:24: in npm_translate_lock
  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_rules_js/npm/private/npm_translate_lock.bzl:545:28: in npm_translate_lock
Repository rule npm_translate_lock_rule defined at:
  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_rules_js/npm/private/npm_translate_lock.bzl:133:42: in <toplevel>

```

#### Definition
```python
load("@@aspect_rules_js//npm/private:npm_translate_lock.bzl", "npm_translate_lock_rule")
npm_translate_lock_rule(
  name = "npm",
  additional_file_contents = {  },
  bins = {  },
  custom_postinstalls = {  },
  data = [
    "//:package.json"
  ],
  dev = False,
  external_repository_action_cache = ".aspect/rules/external_repository_action_cache",
  lifecycle_hooks_envs = {  },
  lifecycle_hooks_execution_requirements = {
    "*": [
        "no-sandbox"
    ]
  },
  lifecycle_hooks = {
    "*": [
        "preinstall",
        "install",
        "postinstall"
    ]
  },
  no_optional = False,
  npm_package_lock = "//:package-lock.json",
  npmrc = "//:.npmrc",
  package_visibility = {  },
  patch_args = {
    "*": [
        "-p0"
    ]
  },
  patches = {  },
  pnpm_lock = "//:pnpm-lock.yaml",
  preupdate = [  ],
  prod = False,
  public_hoist_packages = {  },
  quiet = True,
  update_pnpm_lock = True,
  update_pnpm_lock_node_toolchain_prefix = "nodejs",
  verify_node_modules_ignored = "//:.bazelignore",
  npm_package_target_name = "{dirname}",
)
```
**Tip**: URLs usually show which version was used.
</details>

___
	It is not found in BCR. 

	It has been introduced using a module extension:

## Migration of `aspect_rules_esbuild`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository aspect_rules_esbuild instantiated at:
  /home/jcgregorio/goldmine/WORKSPACE:261:13: in <toplevel>
Repository rule http_archive defined at:
  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>

```

#### Definition
```python
load("@@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
http_archive(
  name = "aspect_rules_esbuild",
  urls = [
    "https://storage.googleapis.com/skia-world-readable/bazel/46aab76044f040c1c0bd97672d56324619af4913cb9e96606ec37ddd4605831d.tar.gz",
    "https://github.com/aspect-build/rules_esbuild/releases/download/v0.16.0/rules_esbuild-v0.16.0.tar.gz"
  ],
  sha256 = "46aab76044f040c1c0bd97672d56324619af4913cb9e96606ec37ddd4605831d",
  strip_prefix = "rules_esbuild-0.16.0",
)
```
**Tip**: URLs usually show which version was used.
</details>

___
Found perfect name match in BCR: `aspect_rules_esbuild`

It has been introduced as a Bazel module:

	bazel_dep(name = "aspect_rules_esbuild", version = "0.22.1")
## Migration of `aspect_bazel_lib`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository aspect_bazel_lib instantiated at:
  /home/jcgregorio/goldmine/WORKSPACE:210:22: in <toplevel>
  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_rules_js/js/repositories.bzl:23:17: in rules_js_dependencies
  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_rules_js/js/private/maybe.bzl:7:10: in maybe_http_archive
  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe
Repository rule http_archive defined at:
  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>

```

#### Definition
```python
load("@@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
http_archive(
  name = "aspect_bazel_lib",
  url = "https://github.com/aspect-build/bazel-lib/releases/download/v1.39.0/bazel-lib-v1.39.0.tar.gz",
  sha256 = "4d6010ca5e3bb4d7045b071205afa8db06ec11eb24de3f023d74d77cca765f66",
  strip_prefix = "bazel-lib-1.39.0",
)
```
**Tip**: URLs usually show which version was used.
</details>

___
Found perfect name match in BCR: `aspect_bazel_lib`

It has been introduced as a Bazel module:

	bazel_dep(name = "aspect_bazel_lib", version = "2.21.1")
