resolved = [
     {
          "original_rule_class": "local_repository",
          "original_attributes": {
               "name": "bazel_tools",
               "path": "/home/jcgregorio/.cache/bazel/_bazel_jcgregorio/install/5309d864f9edb3a2e8380ffc84e6b95c/embedded_tools"
          },
          "native": "local_repository(name = \"bazel_tools\", path = __embedded_dir__ + \"/\" + \"embedded_tools\")"
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository rules_python instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:27:13: in <toplevel>\nRepository rule http_archive defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "rules_python",
               "urls": [
                    "https://storage.googleapis.com/skia-world-readable/bazel/cdf6b84084aad8f10bf20b46b77cb48d83c319ebe6458a18e9d2cebf57807cdd.tar.gz",
                    "https://github.com/bazelbuild/rules_python/archive/refs/tags/0.8.1.tar.gz"
               ],
               "sha256": "cdf6b84084aad8f10bf20b46b77cb48d83c319ebe6458a18e9d2cebf57807cdd",
               "strip_prefix": "rules_python-0.8.1"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://storage.googleapis.com/skia-world-readable/bazel/cdf6b84084aad8f10bf20b46b77cb48d83c319ebe6458a18e9d2cebf57807cdd.tar.gz",
                              "https://github.com/bazelbuild/rules_python/archive/refs/tags/0.8.1.tar.gz"
                         ],
                         "sha256": "cdf6b84084aad8f10bf20b46b77cb48d83c319ebe6458a18e9d2cebf57807cdd",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "rules_python-0.8.1",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "rules_python"
                    },
                    "output_tree_hash": "ea637cb2372183d4b24b7962b97dfa12fd43c59467182626d6ecbadfd687cb4b"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository bazel_gazelle instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:61:13: in <toplevel>\nRepository rule http_archive defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "bazel_gazelle",
               "urls": [
                    "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.33.0/bazel-gazelle-v0.33.0.tar.gz",
                    "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.33.0/bazel-gazelle-v0.33.0.tar.gz"
               ],
               "sha256": "d3fa66a39028e97d76f9e2db8f1b0c11c099e8e01bf363a923074784e451f809"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.33.0/bazel-gazelle-v0.33.0.tar.gz",
                              "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.33.0/bazel-gazelle-v0.33.0.tar.gz"
                         ],
                         "sha256": "d3fa66a39028e97d76f9e2db8f1b0c11c099e8e01bf363a923074784e451f809",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "bazel_gazelle"
                    },
                    "output_tree_hash": "565a360d574b62b1396be01c757bebed65e2790f4370653404e5827b8cfeca0d"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository io_bazel_rules_go instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:52:13: in <toplevel>\nRepository rule http_archive defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "io_bazel_rules_go",
               "urls": [
                    "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.42.0/rules_go-v0.42.0.zip",
                    "https://github.com/bazelbuild/rules_go/releases/download/v0.42.0/rules_go-v0.42.0.zip"
               ],
               "sha256": "91585017debb61982f7054c9688857a2ad1fd823fc3f9cb05048b0025c47d023"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.42.0/rules_go-v0.42.0.zip",
                              "https://github.com/bazelbuild/rules_go/releases/download/v0.42.0/rules_go-v0.42.0.zip"
                         ],
                         "sha256": "91585017debb61982f7054c9688857a2ad1fd823fc3f9cb05048b0025c47d023",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "io_bazel_rules_go"
                    },
                    "output_tree_hash": "7a9df935fc2ba7b08945ced2401f13480236221c0a56dd36d2e615bc3b2d0ba1"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository googleapis instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:122:13: in <toplevel>\nRepository rule http_archive defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "googleapis",
               "urls": [
                    "https://storage.googleapis.com/skia-world-readable/bazel/b28c13e99001664eac5f1fb81b44d912d19fbc041e30772263251da131f6573c.zip",
                    "https://github.com/googleapis/googleapis/archive/bb964feba5980ed70c9fb8f84fe6e86694df65b0.zip"
               ],
               "sha256": "b28c13e99001664eac5f1fb81b44d912d19fbc041e30772263251da131f6573c",
               "strip_prefix": "googleapis-bb964feba5980ed70c9fb8f84fe6e86694df65b0",
               "build_file": "//bazel/external:googleapis.BUILD"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://storage.googleapis.com/skia-world-readable/bazel/b28c13e99001664eac5f1fb81b44d912d19fbc041e30772263251da131f6573c.zip",
                              "https://github.com/googleapis/googleapis/archive/bb964feba5980ed70c9fb8f84fe6e86694df65b0.zip"
                         ],
                         "sha256": "b28c13e99001664eac5f1fb81b44d912d19fbc041e30772263251da131f6573c",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "googleapis-bb964feba5980ed70c9fb8f84fe6e86694df65b0",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file": "//bazel/external:googleapis.BUILD",
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "googleapis"
                    },
                    "output_tree_hash": "f3069d2dc99cf708cf81763a9de0f7dfb59a949ca995371e69359c57ddfdb4cb"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository aspect_rules_js instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:198:13: in <toplevel>\nRepository rule http_archive defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "aspect_rules_js",
               "urls": [
                    "https://storage.googleapis.com/skia-world-readable/bazel/76a04ef2120ee00231d85d1ff012ede23963733339ad8db81f590791a031f643.tar.gz",
                    "https://github.com/aspect-build/rules_js/releases/download/v1.34.1/rules_js-v1.34.1.tar.gz"
               ],
               "sha256": "76a04ef2120ee00231d85d1ff012ede23963733339ad8db81f590791a031f643",
               "strip_prefix": "rules_js-1.34.1"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://storage.googleapis.com/skia-world-readable/bazel/76a04ef2120ee00231d85d1ff012ede23963733339ad8db81f590791a031f643.tar.gz",
                              "https://github.com/aspect-build/rules_js/releases/download/v1.34.1/rules_js-v1.34.1.tar.gz"
                         ],
                         "sha256": "76a04ef2120ee00231d85d1ff012ede23963733339ad8db81f590791a031f643",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "rules_js-1.34.1",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "aspect_rules_js"
                    },
                    "output_tree_hash": "7580b45687d95518249575379a212134f729a93562de22543473e191317f3e33"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository rules_nodejs instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:210:22: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_rules_js/js/repositories.bzl:17:17: in rules_js_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_rules_js/js/private/maybe.bzl:7:10: in maybe_http_archive\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\nRepository rule http_archive defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "rules_nodejs",
               "generator_name": "rules_nodejs",
               "generator_function": "rules_js_dependencies",
               "generator_location": None,
               "urls": [
                    "https://github.com/bazelbuild/rules_nodejs/releases/download/5.8.4/rules_nodejs-core-5.8.4.tar.gz"
               ],
               "sha256": "8fc8e300cb67b89ceebd5b8ba6896ff273c84f6099fc88d23f24e7102319d8fd"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://github.com/bazelbuild/rules_nodejs/releases/download/5.8.4/rules_nodejs-core-5.8.4.tar.gz"
                         ],
                         "sha256": "8fc8e300cb67b89ceebd5b8ba6896ff273c84f6099fc88d23f24e7102319d8fd",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "rules_nodejs"
                    },
                    "output_tree_hash": "76e361d09eade6d00747d5881d6f57fa81979e8ac70ce67be5424c32a6b98481"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository bazel_features instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:210:22: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_rules_js/js/repositories.bzl:30:17: in rules_js_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_rules_js/js/private/maybe.bzl:7:10: in maybe_http_archive\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\nRepository rule http_archive defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "bazel_features",
               "generator_name": "bazel_features",
               "generator_function": "rules_js_dependencies",
               "generator_location": None,
               "url": "https://github.com/bazel-contrib/bazel_features/releases/download/v0.1.0/bazel_features-v0.1.0.tar.gz",
               "sha256": "f3082bfcdca73dc77dcd68faace806135a2e08c230b02b1d9fbdbd7db9d9c450",
               "strip_prefix": "bazel_features-0.1.0"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "https://github.com/bazel-contrib/bazel_features/releases/download/v0.1.0/bazel_features-v0.1.0.tar.gz",
                         "urls": [],
                         "sha256": "f3082bfcdca73dc77dcd68faace806135a2e08c230b02b1d9fbdbd7db9d9c450",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "bazel_features-0.1.0",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "bazel_features"
                    },
                    "output_tree_hash": "90165f66d4e7e2f6fa135dc1c6bda5d115d24aea8cc85557b50757799e681738"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository bazel_skylib instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:77:22: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/io_bazel_rules_go/go/private/repositories.bzl:50:12: in go_rules_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/io_bazel_rules_go/go/private/repositories.bzl:288:18: in _maybe\nRepository rule http_archive defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "bazel_skylib",
               "generator_name": "bazel_skylib",
               "generator_function": "go_rules_dependencies",
               "generator_location": None,
               "urls": [
                    "https://mirror.bazel.build/github.com/bazelbuild/bazel-skylib/releases/download/1.4.2/bazel-skylib-1.4.2.tar.gz",
                    "https://github.com/bazelbuild/bazel-skylib/releases/download/1.4.2/bazel-skylib-1.4.2.tar.gz"
               ],
               "sha256": "66ffd9315665bfaafc96b52278f57c7e2dd09f5ede279ea6d39b2be471e7e3aa",
               "strip_prefix": ""
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://mirror.bazel.build/github.com/bazelbuild/bazel-skylib/releases/download/1.4.2/bazel-skylib-1.4.2.tar.gz",
                              "https://github.com/bazelbuild/bazel-skylib/releases/download/1.4.2/bazel-skylib-1.4.2.tar.gz"
                         ],
                         "sha256": "66ffd9315665bfaafc96b52278f57c7e2dd09f5ede279ea6d39b2be471e7e3aa",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "bazel_skylib"
                    },
                    "output_tree_hash": "d580e182a6a53edcaa9fd04b48a7ade7ad17cbac28c44e48135bc72a80faf8be"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository aspect_bazel_lib instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:210:22: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_rules_js/js/repositories.bzl:23:17: in rules_js_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_rules_js/js/private/maybe.bzl:7:10: in maybe_http_archive\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\nRepository rule http_archive defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "aspect_bazel_lib",
               "generator_name": "aspect_bazel_lib",
               "generator_function": "rules_js_dependencies",
               "generator_location": None,
               "url": "https://github.com/aspect-build/bazel-lib/releases/download/v1.39.0/bazel-lib-v1.39.0.tar.gz",
               "sha256": "4d6010ca5e3bb4d7045b071205afa8db06ec11eb24de3f023d74d77cca765f66",
               "strip_prefix": "bazel-lib-1.39.0"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "https://github.com/aspect-build/bazel-lib/releases/download/v1.39.0/bazel-lib-v1.39.0.tar.gz",
                         "urls": [],
                         "sha256": "4d6010ca5e3bb4d7045b071205afa8db06ec11eb24de3f023d74d77cca765f66",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "bazel-lib-1.39.0",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "aspect_bazel_lib"
                    },
                    "output_tree_hash": "b52bd98be3a575fd249a6e8750aec03f2b2ca7efdbda21571988f1c52011e3c5"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_nodejs//nodejs:repositories.bzl%node_repositories",
          "definition_information": "Repository nodejs_linux_amd64 instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:214:27: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_nodejs/nodejs/repositories.bzl:404:26: in nodejs_register_toolchains\nRepository rule node_repositories defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_nodejs/nodejs/repositories.bzl:378:36: in <toplevel>\n",
          "original_attributes": {
               "name": "nodejs_linux_amd64",
               "generator_name": "nodejs_linux_amd64",
               "generator_function": "nodejs_register_toolchains",
               "generator_location": None,
               "node_version": "16.14.0",
               "platform": "linux_amd64"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_nodejs//nodejs:repositories.bzl%node_repositories",
                    "attributes": {
                         "name": "nodejs_linux_amd64",
                         "generator_name": "nodejs_linux_amd64",
                         "generator_function": "nodejs_register_toolchains",
                         "generator_location": None,
                         "node_version": "16.14.0",
                         "platform": "linux_amd64"
                    },
                    "output_tree_hash": "718708f971fbe453c480f134455d2a57aef204c5b218eb98275c33124b48b63a"
               }
          ]
     },
     {
          "original_rule_class": "@@aspect_rules_js//npm/private:npm_import.bzl%npm_import_rule",
          "definition_information": "Repository pnpm instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:221:19: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_rules_js/npm/repositories.bzl:12:24: in npm_translate_lock\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_rules_js/npm/private/npm_translate_lock.bzl:500:25: in npm_translate_lock\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_rules_js/npm/private/pnpm_repository.bzl:32:20: in pnpm_repository\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_rules_js/npm/private/npm_import.bzl:1153:20: in npm_import\nRepository rule npm_import_rule defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_rules_js/npm/private/npm_import.bzl:861:34: in <toplevel>\n",
          "original_attributes": {
               "name": "pnpm",
               "generator_name": "pnpm",
               "generator_function": "npm_translate_lock",
               "generator_location": None,
               "link_packages": {},
               "package": "pnpm",
               "root_package": "",
               "version": "8.10.2",
               "commit": "",
               "custom_postinstall": "",
               "extra_build_content": "load(\"@aspect_rules_js//js:defs.bzl\", \"js_binary\")\njs_binary(name = \"pnpm\", data = glob([\"package/**\"]), entry_point = \"package/dist/pnpm.cjs\", visibility = [\"//visibility:public\"])",
               "integrity": "sha512-B4IJPVumx62UYggbwe8HdQFqS0EJ7KHh/tzqbxEBQ69fUJk9s2xCfU+oxivjkgoyJNsS2nGdJGyhndnxgEjDPA==",
               "lifecycle_hooks": [],
               "link_workspace": "",
               "npm_auth": "",
               "npm_auth_basic": "",
               "npm_auth_password": "",
               "npm_auth_username": "",
               "patch_args": [
                    "-p0"
               ],
               "patches": [],
               "url": ""
          },
          "repositories": [
               {
                    "rule_class": "@@aspect_rules_js//npm/private:npm_import.bzl%npm_import_rule",
                    "attributes": {
                         "name": "pnpm",
                         "generator_name": "pnpm",
                         "generator_function": "npm_translate_lock",
                         "generator_location": None,
                         "link_packages": {},
                         "package": "pnpm",
                         "root_package": "",
                         "version": "8.10.2",
                         "commit": "",
                         "custom_postinstall": "",
                         "extra_build_content": "load(\"@aspect_rules_js//js:defs.bzl\", \"js_binary\")\njs_binary(name = \"pnpm\", data = glob([\"package/**\"]), entry_point = \"package/dist/pnpm.cjs\", visibility = [\"//visibility:public\"])",
                         "integrity": "sha512-B4IJPVumx62UYggbwe8HdQFqS0EJ7KHh/tzqbxEBQ69fUJk9s2xCfU+oxivjkgoyJNsS2nGdJGyhndnxgEjDPA==",
                         "lifecycle_hooks": [],
                         "link_workspace": "",
                         "npm_auth": "",
                         "npm_auth_basic": "",
                         "npm_auth_password": "",
                         "npm_auth_username": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patches": [],
                         "url": ""
                    },
                    "output_tree_hash": "eccc22e2bd35ef3f84683a2afa08e32f354ef3ae4576a127d2e904952f46c284"
               }
          ]
     },
     {
          "original_rule_class": "@@aspect_rules_js//npm/private:npm_translate_lock.bzl%npm_translate_lock_rule",
          "definition_information": "Repository npm instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:221:19: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_rules_js/npm/repositories.bzl:12:24: in npm_translate_lock\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_rules_js/npm/private/npm_translate_lock.bzl:545:28: in npm_translate_lock\nRepository rule npm_translate_lock_rule defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_rules_js/npm/private/npm_translate_lock.bzl:133:42: in <toplevel>\n",
          "original_attributes": {
               "name": "npm",
               "generator_name": "npm",
               "generator_function": "npm_translate_lock",
               "generator_location": None,
               "additional_file_contents": {},
               "bins": {},
               "custom_postinstalls": {},
               "data": [
                    "//:package.json"
               ],
               "dev": False,
               "external_repository_action_cache": ".aspect/rules/external_repository_action_cache",
               "lifecycle_hooks_envs": {},
               "lifecycle_hooks_execution_requirements": {
                    "*": [
                         "no-sandbox"
                    ]
               },
               "lifecycle_hooks": {
                    "*": [
                         "preinstall",
                         "install",
                         "postinstall"
                    ]
               },
               "no_optional": False,
               "npm_package_lock": "//:package-lock.json",
               "npmrc": "//:.npmrc",
               "package_visibility": {},
               "patch_args": {
                    "*": [
                         "-p0"
                    ]
               },
               "patches": {},
               "pnpm_lock": "//:pnpm-lock.yaml",
               "preupdate": [],
               "prod": False,
               "public_hoist_packages": {},
               "quiet": True,
               "update_pnpm_lock": True,
               "update_pnpm_lock_node_toolchain_prefix": "nodejs",
               "verify_node_modules_ignored": "//:.bazelignore",
               "npm_package_target_name": "{dirname}"
          },
          "repositories": [
               {
                    "rule_class": "@@aspect_rules_js//npm/private:npm_translate_lock.bzl%npm_translate_lock_rule",
                    "attributes": {
                         "name": "npm",
                         "generator_name": "npm",
                         "generator_function": "npm_translate_lock",
                         "generator_location": None,
                         "additional_file_contents": {},
                         "bins": {},
                         "custom_postinstalls": {},
                         "data": [
                              "//:package.json"
                         ],
                         "dev": False,
                         "external_repository_action_cache": ".aspect/rules/external_repository_action_cache",
                         "lifecycle_hooks_envs": {},
                         "lifecycle_hooks_execution_requirements": {
                              "*": [
                                   "no-sandbox"
                              ]
                         },
                         "lifecycle_hooks": {
                              "*": [
                                   "preinstall",
                                   "install",
                                   "postinstall"
                              ]
                         },
                         "no_optional": False,
                         "npm_package_lock": "//:package-lock.json",
                         "npmrc": "//:.npmrc",
                         "package_visibility": {},
                         "patch_args": {
                              "*": [
                                   "-p0"
                              ]
                         },
                         "patches": {},
                         "pnpm_lock": "//:pnpm-lock.yaml",
                         "preupdate": [],
                         "prod": False,
                         "public_hoist_packages": {},
                         "quiet": True,
                         "update_pnpm_lock": True,
                         "update_pnpm_lock_node_toolchain_prefix": "nodejs",
                         "verify_node_modules_ignored": "//:.bazelignore",
                         "npm_package_target_name": "{dirname}"
                    },
                    "output_tree_hash": "40a01345b6b63cc0bde9588f8d0b76868e549fc7ceb366314591a86e5726b23a"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository aspect_rules_ts instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:240:13: in <toplevel>\nRepository rule http_archive defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "aspect_rules_ts",
               "urls": [
                    "https://storage.googleapis.com/skia-world-readable/bazel/bd3e7b17e677d2b8ba1bac3862f0f238ab16edb3e43fb0f0b9308649ea58a2ad.tar.gz",
                    "https://github.com/aspect-build/rules_ts/releases/download/v2.1.0/rules_ts-v2.1.0.tar.gz"
               ],
               "sha256": "bd3e7b17e677d2b8ba1bac3862f0f238ab16edb3e43fb0f0b9308649ea58a2ad",
               "strip_prefix": "rules_ts-2.1.0"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://storage.googleapis.com/skia-world-readable/bazel/bd3e7b17e677d2b8ba1bac3862f0f238ab16edb3e43fb0f0b9308649ea58a2ad.tar.gz",
                              "https://github.com/aspect-build/rules_ts/releases/download/v2.1.0/rules_ts-v2.1.0.tar.gz"
                         ],
                         "sha256": "bd3e7b17e677d2b8ba1bac3862f0f238ab16edb3e43fb0f0b9308649ea58a2ad",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "rules_ts-2.1.0",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "aspect_rules_ts"
                    },
                    "output_tree_hash": "7f26c07bf9d9652c2701867ef523b9a43e186f2790d2886f2b1dca41f6b6683f"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository aspect_rules_esbuild instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:261:13: in <toplevel>\nRepository rule http_archive defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "aspect_rules_esbuild",
               "urls": [
                    "https://storage.googleapis.com/skia-world-readable/bazel/46aab76044f040c1c0bd97672d56324619af4913cb9e96606ec37ddd4605831d.tar.gz",
                    "https://github.com/aspect-build/rules_esbuild/releases/download/v0.16.0/rules_esbuild-v0.16.0.tar.gz"
               ],
               "sha256": "46aab76044f040c1c0bd97672d56324619af4913cb9e96606ec37ddd4605831d",
               "strip_prefix": "rules_esbuild-0.16.0"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://storage.googleapis.com/skia-world-readable/bazel/46aab76044f040c1c0bd97672d56324619af4913cb9e96606ec37ddd4605831d.tar.gz",
                              "https://github.com/aspect-build/rules_esbuild/releases/download/v0.16.0/rules_esbuild-v0.16.0.tar.gz"
                         ],
                         "sha256": "46aab76044f040c1c0bd97672d56324619af4913cb9e96606ec37ddd4605831d",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "rules_esbuild-0.16.0",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "aspect_rules_esbuild"
                    },
                    "output_tree_hash": "beb0bca91de5fdbfb6f4bb06e918a35c79b35d0f12b66ceb8ae4cfb47dddfcad"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository rules_pkg instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:289:13: in <toplevel>\nRepository rule http_archive defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "rules_pkg",
               "urls": [
                    "https://mirror.bazel.build/github.com/bazelbuild/rules_pkg/releases/download/0.4.0/rules_pkg-0.4.0.tar.gz",
                    "https://github.com/bazelbuild/rules_pkg/releases/download/0.4.0/rules_pkg-0.4.0.tar.gz"
               ],
               "sha256": "038f1caa773a7e35b3663865ffb003169c6a71dc995e39bf4815792f385d837d"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://mirror.bazel.build/github.com/bazelbuild/rules_pkg/releases/download/0.4.0/rules_pkg-0.4.0.tar.gz",
                              "https://github.com/bazelbuild/rules_pkg/releases/download/0.4.0/rules_pkg-0.4.0.tar.gz"
                         ],
                         "sha256": "038f1caa773a7e35b3663865ffb003169c6a71dc995e39bf4815792f385d837d",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "rules_pkg"
                    },
                    "output_tree_hash": "078307a6beb3424c49c98d1fa55fb8eeac66f344add67f41d2c96ab0f3cd6f49"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository io_bazel_rules_docker instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:306:13: in <toplevel>\nRepository rule http_archive defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "io_bazel_rules_docker",
               "urls": [
                    "https://storage.googleapis.com/skia-world-readable/bazel/27d53c1d646fc9537a70427ad7b034734d08a9c38924cc6357cc973fed300820.tar.gz",
                    "https://github.com/bazelbuild/rules_docker/releases/download/v0.24.0/rules_docker-v0.24.0.tar.gz"
               ],
               "sha256": "27d53c1d646fc9537a70427ad7b034734d08a9c38924cc6357cc973fed300820",
               "strip_prefix": "rules_docker-0.24.0"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://storage.googleapis.com/skia-world-readable/bazel/27d53c1d646fc9537a70427ad7b034734d08a9c38924cc6357cc973fed300820.tar.gz",
                              "https://github.com/bazelbuild/rules_docker/releases/download/v0.24.0/rules_docker-v0.24.0.tar.gz"
                         ],
                         "sha256": "27d53c1d646fc9537a70427ad7b034734d08a9c38924cc6357cc973fed300820",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "rules_docker-0.24.0",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "io_bazel_rules_docker"
                    },
                    "output_tree_hash": "46e158598243746e7f9c32bdcb766ff76aed386e97eea73e09c6276554ccef56"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository bazel_toolchains instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:13:13: in <toplevel>\nRepository rule http_archive defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "bazel_toolchains",
               "urls": [
                    "https://mirror.bazel.build/github.com/bazelbuild/bazel-toolchains/releases/download/4.1.0/bazel-toolchains-4.1.0.tar.gz",
                    "https://github.com/bazelbuild/bazel-toolchains/releases/download/4.1.0/bazel-toolchains-4.1.0.tar.gz"
               ],
               "sha256": "179ec02f809e86abf56356d8898c8bd74069f1bd7c56044050c2cd3d79d0e024",
               "strip_prefix": "bazel-toolchains-4.1.0"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://mirror.bazel.build/github.com/bazelbuild/bazel-toolchains/releases/download/4.1.0/bazel-toolchains-4.1.0.tar.gz",
                              "https://github.com/bazelbuild/bazel-toolchains/releases/download/4.1.0/bazel-toolchains-4.1.0.tar.gz"
                         ],
                         "sha256": "179ec02f809e86abf56356d8898c8bd74069f1bd7c56044050c2cd3d79d0e024",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "bazel-toolchains-4.1.0",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "bazel_toolchains"
                    },
                    "output_tree_hash": "7ef5573ceabea4840ef82399f66d18e61ba51b94612a0079113b674448c2209a"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:local.bzl%local_repository",
          "definition_information": "Repository rules_java_builtin instantiated at:\n  /DEFAULT.WORKSPACE:12:36: in <toplevel>\nRepository rule local_repository defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/local.bzl:64:35: in <toplevel>\n",
          "original_attributes": {
               "name": "rules_java_builtin",
               "path": "/home/jcgregorio/.cache/bazel/_bazel_jcgregorio/install/5309d864f9edb3a2e8380ffc84e6b95c/rules_java"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:local.bzl%local_repository",
                    "attributes": {
                         "name": "rules_java_builtin",
                         "path": "/home/jcgregorio/.cache/bazel/_bazel_jcgregorio/install/5309d864f9edb3a2e8380ffc84e6b95c/rules_java"
                    },
                    "output_tree_hash": "23156af102e8441d4b3e5358092fc1dce333786289d48b1df6503ecb8c735cf3"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:local.bzl%local_repository",
          "definition_information": "Repository internal_platforms_do_not_use instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:153:6: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\nRepository rule local_repository defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/local.bzl:64:35: in <toplevel>\n",
          "original_attributes": {
               "name": "internal_platforms_do_not_use",
               "generator_name": "internal_platforms_do_not_use",
               "generator_function": "maybe",
               "generator_location": None,
               "path": "/home/jcgregorio/.cache/bazel/_bazel_jcgregorio/install/5309d864f9edb3a2e8380ffc84e6b95c/platforms"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:local.bzl%local_repository",
                    "attributes": {
                         "name": "internal_platforms_do_not_use",
                         "generator_name": "internal_platforms_do_not_use",
                         "generator_function": "maybe",
                         "generator_location": None,
                         "path": "/home/jcgregorio/.cache/bazel/_bazel_jcgregorio/install/5309d864f9edb3a2e8380ffc84e6b95c/platforms"
                    },
                    "output_tree_hash": "db797f5ddb49595460e727f2c71af1b3adfed4d65132bbe31bd9d3a06bd95dba"
               }
          ]
     },
     {
          "original_rule_class": "@@internal_platforms_do_not_use//host:extension.bzl%host_platform_repo",
          "definition_information": "Repository host_platform instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:165:6: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\nRepository rule host_platform_repo defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/internal_platforms_do_not_use/host/extension.bzl:51:37: in <toplevel>\n",
          "original_attributes": {
               "name": "host_platform",
               "generator_name": "host_platform",
               "generator_function": "maybe",
               "generator_location": None
          },
          "repositories": [
               {
                    "rule_class": "@@internal_platforms_do_not_use//host:extension.bzl%host_platform_repo",
                    "attributes": {
                         "name": "host_platform",
                         "generator_name": "host_platform",
                         "generator_function": "maybe",
                         "generator_location": None
                    },
                    "output_tree_hash": "7bb7732a410e479305fb8602fbfbe14a04e932eed9f8384852c03def646e87d5"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:local.bzl%local_repository",
          "definition_information": "Repository platforms instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:147:6: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\nRepository rule local_repository defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/local.bzl:64:35: in <toplevel>\n",
          "original_attributes": {
               "name": "platforms",
               "generator_name": "platforms",
               "generator_function": "maybe",
               "generator_location": None,
               "path": "/home/jcgregorio/.cache/bazel/_bazel_jcgregorio/install/5309d864f9edb3a2e8380ffc84e6b95c/platforms"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:local.bzl%local_repository",
                    "attributes": {
                         "name": "platforms",
                         "generator_name": "platforms",
                         "generator_function": "maybe",
                         "generator_location": None,
                         "path": "/home/jcgregorio/.cache/bazel/_bazel_jcgregorio/install/5309d864f9edb3a2e8380ffc84e6b95c/platforms"
                    },
                    "output_tree_hash": "db797f5ddb49595460e727f2c71af1b3adfed4d65132bbe31bd9d3a06bd95dba"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remote_jdk8_linux_aarch64_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:370:22: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:349:34: in remote_jdk8_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remote_jdk8_linux_aarch64_toolchain_config_repo",
               "generator_name": "remote_jdk8_linux_aarch64_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_8\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"8\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_linux_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_linux_aarch64//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remote_jdk8_linux_aarch64_toolchain_config_repo",
                         "generator_name": "remote_jdk8_linux_aarch64_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_8\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"8\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_linux_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_linux_aarch64//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "c9c795851cffbf2a808bfc7cccea597c3b3fef46cfefa084f7e9de7e90b65447"
               }
          ]
     },
     {
          "original_rule_class": "@@io_bazel_rules_go//go/private:sdk.bzl%go_multiple_toolchains",
          "definition_information": "Repository go_sdk_toolchains instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:79:23: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/io_bazel_rules_go/go/private/sdk.bzl:707:28: in go_register_toolchains\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/io_bazel_rules_go/go/private/sdk.bzl:318:19: in go_download_sdk\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/io_bazel_rules_go/go/private/sdk.bzl:306:27: in _go_toolchains\nRepository rule go_multiple_toolchains defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/io_bazel_rules_go/go/private/sdk.bzl:293:41: in <toplevel>\n",
          "original_attributes": {
               "name": "go_sdk_toolchains",
               "generator_name": "go_sdk_toolchains",
               "generator_function": "go_register_toolchains",
               "generator_location": None,
               "prefixes": [
                    ""
               ],
               "sdk_repos": [
                    "go_sdk"
               ],
               "sdk_types": [
                    "remote"
               ],
               "sdk_versions": [
                    "1.21.4"
               ],
               "geese": [
                    ""
               ],
               "goarchs": [
                    ""
               ]
          },
          "repositories": [
               {
                    "rule_class": "@@io_bazel_rules_go//go/private:sdk.bzl%go_multiple_toolchains",
                    "attributes": {
                         "name": "go_sdk_toolchains",
                         "generator_name": "go_sdk_toolchains",
                         "generator_function": "go_register_toolchains",
                         "generator_location": None,
                         "prefixes": [
                              ""
                         ],
                         "sdk_repos": [
                              "go_sdk"
                         ],
                         "sdk_types": [
                              "remote"
                         ],
                         "sdk_versions": [
                              "1.21.4"
                         ],
                         "geese": [
                              ""
                         ],
                         "goarchs": [
                              ""
                         ]
                    },
                    "output_tree_hash": "a41677bbdeb2d7616ac82c5160e645fe54601f727132f7e1796eadb842154aaf"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remote_jdk8_linux_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:370:22: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:349:34: in remote_jdk8_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remote_jdk8_linux_toolchain_config_repo",
               "generator_name": "remote_jdk8_linux_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_8\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"8\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_linux//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_linux//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remote_jdk8_linux_toolchain_config_repo",
                         "generator_name": "remote_jdk8_linux_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_8\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"8\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_linux//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_linux//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "b6a178fc0ca08a4473490f1c5d0f9f633db0ca0f2834c69dd08ce8290cf9ca86"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_nodejs//nodejs/private:toolchains_repo.bzl%toolchains_repo",
          "definition_information": "Repository nodejs_toolchains instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:214:27: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_nodejs/nodejs/repositories.bzl:425:20: in nodejs_register_toolchains\nRepository rule toolchains_repo defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_nodejs/nodejs/private/toolchains_repo.bzl:127:34: in <toplevel>\n",
          "original_attributes": {
               "name": "nodejs_toolchains",
               "generator_name": "nodejs_toolchains",
               "generator_function": "nodejs_register_toolchains",
               "generator_location": None,
               "user_node_repository_name": "nodejs"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_nodejs//nodejs/private:toolchains_repo.bzl%toolchains_repo",
                    "attributes": {
                         "name": "nodejs_toolchains",
                         "generator_name": "nodejs_toolchains",
                         "generator_function": "nodejs_register_toolchains",
                         "generator_location": None,
                         "user_node_repository_name": "nodejs"
                    },
                    "output_tree_hash": "9d1cdbde62627f38d6066fcb7c51f3d02c685662707e1e80e1471d6a6150a8cc"
               }
          ]
     },
     {
          "original_rule_class": "@@aspect_rules_esbuild//esbuild/private:toolchains_repo.bzl%toolchains_repo",
          "definition_information": "Repository esbuild_toolchains instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:277:28: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_rules_esbuild/esbuild/repositories.bzl:114:20: in esbuild_register_toolchains\nRepository rule toolchains_repo defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_rules_esbuild/esbuild/private/toolchains_repo.bzl:108:34: in <toplevel>\n",
          "original_attributes": {
               "name": "esbuild_toolchains",
               "generator_name": "esbuild_toolchains",
               "generator_function": "esbuild_register_toolchains",
               "generator_location": None,
               "esbuild_version": "0.19.2",
               "user_repository_name": "esbuild"
          },
          "repositories": [
               {
                    "rule_class": "@@aspect_rules_esbuild//esbuild/private:toolchains_repo.bzl%toolchains_repo",
                    "attributes": {
                         "name": "esbuild_toolchains",
                         "generator_name": "esbuild_toolchains",
                         "generator_function": "esbuild_register_toolchains",
                         "generator_location": None,
                         "esbuild_version": "0.19.2",
                         "user_repository_name": "esbuild"
                    },
                    "output_tree_hash": "cde991a1e43622b7ebb5b752d34124d53701c5d20d3f624c526ab6ba1939095a"
               }
          ]
     },
     {
          "original_rule_class": "@@aspect_bazel_lib//lib/private:copy_to_directory_toolchain.bzl%copy_to_directory_toolchains_repo",
          "definition_information": "Repository copy_to_directory_toolchains instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:221:19: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_rules_js/npm/repositories.bzl:12:24: in npm_translate_lock\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_rules_js/npm/private/npm_translate_lock.bzl:485:47: in npm_translate_lock\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_bazel_lib/lib/repositories.bzl:200:38: in register_copy_to_directory_toolchains\nRepository rule copy_to_directory_toolchains_repo defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_bazel_lib/lib/private/copy_to_directory_toolchain.bzl:146:52: in <toplevel>\n",
          "original_attributes": {
               "name": "copy_to_directory_toolchains",
               "generator_name": "copy_to_directory_toolchains",
               "generator_function": "npm_translate_lock",
               "generator_location": None,
               "user_repository_name": "copy_to_directory"
          },
          "repositories": [
               {
                    "rule_class": "@@aspect_bazel_lib//lib/private:copy_to_directory_toolchain.bzl%copy_to_directory_toolchains_repo",
                    "attributes": {
                         "name": "copy_to_directory_toolchains",
                         "generator_name": "copy_to_directory_toolchains",
                         "generator_function": "npm_translate_lock",
                         "generator_location": None,
                         "user_repository_name": "copy_to_directory"
                    },
                    "output_tree_hash": "bbe208edab7e408878068af72042aef50aa472366a194f0817af359f1d572110"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remote_jdk8_linux_s390x_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:370:22: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:349:34: in remote_jdk8_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remote_jdk8_linux_s390x_toolchain_config_repo",
               "generator_name": "remote_jdk8_linux_s390x_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_8\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"8\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_linux_s390x//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_linux_s390x//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remote_jdk8_linux_s390x_toolchain_config_repo",
                         "generator_name": "remote_jdk8_linux_s390x_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_8\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"8\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_linux_s390x//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_linux_s390x//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "f1e3f0b4884e21863a7c19a3a12a8995ed4162e02bd07cbb61b42799fc2d7359"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remote_jdk8_macos_aarch64_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:370:22: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:349:34: in remote_jdk8_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remote_jdk8_macos_aarch64_toolchain_config_repo",
               "generator_name": "remote_jdk8_macos_aarch64_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_8\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"8\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_macos_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_macos_aarch64//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remote_jdk8_macos_aarch64_toolchain_config_repo",
                         "generator_name": "remote_jdk8_macos_aarch64_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_8\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"8\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_macos_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_macos_aarch64//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "4d721d8b0731cfb50f963f8b55c7bef9f572de0e2f251f07a12c722ef1acbb2f"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remote_jdk8_macos_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:370:22: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:349:34: in remote_jdk8_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remote_jdk8_macos_toolchain_config_repo",
               "generator_name": "remote_jdk8_macos_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_8\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"8\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_macos//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_macos//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remote_jdk8_macos_toolchain_config_repo",
                         "generator_name": "remote_jdk8_macos_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_8\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"8\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_macos//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_macos//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "e0d82dc2dbe8ec49d859811afe4973ec36226875a39ac7fc8419e91e7e9c89fb"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_python//python/private:toolchains_repo.bzl%toolchains_repo",
          "definition_information": "Repository python3_10_toolchains instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:41:27: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_python/python/repositories.bzl:298:20: in python_register_toolchains\nRepository rule toolchains_repo defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_python/python/private/toolchains_repo.bzl:64:34: in <toplevel>\n",
          "original_attributes": {
               "name": "python3_10_toolchains",
               "generator_name": "python3_10_toolchains",
               "generator_function": "python_register_toolchains",
               "generator_location": None,
               "user_repository_name": "python3_10"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_python//python/private:toolchains_repo.bzl%toolchains_repo",
                    "attributes": {
                         "name": "python3_10_toolchains",
                         "generator_name": "python3_10_toolchains",
                         "generator_function": "python_register_toolchains",
                         "generator_location": None,
                         "user_repository_name": "python3_10"
                    },
                    "output_tree_hash": "81293ce6090d892209808f52cb578d5299e2a04f46999c298c2ebd01f08fb2b6"
               }
          ]
     },
     {
          "original_rule_class": "@@aspect_bazel_lib//lib/private:copy_directory_toolchain.bzl%copy_directory_toolchains_repo",
          "definition_information": "Repository copy_directory_toolchains instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:221:19: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_rules_js/npm/repositories.bzl:12:24: in npm_translate_lock\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_rules_js/npm/private/npm_translate_lock.bzl:483:44: in npm_translate_lock\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_bazel_lib/lib/repositories.bzl:165:35: in register_copy_directory_toolchains\nRepository rule copy_directory_toolchains_repo defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/aspect_bazel_lib/lib/private/copy_directory_toolchain.bzl:146:49: in <toplevel>\n",
          "original_attributes": {
               "name": "copy_directory_toolchains",
               "generator_name": "copy_directory_toolchains",
               "generator_function": "npm_translate_lock",
               "generator_location": None,
               "user_repository_name": "copy_directory"
          },
          "repositories": [
               {
                    "rule_class": "@@aspect_bazel_lib//lib/private:copy_directory_toolchain.bzl%copy_directory_toolchains_repo",
                    "attributes": {
                         "name": "copy_directory_toolchains",
                         "generator_name": "copy_directory_toolchains",
                         "generator_function": "npm_translate_lock",
                         "generator_location": None,
                         "user_repository_name": "copy_directory"
                    },
                    "output_tree_hash": "390580d5c45dc88c79241f1ca1d2e0f73ebb7c564a0624afe7704409c7eb318f"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remote_jdk8_windows_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:370:22: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:349:34: in remote_jdk8_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remote_jdk8_windows_toolchain_config_repo",
               "generator_name": "remote_jdk8_windows_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_8\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"8\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_windows//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_windows//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remote_jdk8_windows_toolchain_config_repo",
                         "generator_name": "remote_jdk8_windows_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_8\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"8\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_windows//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_windows//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "8d0b08c18f215c185d64efe72054a5ffef36325906c34ebf1d3c710d4ba5c685"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk11_linux_aarch64_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:371:23: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:353:34: in remote_jdk11_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk11_linux_aarch64_toolchain_config_repo",
               "generator_name": "remotejdk11_linux_aarch64_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux_aarch64//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk11_linux_aarch64_toolchain_config_repo",
                         "generator_name": "remotejdk11_linux_aarch64_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux_aarch64//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "bef508c068dd47d605f62c53ab0628f1f7f5101fdcc8ada09b2067b36c47931f"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk11_linux_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:371:23: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:353:34: in remote_jdk11_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk11_linux_toolchain_config_repo",
               "generator_name": "remotejdk11_linux_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk11_linux_toolchain_config_repo",
                         "generator_name": "remotejdk11_linux_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "0a170bf4f31e6c4621aeb4d4ce4b75b808be2f3a63cb55dc8172c27707d299ab"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk11_macos_aarch64_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:371:23: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:353:34: in remote_jdk11_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk11_macos_aarch64_toolchain_config_repo",
               "generator_name": "remotejdk11_macos_aarch64_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_macos_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_macos_aarch64//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk11_macos_aarch64_toolchain_config_repo",
                         "generator_name": "remotejdk11_macos_aarch64_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_macos_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_macos_aarch64//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "ca1d067909669aa58188026a7da06d43bdec74a3ba5c122af8a4c3660acd8d8f"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk11_win_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:371:23: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:353:34: in remote_jdk11_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk11_win_toolchain_config_repo",
               "generator_name": "remotejdk11_win_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_win//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_win//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk11_win_toolchain_config_repo",
                         "generator_name": "remotejdk11_win_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_win//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_win//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "d0587a4ecc9323d5cf65314b2d284b520ffb5ee1d3231cc6601efa13dadcc0f4"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk11_macos_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:371:23: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:353:34: in remote_jdk11_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk11_macos_toolchain_config_repo",
               "generator_name": "remotejdk11_macos_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_macos//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_macos//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk11_macos_toolchain_config_repo",
                         "generator_name": "remotejdk11_macos_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_macos//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_macos//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "45b3b36d22d3e614745e7a5e838351c32fe0eabb09a4a197bac0f4d416a950ce"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:local_java_repository.bzl%_local_java_repository_rule",
          "definition_information": "Repository local_jdk instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:85:6: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/local_java_repository.bzl:335:32: in local_java_repository\nRepository rule _local_java_repository_rule defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/local_java_repository.bzl:290:46: in <toplevel>\n",
          "original_attributes": {
               "name": "local_jdk",
               "generator_name": "local_jdk",
               "generator_function": "maybe",
               "generator_location": None,
               "build_file_content": "load(\"@rules_java//java:defs.bzl\", \"java_runtime\")\n\npackage(default_visibility = [\"//visibility:public\"])\n\nexports_files([\"WORKSPACE\", \"BUILD.bazel\"])\n\nfilegroup(\n    name = \"jre\",\n    srcs = glob(\n        [\n            \"jre/bin/**\",\n            \"jre/lib/**\",\n        ],\n        allow_empty = True,\n        # In some configurations, Java browser plugin is considered harmful and\n        # common antivirus software blocks access to npjp2.dll interfering with Bazel,\n        # so do not include it in JRE on Windows.\n        exclude = [\"jre/bin/plugin2/**\"],\n    ),\n)\n\nfilegroup(\n    name = \"jdk-bin\",\n    srcs = glob(\n        [\"bin/**\"],\n        # The JDK on Windows sometimes contains a directory called\n        # \"%systemroot%\", which is not a valid label.\n        exclude = [\"**/*%*/**\"],\n    ),\n)\n\n# This folder holds security policies.\nfilegroup(\n    name = \"jdk-conf\",\n    srcs = glob(\n        [\"conf/**\"],\n        allow_empty = True,\n    ),\n)\n\nfilegroup(\n    name = \"jdk-include\",\n    srcs = glob(\n        [\"include/**\"],\n        allow_empty = True,\n    ),\n)\n\nfilegroup(\n    name = \"jdk-lib\",\n    srcs = glob(\n        [\"lib/**\", \"release\"],\n        allow_empty = True,\n        exclude = [\n            \"lib/missioncontrol/**\",\n            \"lib/visualvm/**\",\n        ],\n    ),\n)\n\njava_runtime(\n    name = \"jdk\",\n    srcs = [\n        \":jdk-bin\",\n        \":jdk-conf\",\n        \":jdk-include\",\n        \":jdk-lib\",\n        \":jre\",\n    ],\n    # Provide the 'java` binary explicitly so that the correct path is used by\n    # Bazel even when the host platform differs from the execution platform.\n    # Exactly one of the two globs will be empty depending on the host platform.\n    # When --incompatible_disallow_empty_glob is enabled, each individual empty\n    # glob will fail without allow_empty = True, even if the overall result is\n    # non-empty.\n    java = glob([\"bin/java.exe\", \"bin/java\"], allow_empty = True)[0],\n    version = {RUNTIME_VERSION},\n)\n\nfilegroup(\n    name = \"jdk-jmods\",\n    srcs = glob(\n        [\"jmods/**\"],\n        allow_empty = True,\n    ),\n)\n\njava_runtime(\n    name = \"jdk-with-jmods\",\n    srcs = [\n        \":jdk-bin\",\n        \":jdk-conf\",\n        \":jdk-include\",\n        \":jdk-lib\",\n        \":jdk-jmods\",\n        \":jre\",\n    ],\n    java = glob([\"bin/java.exe\", \"bin/java\"], allow_empty = True)[0],\n    version = {RUNTIME_VERSION},\n)\n",
               "java_home": "",
               "version": ""
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:local_java_repository.bzl%_local_java_repository_rule",
                    "attributes": {
                         "name": "local_jdk",
                         "generator_name": "local_jdk",
                         "generator_function": "maybe",
                         "generator_location": None,
                         "build_file_content": "load(\"@rules_java//java:defs.bzl\", \"java_runtime\")\n\npackage(default_visibility = [\"//visibility:public\"])\n\nexports_files([\"WORKSPACE\", \"BUILD.bazel\"])\n\nfilegroup(\n    name = \"jre\",\n    srcs = glob(\n        [\n            \"jre/bin/**\",\n            \"jre/lib/**\",\n        ],\n        allow_empty = True,\n        # In some configurations, Java browser plugin is considered harmful and\n        # common antivirus software blocks access to npjp2.dll interfering with Bazel,\n        # so do not include it in JRE on Windows.\n        exclude = [\"jre/bin/plugin2/**\"],\n    ),\n)\n\nfilegroup(\n    name = \"jdk-bin\",\n    srcs = glob(\n        [\"bin/**\"],\n        # The JDK on Windows sometimes contains a directory called\n        # \"%systemroot%\", which is not a valid label.\n        exclude = [\"**/*%*/**\"],\n    ),\n)\n\n# This folder holds security policies.\nfilegroup(\n    name = \"jdk-conf\",\n    srcs = glob(\n        [\"conf/**\"],\n        allow_empty = True,\n    ),\n)\n\nfilegroup(\n    name = \"jdk-include\",\n    srcs = glob(\n        [\"include/**\"],\n        allow_empty = True,\n    ),\n)\n\nfilegroup(\n    name = \"jdk-lib\",\n    srcs = glob(\n        [\"lib/**\", \"release\"],\n        allow_empty = True,\n        exclude = [\n            \"lib/missioncontrol/**\",\n            \"lib/visualvm/**\",\n        ],\n    ),\n)\n\njava_runtime(\n    name = \"jdk\",\n    srcs = [\n        \":jdk-bin\",\n        \":jdk-conf\",\n        \":jdk-include\",\n        \":jdk-lib\",\n        \":jre\",\n    ],\n    # Provide the 'java` binary explicitly so that the correct path is used by\n    # Bazel even when the host platform differs from the execution platform.\n    # Exactly one of the two globs will be empty depending on the host platform.\n    # When --incompatible_disallow_empty_glob is enabled, each individual empty\n    # glob will fail without allow_empty = True, even if the overall result is\n    # non-empty.\n    java = glob([\"bin/java.exe\", \"bin/java\"], allow_empty = True)[0],\n    version = {RUNTIME_VERSION},\n)\n\nfilegroup(\n    name = \"jdk-jmods\",\n    srcs = glob(\n        [\"jmods/**\"],\n        allow_empty = True,\n    ),\n)\n\njava_runtime(\n    name = \"jdk-with-jmods\",\n    srcs = [\n        \":jdk-bin\",\n        \":jdk-conf\",\n        \":jdk-include\",\n        \":jdk-lib\",\n        \":jdk-jmods\",\n        \":jre\",\n    ],\n    java = glob([\"bin/java.exe\", \"bin/java\"], allow_empty = True)[0],\n    version = {RUNTIME_VERSION},\n)\n",
                         "java_home": "",
                         "version": ""
                    },
                    "output_tree_hash": "51f51a2eeb33ccc151e4325f2e611aecf53b42ddfa78d39e38b28dcd48741216"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk11_linux_s390x_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:371:23: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:353:34: in remote_jdk11_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk11_linux_s390x_toolchain_config_repo",
               "generator_name": "remotejdk11_linux_s390x_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux_s390x//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux_s390x//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk11_linux_s390x_toolchain_config_repo",
                         "generator_name": "remotejdk11_linux_s390x_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux_s390x//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux_s390x//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "244e11245106a8495ac4744a90023b87008e3e553766ba11d47a9fe5b4bb408d"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk17_linux_aarch64_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:372:23: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:357:34: in remote_jdk17_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk17_linux_aarch64_toolchain_config_repo",
               "generator_name": "remotejdk17_linux_aarch64_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux_aarch64//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk17_linux_aarch64_toolchain_config_repo",
                         "generator_name": "remotejdk17_linux_aarch64_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux_aarch64//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "b169b01ac1a169d7eb5e3525454c3e408e9127993ac0f578dc2c5ad183fd4e3e"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/sh:sh_configure.bzl%sh_config",
          "definition_information": "Repository local_config_sh instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:187:13: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/sh/sh_configure.bzl:83:14: in sh_configure\nRepository rule sh_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/sh/sh_configure.bzl:72:28: in <toplevel>\n",
          "original_attributes": {
               "name": "local_config_sh",
               "generator_name": "local_config_sh",
               "generator_function": "sh_configure",
               "generator_location": None
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/sh:sh_configure.bzl%sh_config",
                    "attributes": {
                         "name": "local_config_sh",
                         "generator_name": "local_config_sh",
                         "generator_function": "sh_configure",
                         "generator_location": None
                    },
                    "output_tree_hash": "7bf5ba89669bcdf4526f821bc9f1f9f49409ae9c61c4e8f21c9f17e06c475127"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk11_linux_ppc64le_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:371:23: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:353:34: in remote_jdk11_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk11_linux_ppc64le_toolchain_config_repo",
               "generator_name": "remotejdk11_linux_ppc64le_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:ppc\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux_ppc64le//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:ppc\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux_ppc64le//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk11_linux_ppc64le_toolchain_config_repo",
                         "generator_name": "remotejdk11_linux_ppc64le_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:ppc\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux_ppc64le//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:ppc\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux_ppc64le//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "3272b586976beea589d09ea8029fd5d714da40127c8850e3480991c2440c5825"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk11_win_arm64_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:371:23: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:353:34: in remote_jdk11_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk11_win_arm64_toolchain_config_repo",
               "generator_name": "remotejdk11_win_arm64_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:arm64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_win_arm64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:arm64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_win_arm64//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk11_win_arm64_toolchain_config_repo",
                         "generator_name": "remotejdk11_win_arm64_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:arm64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_win_arm64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:arm64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_win_arm64//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "c237bd9668de9b6437c452c020ea5bc717ff80b1a5ffd581adfdc7d4a6c5fe03"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk17_win_arm64_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:372:23: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:357:34: in remote_jdk17_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk17_win_arm64_toolchain_config_repo",
               "generator_name": "remotejdk17_win_arm64_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:arm64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_win_arm64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:arm64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_win_arm64//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk17_win_arm64_toolchain_config_repo",
                         "generator_name": "remotejdk17_win_arm64_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:arm64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_win_arm64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:arm64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_win_arm64//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "86b129d9c464a9b08f97eca7d8bc5bdb3676b581f8aac044451dbdfaa49e69d3"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk17_linux_ppc64le_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:372:23: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:357:34: in remote_jdk17_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk17_linux_ppc64le_toolchain_config_repo",
               "generator_name": "remotejdk17_linux_ppc64le_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:ppc\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux_ppc64le//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:ppc\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux_ppc64le//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk17_linux_ppc64le_toolchain_config_repo",
                         "generator_name": "remotejdk17_linux_ppc64le_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:ppc\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux_ppc64le//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:ppc\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux_ppc64le//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "fdc8ae00f2436bfc46b2f54c84f2bd84122787ede232a4d61ffc284bfe6f61ec"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk21_linux_aarch64_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:373:23: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:361:34: in remote_jdk21_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk21_linux_aarch64_toolchain_config_repo",
               "generator_name": "remotejdk21_linux_aarch64_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux_aarch64//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk21_linux_aarch64_toolchain_config_repo",
                         "generator_name": "remotejdk21_linux_aarch64_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux_aarch64//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "bb33021f243382d2fb849ec204c5c8be5083c37e081df71d34a84324687cf001"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk17_win_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:372:23: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:357:34: in remote_jdk17_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk17_win_toolchain_config_repo",
               "generator_name": "remotejdk17_win_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_win//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_win//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk17_win_toolchain_config_repo",
                         "generator_name": "remotejdk17_win_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_win//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_win//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "170c3c9a35e502555dc9f04b345e064880acbf7df935f673154011356f4aad34"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk17_macos_aarch64_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:372:23: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:357:34: in remote_jdk17_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk17_macos_aarch64_toolchain_config_repo",
               "generator_name": "remotejdk17_macos_aarch64_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_macos_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_macos_aarch64//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk17_macos_aarch64_toolchain_config_repo",
                         "generator_name": "remotejdk17_macos_aarch64_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_macos_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_macos_aarch64//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "0eb17f6d969bc665a21e55d29eb51e88a067159ee62cf5094b17658a07d3accb"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk17_macos_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:372:23: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:357:34: in remote_jdk17_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk17_macos_toolchain_config_repo",
               "generator_name": "remotejdk17_macos_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_macos//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_macos//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk17_macos_toolchain_config_repo",
                         "generator_name": "remotejdk17_macos_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_macos//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_macos//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "41aa7b3317f8d9001746e908454760bf544ffaa058abe22f40711246608022ba"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk21_win_arm64_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:373:23: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:361:34: in remote_jdk21_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk21_win_arm64_toolchain_config_repo",
               "generator_name": "remotejdk21_win_arm64_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:arm64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_win_arm64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:arm64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_win_arm64//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk21_win_arm64_toolchain_config_repo",
                         "generator_name": "remotejdk21_win_arm64_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:arm64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_win_arm64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:arm64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_win_arm64//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "9bbdbb329eeba27bc482582360abc6e3351d9a9a07ee11cba3a0026c90223e85"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk21_linux_s390x_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:373:23: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:361:34: in remote_jdk21_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk21_linux_s390x_toolchain_config_repo",
               "generator_name": "remotejdk21_linux_s390x_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux_s390x//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux_s390x//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk21_linux_s390x_toolchain_config_repo",
                         "generator_name": "remotejdk21_linux_s390x_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux_s390x//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux_s390x//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "30b78e0951c37c2d7ae1318f83045ff42ef261dbb93c5b4fd3ba963e12cf68d6"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk21_linux_ppc64le_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:373:23: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:361:34: in remote_jdk21_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk21_linux_ppc64le_toolchain_config_repo",
               "generator_name": "remotejdk21_linux_ppc64le_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:ppc\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux_ppc64le//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:ppc\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux_ppc64le//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk21_linux_ppc64le_toolchain_config_repo",
                         "generator_name": "remotejdk21_linux_ppc64le_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:ppc\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux_ppc64le//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:ppc\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux_ppc64le//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "7886e497d586c3f3c8225685281b0940e9aa699af208dc98de3db8839e197be3"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk21_win_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:373:23: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:361:34: in remote_jdk21_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk21_win_toolchain_config_repo",
               "generator_name": "remotejdk21_win_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_win//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_win//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk21_win_toolchain_config_repo",
                         "generator_name": "remotejdk21_win_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_win//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_win//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "87012328b07a779503deec0ef47132a0de50efd69afe7df87619bcc07b1dc4ed"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk21_macos_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:373:23: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:361:34: in remote_jdk21_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk21_macos_toolchain_config_repo",
               "generator_name": "remotejdk21_macos_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_macos//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_macos//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk21_macos_toolchain_config_repo",
                         "generator_name": "remotejdk21_macos_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_macos//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_macos//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "434446eddb7f6a3dcc7a2a5330ed9ab26579c5142c19866b197475a695fbb32f"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk21_macos_aarch64_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:373:23: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:361:34: in remote_jdk21_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk21_macos_aarch64_toolchain_config_repo",
               "generator_name": "remotejdk21_macos_aarch64_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_macos_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_macos_aarch64//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk21_macos_aarch64_toolchain_config_repo",
                         "generator_name": "remotejdk21_macos_aarch64_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_macos_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_macos_aarch64//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "706d910cc6809ea7f77fa4f938a4f019dd90d9dad927fb804a14b04321300a36"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk21_linux_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:373:23: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:361:34: in remote_jdk21_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk21_linux_toolchain_config_repo",
               "generator_name": "remotejdk21_linux_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk21_linux_toolchain_config_repo",
                         "generator_name": "remotejdk21_linux_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "ee548ad054c9b75286ff3cd19792e433a2d1236378d3a0d8076fca0bb1a88e05"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk17_linux_s390x_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:372:23: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:357:34: in remote_jdk17_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk17_linux_s390x_toolchain_config_repo",
               "generator_name": "remotejdk17_linux_s390x_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux_s390x//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux_s390x//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk17_linux_s390x_toolchain_config_repo",
                         "generator_name": "remotejdk17_linux_s390x_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux_s390x//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux_s390x//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "6ba1870e09fccfdcd423f4169b966a73f8e9deaff859ec6fb3b626ed61ebd8b5"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk17_linux_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:372:23: in rules_java_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:357:34: in remote_jdk17_repos\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk17_linux_toolchain_config_repo",
               "generator_name": "remotejdk17_linux_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk17_linux_toolchain_config_repo",
                         "generator_name": "remotejdk17_linux_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "f0f07fe0f645f2dc7b8c9953c7962627e1c7425cc52f543729dbff16cd20e461"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository rules_cc instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:50:6: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\nRepository rule http_archive defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "rules_cc",
               "generator_name": "rules_cc",
               "generator_function": "maybe",
               "generator_location": None,
               "urls": [
                    "https://github.com/bazelbuild/rules_cc/releases/download/0.0.9/rules_cc-0.0.9.tar.gz"
               ],
               "sha256": "2037875b9a4456dce4a79d112a8ae885bbc4aad968e6587dca6e64f3a0900cdf",
               "strip_prefix": "rules_cc-0.0.9"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://github.com/bazelbuild/rules_cc/releases/download/0.0.9/rules_cc-0.0.9.tar.gz"
                         ],
                         "sha256": "2037875b9a4456dce4a79d112a8ae885bbc4aad968e6587dca6e64f3a0900cdf",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "rules_cc-0.0.9",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "rules_cc"
                    },
                    "output_tree_hash": "eefc332fe980e25b58e7a4bd9fccd9e1a06dcb06f81423ce66248b4f1e7f8f74"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/cpp:cc_configure.bzl%cc_autoconf_toolchains",
          "definition_information": "Repository local_config_cc_toolchains instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:181:13: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/cpp/cc_configure.bzl:148:27: in cc_configure\nRepository rule cc_autoconf_toolchains defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/cpp/cc_configure.bzl:47:41: in <toplevel>\n",
          "original_attributes": {
               "name": "local_config_cc_toolchains",
               "generator_name": "local_config_cc_toolchains",
               "generator_function": "cc_configure",
               "generator_location": None
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/cpp:cc_configure.bzl%cc_autoconf_toolchains",
                    "attributes": {
                         "name": "local_config_cc_toolchains",
                         "generator_name": "local_config_cc_toolchains",
                         "generator_function": "cc_configure",
                         "generator_location": None
                    },
                    "output_tree_hash": "f95f3d84ac75b4a4d9817af803f1d998a755bd9c20c700c79fa82cb159e98cfc"
               }
          ]
     },
     {
          "original_rule_class": "local_config_platform",
          "original_attributes": {
               "name": "local_config_platform"
          },
          "native": "local_config_platform(name = 'local_config_platform')"
     },
     {
          "original_rule_class": "@@io_bazel_rules_go//go/private:nogo.bzl%go_register_nogo",
          "definition_information": "Repository io_bazel_rules_nogo instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:77:22: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/io_bazel_rules_go/go/private/repositories.bzl:280:12: in go_rules_dependencies\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/io_bazel_rules_go/go/private/repositories.bzl:288:18: in _maybe\nRepository rule go_register_nogo defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/io_bazel_rules_go/go/private/nogo.bzl:31:35: in <toplevel>\n",
          "original_attributes": {
               "name": "io_bazel_rules_nogo",
               "generator_name": "io_bazel_rules_nogo",
               "generator_function": "go_rules_dependencies",
               "generator_location": None,
               "nogo": "@io_bazel_rules_go//:default_nogo"
          },
          "repositories": [
               {
                    "rule_class": "@@io_bazel_rules_go//go/private:nogo.bzl%go_register_nogo",
                    "attributes": {
                         "name": "io_bazel_rules_nogo",
                         "generator_name": "io_bazel_rules_nogo",
                         "generator_function": "go_rules_dependencies",
                         "generator_location": None,
                         "nogo": "@io_bazel_rules_go//:default_nogo"
                    },
                    "output_tree_hash": "e4772e86da6e3bc0887a236dc36324e6b44be0e0315adf28750fcb363acae413"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/osx:xcode_configure.bzl%xcode_autoconf",
          "definition_information": "Repository local_config_xcode instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:184:16: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/osx/xcode_configure.bzl:312:19: in xcode_configure\nRepository rule xcode_autoconf defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/osx/xcode_configure.bzl:297:33: in <toplevel>\n",
          "original_attributes": {
               "name": "local_config_xcode",
               "generator_name": "local_config_xcode",
               "generator_function": "xcode_configure",
               "generator_location": None,
               "xcode_locator": "@bazel_tools//tools/osx:xcode_locator.m"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/osx:xcode_configure.bzl%xcode_autoconf",
                    "attributes": {
                         "name": "local_config_xcode",
                         "generator_name": "local_config_xcode",
                         "generator_function": "xcode_configure",
                         "generator_location": None,
                         "xcode_locator": "@bazel_tools//tools/osx:xcode_locator.m"
                    },
                    "output_tree_hash": "ec026961157bb71cf68d1b568815ad68147ed16f318161bc0da40f6f3d7d79fd"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/cpp:cc_configure.bzl%cc_autoconf",
          "definition_information": "Repository local_config_cc instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:181:13: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/cpp/cc_configure.bzl:149:16: in cc_configure\nRepository rule cc_autoconf defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_tools/tools/cpp/cc_configure.bzl:109:30: in <toplevel>\n",
          "original_attributes": {
               "name": "local_config_cc",
               "generator_name": "local_config_cc",
               "generator_function": "cc_configure",
               "generator_location": None
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/cpp:cc_configure.bzl%cc_autoconf",
                    "attributes": {
                         "name": "local_config_cc",
                         "generator_name": "local_config_cc",
                         "generator_function": "cc_configure",
                         "generator_location": None
                    },
                    "output_tree_hash": "6acf3ffd7e754eed24bb5ca8cf74b19fa0da7bed19eb1a696f300a446da0bacf"
               }
          ]
     },
     {
          "original_rule_class": "@@io_bazel_rules_go//go/private:sdk.bzl%go_download_sdk_rule",
          "definition_information": "Repository go_sdk instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:79:23: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/io_bazel_rules_go/go/private/sdk.bzl:707:28: in go_register_toolchains\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/io_bazel_rules_go/go/private/sdk.bzl:317:25: in go_download_sdk\nRepository rule go_download_sdk_rule defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/io_bazel_rules_go/go/private/sdk.bzl:135:39: in <toplevel>\n",
          "original_attributes": {
               "name": "go_sdk",
               "generator_name": "go_sdk",
               "generator_function": "go_register_toolchains",
               "generator_location": None,
               "version": "1.21.4"
          },
          "repositories": [
               {
                    "rule_class": "@@io_bazel_rules_go//go/private:sdk.bzl%go_download_sdk_rule",
                    "attributes": {
                         "name": "go_sdk",
                         "generator_name": "go_sdk",
                         "generator_function": "go_register_toolchains",
                         "generator_location": None,
                         "version": "1.21.4"
                    },
                    "output_tree_hash": "407803653f372829a6dca4b67185b307e0bc606ad53d29efdc66500ceb354e97"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_gazelle//internal:go_repository_cache.bzl%go_repository_cache",
          "definition_information": "Repository bazel_gazelle_go_repository_cache instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:81:21: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_gazelle/deps.bzl:76:28: in gazelle_dependencies\nRepository rule go_repository_cache defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_gazelle/internal/go_repository_cache.bzl:71:38: in <toplevel>\n",
          "original_attributes": {
               "name": "bazel_gazelle_go_repository_cache",
               "generator_name": "bazel_gazelle_go_repository_cache",
               "generator_function": "gazelle_dependencies",
               "generator_location": None,
               "go_sdk_info": {
                    "go_sdk": "host"
               },
               "go_env": {}
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_gazelle//internal:go_repository_cache.bzl%go_repository_cache",
                    "attributes": {
                         "name": "bazel_gazelle_go_repository_cache",
                         "generator_name": "bazel_gazelle_go_repository_cache",
                         "generator_function": "gazelle_dependencies",
                         "generator_location": None,
                         "go_sdk_info": {
                              "go_sdk": "host"
                         },
                         "go_env": {}
                    },
                    "output_tree_hash": "3204defc529465c633f077e061c3bbb76198bb0c74e6bd54143245b71138ca45"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_gazelle//internal:go_repository_tools.bzl%go_repository_tools",
          "definition_information": "Repository bazel_gazelle_go_repository_tools instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:81:21: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_gazelle/deps.bzl:82:24: in gazelle_dependencies\nRepository rule go_repository_tools defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_gazelle/internal/go_repository_tools.bzl:117:38: in <toplevel>\n",
          "original_attributes": {
               "name": "bazel_gazelle_go_repository_tools",
               "generator_name": "bazel_gazelle_go_repository_tools",
               "generator_function": "gazelle_dependencies",
               "generator_location": None,
               "go_cache": "@@bazel_gazelle_go_repository_cache//:go.env"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_gazelle//internal:go_repository_tools.bzl%go_repository_tools",
                    "attributes": {
                         "name": "bazel_gazelle_go_repository_tools",
                         "generator_name": "bazel_gazelle_go_repository_tools",
                         "generator_function": "gazelle_dependencies",
                         "generator_location": None,
                         "go_cache": "@@bazel_gazelle_go_repository_cache//:go.env"
                    },
                    "output_tree_hash": "5be2eed536d1785dd61597180209e6b70ea56e8704baa98ad4505fd8e72ccad6"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_gazelle//internal:go_repository_config.bzl%go_repository_config",
          "definition_information": "Repository bazel_gazelle_go_repository_config instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:81:21: in <toplevel>\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_gazelle/deps.bzl:87:25: in gazelle_dependencies\nRepository rule go_repository_config defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_gazelle/internal/go_repository_config.bzl:69:39: in <toplevel>\n",
          "original_attributes": {
               "name": "bazel_gazelle_go_repository_config",
               "generator_name": "bazel_gazelle_go_repository_config",
               "generator_function": "gazelle_dependencies",
               "generator_location": None,
               "config": "//:WORKSPACE"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_gazelle//internal:go_repository_config.bzl%go_repository_config",
                    "attributes": {
                         "name": "bazel_gazelle_go_repository_config",
                         "generator_name": "bazel_gazelle_go_repository_config",
                         "generator_function": "gazelle_dependencies",
                         "generator_location": None,
                         "config": "//:WORKSPACE"
                    },
                    "output_tree_hash": "2d54b627ada38ebf31eb35ae056584e06af0ad51ee8b22c56983cd171456287e"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
          "definition_information": "Repository com_github_jackc_pgconn instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:75:16: in <toplevel>\n  /home/jcgregorio/goldmine/go_repositories.bzl:1349:18: in go_repositories\nRepository rule go_repository defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_gazelle/internal/go_repository.bzl:340:32: in <toplevel>\n",
          "original_attributes": {
               "name": "com_github_jackc_pgconn",
               "generator_name": "com_github_jackc_pgconn",
               "generator_function": "go_repositories",
               "generator_location": None,
               "importpath": "github.com/jackc/pgconn",
               "version": "v1.14.0",
               "sum": "h1:vrbA9Ud87g6JdFWkHTJXppVce58qPIdP7N8y0Ml/A7Q="
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
                    "attributes": {
                         "name": "com_github_jackc_pgconn",
                         "generator_name": "com_github_jackc_pgconn",
                         "generator_function": "go_repositories",
                         "generator_location": None,
                         "importpath": "github.com/jackc/pgconn",
                         "version": "v1.14.0",
                         "sum": "h1:vrbA9Ud87g6JdFWkHTJXppVce58qPIdP7N8y0Ml/A7Q="
                    },
                    "output_tree_hash": "f739a7c0575d3d8f6adeba3f5313e13ea02e7df0478094050ca56aa635c389b5"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
          "definition_information": "Repository com_github_jackc_pgx_v4 instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:75:16: in <toplevel>\n  /home/jcgregorio/goldmine/go_repositories.bzl:1404:18: in go_repositories\nRepository rule go_repository defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_gazelle/internal/go_repository.bzl:340:32: in <toplevel>\n",
          "original_attributes": {
               "name": "com_github_jackc_pgx_v4",
               "generator_name": "com_github_jackc_pgx_v4",
               "generator_function": "go_repositories",
               "generator_location": None,
               "importpath": "github.com/jackc/pgx/v4",
               "version": "v4.18.1",
               "sum": "h1:YP7G1KABtKpB5IHrO9vYwSrCOhs7p3uqhvhhQBptya0="
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
                    "attributes": {
                         "name": "com_github_jackc_pgx_v4",
                         "generator_name": "com_github_jackc_pgx_v4",
                         "generator_function": "go_repositories",
                         "generator_location": None,
                         "importpath": "github.com/jackc/pgx/v4",
                         "version": "v4.18.1",
                         "sum": "h1:YP7G1KABtKpB5IHrO9vYwSrCOhs7p3uqhvhhQBptya0="
                    },
                    "output_tree_hash": "09991623b2f9c19800997ea3d542a9063c7fa7dd8308e5f44ea11fe814e51955"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
          "definition_information": "Repository com_github_jackc_pgpassfile instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:75:16: in <toplevel>\n  /home/jcgregorio/goldmine/go_repositories.bzl:1370:18: in go_repositories\nRepository rule go_repository defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_gazelle/internal/go_repository.bzl:340:32: in <toplevel>\n",
          "original_attributes": {
               "name": "com_github_jackc_pgpassfile",
               "generator_name": "com_github_jackc_pgpassfile",
               "generator_function": "go_repositories",
               "generator_location": None,
               "importpath": "github.com/jackc/pgpassfile",
               "version": "v1.0.0",
               "sum": "h1:/6Hmqy13Ss2zCq62VdNG8tM1wchn8zjSGOBJ6icpsIM="
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
                    "attributes": {
                         "name": "com_github_jackc_pgpassfile",
                         "generator_name": "com_github_jackc_pgpassfile",
                         "generator_function": "go_repositories",
                         "generator_location": None,
                         "importpath": "github.com/jackc/pgpassfile",
                         "version": "v1.0.0",
                         "sum": "h1:/6Hmqy13Ss2zCq62VdNG8tM1wchn8zjSGOBJ6icpsIM="
                    },
                    "output_tree_hash": "8c1c028c64daab26c4b58edeecbdfd4aeb691ad5639ca563e66db14b84a2f85e"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
          "definition_information": "Repository com_github_jackc_chunkreader_v2 instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:75:16: in <toplevel>\n  /home/jcgregorio/goldmine/go_repositories.bzl:1342:18: in go_repositories\nRepository rule go_repository defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_gazelle/internal/go_repository.bzl:340:32: in <toplevel>\n",
          "original_attributes": {
               "name": "com_github_jackc_chunkreader_v2",
               "generator_name": "com_github_jackc_chunkreader_v2",
               "generator_function": "go_repositories",
               "generator_location": None,
               "importpath": "github.com/jackc/chunkreader/v2",
               "version": "v2.0.1",
               "sum": "h1:i+RDz65UE+mmpjTfyz0MoVTnzeYxroil2G82ki7MGG8="
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
                    "attributes": {
                         "name": "com_github_jackc_chunkreader_v2",
                         "generator_name": "com_github_jackc_chunkreader_v2",
                         "generator_function": "go_repositories",
                         "generator_location": None,
                         "importpath": "github.com/jackc/chunkreader/v2",
                         "version": "v2.0.1",
                         "sum": "h1:i+RDz65UE+mmpjTfyz0MoVTnzeYxroil2G82ki7MGG8="
                    },
                    "output_tree_hash": "5223c63959c01e60a9de0523c907835044244fa2e4cc5c9e2fc02bc20d082d51"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
          "definition_information": "Repository com_github_jackc_pgio instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:75:16: in <toplevel>\n  /home/jcgregorio/goldmine/go_repositories.bzl:1356:18: in go_repositories\nRepository rule go_repository defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_gazelle/internal/go_repository.bzl:340:32: in <toplevel>\n",
          "original_attributes": {
               "name": "com_github_jackc_pgio",
               "generator_name": "com_github_jackc_pgio",
               "generator_function": "go_repositories",
               "generator_location": None,
               "importpath": "github.com/jackc/pgio",
               "version": "v1.0.0",
               "sum": "h1:g12B9UwVnzGhueNavwioyEEpAmqMe1E/BN9ES+8ovkE="
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
                    "attributes": {
                         "name": "com_github_jackc_pgio",
                         "generator_name": "com_github_jackc_pgio",
                         "generator_function": "go_repositories",
                         "generator_location": None,
                         "importpath": "github.com/jackc/pgio",
                         "version": "v1.0.0",
                         "sum": "h1:g12B9UwVnzGhueNavwioyEEpAmqMe1E/BN9ES+8ovkE="
                    },
                    "output_tree_hash": "75a5379f2e348d248ad2bed4cd52589c56af05dbdf61b71cf43ee7a4d0a6062b"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
          "definition_information": "Repository com_github_jackc_pgservicefile instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:75:16: in <toplevel>\n  /home/jcgregorio/goldmine/go_repositories.bzl:1390:18: in go_repositories\nRepository rule go_repository defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_gazelle/internal/go_repository.bzl:340:32: in <toplevel>\n",
          "original_attributes": {
               "name": "com_github_jackc_pgservicefile",
               "generator_name": "com_github_jackc_pgservicefile",
               "generator_function": "go_repositories",
               "generator_location": None,
               "importpath": "github.com/jackc/pgservicefile",
               "version": "v0.0.0-20221227161230-091c0ba34f0a",
               "sum": "h1:bbPeKD0xmW/Y25WS6cokEszi5g+S0QxI/d45PkRi7Nk="
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
                    "attributes": {
                         "name": "com_github_jackc_pgservicefile",
                         "generator_name": "com_github_jackc_pgservicefile",
                         "generator_function": "go_repositories",
                         "generator_location": None,
                         "importpath": "github.com/jackc/pgservicefile",
                         "version": "v0.0.0-20221227161230-091c0ba34f0a",
                         "sum": "h1:bbPeKD0xmW/Y25WS6cokEszi5g+S0QxI/d45PkRi7Nk="
                    },
                    "output_tree_hash": "d073c8146e0c716a917876340d27695f75fb388f3d1d51fa37b8119dcf6dc36f"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
          "definition_information": "Repository com_github_jackc_puddle instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:75:16: in <toplevel>\n  /home/jcgregorio/goldmine/go_repositories.bzl:1411:18: in go_repositories\nRepository rule go_repository defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_gazelle/internal/go_repository.bzl:340:32: in <toplevel>\n",
          "original_attributes": {
               "name": "com_github_jackc_puddle",
               "generator_name": "com_github_jackc_puddle",
               "generator_function": "go_repositories",
               "generator_location": None,
               "importpath": "github.com/jackc/puddle",
               "version": "v1.3.0",
               "sum": "h1:eHK/5clGOatcjX3oWGBO/MpxpbHzSwud5EWTSCI+MX0="
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
                    "attributes": {
                         "name": "com_github_jackc_puddle",
                         "generator_name": "com_github_jackc_puddle",
                         "generator_function": "go_repositories",
                         "generator_location": None,
                         "importpath": "github.com/jackc/puddle",
                         "version": "v1.3.0",
                         "sum": "h1:eHK/5clGOatcjX3oWGBO/MpxpbHzSwud5EWTSCI+MX0="
                    },
                    "output_tree_hash": "1d2a4a90e52d440e39a1e59310d48e7147799d1256f7b61ca653732faa87e5dd"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
          "definition_information": "Repository com_github_jackc_pgtype instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:75:16: in <toplevel>\n  /home/jcgregorio/goldmine/go_repositories.bzl:1397:18: in go_repositories\nRepository rule go_repository defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_gazelle/internal/go_repository.bzl:340:32: in <toplevel>\n",
          "original_attributes": {
               "name": "com_github_jackc_pgtype",
               "generator_name": "com_github_jackc_pgtype",
               "generator_function": "go_repositories",
               "generator_location": None,
               "importpath": "github.com/jackc/pgtype",
               "version": "v1.14.0",
               "sum": "h1:y+xUdabmyMkJLyApYuPj38mW+aAIqCe5uuBB51rH3Vw="
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
                    "attributes": {
                         "name": "com_github_jackc_pgtype",
                         "generator_name": "com_github_jackc_pgtype",
                         "generator_function": "go_repositories",
                         "generator_location": None,
                         "importpath": "github.com/jackc/pgtype",
                         "version": "v1.14.0",
                         "sum": "h1:y+xUdabmyMkJLyApYuPj38mW+aAIqCe5uuBB51rH3Vw="
                    },
                    "output_tree_hash": "bbfd98728419e2450edf409b8c0e8e71f28a6725e1abfddb8e27566470c22fd8"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
          "definition_information": "Repository com_github_jackc_pgproto3_v2 instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:75:16: in <toplevel>\n  /home/jcgregorio/goldmine/go_repositories.bzl:1384:18: in go_repositories\nRepository rule go_repository defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_gazelle/internal/go_repository.bzl:340:32: in <toplevel>\n",
          "original_attributes": {
               "name": "com_github_jackc_pgproto3_v2",
               "generator_name": "com_github_jackc_pgproto3_v2",
               "generator_function": "go_repositories",
               "generator_location": None,
               "importpath": "github.com/jackc/pgproto3/v2",
               "version": "v2.3.2",
               "sum": "h1:7eY55bdBeCz1F2fTzSz69QC+pG46jYq9/jtSPiJ5nn0="
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
                    "attributes": {
                         "name": "com_github_jackc_pgproto3_v2",
                         "generator_name": "com_github_jackc_pgproto3_v2",
                         "generator_function": "go_repositories",
                         "generator_location": None,
                         "importpath": "github.com/jackc/pgproto3/v2",
                         "version": "v2.3.2",
                         "sum": "h1:7eY55bdBeCz1F2fTzSz69QC+pG46jYq9/jtSPiJ5nn0="
                    },
                    "output_tree_hash": "260e77225577174c978559d9de0c8bab168877edcbf5acd52916d3549e248ad3"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
          "definition_information": "Repository org_golang_x_crypto instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:75:16: in <toplevel>\n  /home/jcgregorio/goldmine/go_repositories.bzl:3990:18: in go_repositories\nRepository rule go_repository defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_gazelle/internal/go_repository.bzl:340:32: in <toplevel>\n",
          "original_attributes": {
               "name": "org_golang_x_crypto",
               "generator_name": "org_golang_x_crypto",
               "generator_function": "go_repositories",
               "generator_location": None,
               "importpath": "golang.org/x/crypto",
               "version": "v0.17.0",
               "sum": "h1:r8bRNjWL3GshPW3gkd+RpvzWrZAwPS49OmTGZ/uhM4k="
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
                    "attributes": {
                         "name": "org_golang_x_crypto",
                         "generator_name": "org_golang_x_crypto",
                         "generator_function": "go_repositories",
                         "generator_location": None,
                         "importpath": "golang.org/x/crypto",
                         "version": "v0.17.0",
                         "sum": "h1:r8bRNjWL3GshPW3gkd+RpvzWrZAwPS49OmTGZ/uhM4k="
                    },
                    "output_tree_hash": "2338e221cc9eadc5eb851facd135241874bb307103ee6896be859eb0278231aa"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
          "definition_information": "Repository org_golang_x_text instantiated at:\n  /home/jcgregorio/goldmine/WORKSPACE:75:16: in <toplevel>\n  /home/jcgregorio/goldmine/go_repositories.bzl:4066:18: in go_repositories\nRepository rule go_repository defined at:\n  /home/jcgregorio/.cache/bazel/_bazel_jcgregorio/385682bf6f095f8585d188113d285ba8/external/bazel_gazelle/internal/go_repository.bzl:340:32: in <toplevel>\n",
          "original_attributes": {
               "name": "org_golang_x_text",
               "generator_name": "org_golang_x_text",
               "generator_function": "go_repositories",
               "generator_location": None,
               "importpath": "golang.org/x/text",
               "version": "v0.14.0",
               "sum": "h1:ScX5w1eTa3QqT8oi6+ziP7dTV1S2+ALU0bI+0zXKWiQ="
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
                    "attributes": {
                         "name": "org_golang_x_text",
                         "generator_name": "org_golang_x_text",
                         "generator_function": "go_repositories",
                         "generator_location": None,
                         "importpath": "golang.org/x/text",
                         "version": "v0.14.0",
                         "sum": "h1:ScX5w1eTa3QqT8oi6+ziP7dTV1S2+ALU0bI+0zXKWiQ="
                    },
                    "output_tree_hash": "06aca2c88b4219c7f8ae41a1f8f18d43e8615bbf2c41bc4d93247cc59c392cf4"
               }
          ]
     }
]