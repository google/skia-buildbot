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
