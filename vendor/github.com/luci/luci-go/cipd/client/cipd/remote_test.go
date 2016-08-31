// Copyright 2014 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package cipd

import (
	"net/url"
	"strings"
	"testing"
	"time"

	. "github.com/luci/luci-go/cipd/client/cipd/common"
	. "github.com/smartystreets/goconvey/convey"
)

func TestRemoteImpl(t *testing.T) {
	ctx := makeTestContext()

	mockInitiateUpload := func(c C, reply string) (*UploadSession, error) {
		remote := mockRemoteImpl(c, []expectedHTTPCall{
			{
				Method: "POST",
				Path:   "/_ah/api/cas/v1/upload/SHA1/abc",
				Reply:  reply,
			},
		})
		return remote.initiateUpload(ctx, "abc")
	}

	mockFinalizeUpload := func(c C, reply string) (bool, error) {
		remote := mockRemoteImpl(c, []expectedHTTPCall{
			{
				Method: "POST",
				Path:   "/_ah/api/cas/v1/finalize/abc",
				Reply:  reply,
			},
		})
		return remote.finalizeUpload(ctx, "abc")
	}

	mockRegisterInstance := func(c C, reply string) (*registerInstanceResponse, error) {
		remote := mockRemoteImpl(c, []expectedHTTPCall{
			{
				Method: "POST",
				Path:   "/_ah/api/repo/v1/instance",
				Query: url.Values{
					"package_name": []string{"pkgname"},
					"instance_id":  []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
				},
				Reply: reply,
			},
		})
		return remote.registerInstance(ctx, Pin{"pkgname", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	}

	mockDeletePackage := func(c C, reply string) error {
		remote := mockRemoteImpl(c, []expectedHTTPCall{
			{
				Method: "DELETE",
				Path:   "/_ah/api/repo/v1/package",
				Query: url.Values{
					"package_name": []string{"pkgname"},
				},
				Reply: reply,
			},
		})
		return remote.deletePackage(ctx, "pkgname")
	}

	mockFetchInstance := func(c C, reply string) (*fetchInstanceResponse, error) {
		remote := mockRemoteImpl(c, []expectedHTTPCall{
			{
				Method: "GET",
				Path:   "/_ah/api/repo/v1/instance",
				Query: url.Values{
					"package_name": []string{"pkgname"},
					"instance_id":  []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
				},
				Reply: reply,
			},
		})
		return remote.fetchInstance(ctx, Pin{"pkgname", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	}

	mockFetchTags := func(c C, reply string, tags []string) ([]TagInfo, error) {
		query := url.Values{
			"package_name": []string{"pkgname"},
			"instance_id":  []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		}
		if len(tags) != 0 {
			query["tags"] = tags
		}
		remote := mockRemoteImpl(c, []expectedHTTPCall{
			{
				Method: "GET",
				Path:   "/_ah/api/repo/v1/tags",
				Query:  query,
				Reply:  reply,
			},
		})
		return remote.fetchTags(ctx, Pin{"pkgname", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, tags)
	}

	mockFetchRefs := func(c C, reply string, refs []string) ([]RefInfo, error) {
		query := url.Values{
			"package_name": []string{"pkgname"},
			"instance_id":  []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		}
		if len(refs) != 0 {
			query["refs"] = refs
		}
		remote := mockRemoteImpl(c, []expectedHTTPCall{
			{
				Method: "GET",
				Path:   "/_ah/api/repo/v1/ref",
				Query:  query,
				Reply:  reply,
			},
		})
		return remote.fetchRefs(ctx, Pin{"pkgname", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, refs)
	}

	mockFetchACL := func(c C, reply string) ([]PackageACL, error) {
		remote := mockRemoteImpl(c, []expectedHTTPCall{
			{
				Method: "GET",
				Path:   "/_ah/api/repo/v1/acl",
				Query:  url.Values{"package_path": []string{"pkgname"}},
				Reply:  reply,
			},
		})
		return remote.fetchACL(ctx, "pkgname")
	}

	mockModifyACL := func(c C, changes []PackageACLChange, body, reply string) error {
		remote := mockRemoteImpl(c, []expectedHTTPCall{
			{
				Method: "POST",
				Path:   "/_ah/api/repo/v1/acl",
				Query:  url.Values{"package_path": []string{"pkgname"}},
				Body:   body,
				Reply:  reply,
			},
		})
		return remote.modifyACL(ctx, "pkgname", changes)
	}

	mockSetRef := func(c C, reply string) error {
		remote := mockRemoteImpl(c, []expectedHTTPCall{
			{
				Method: "POST",
				Path:   "/_ah/api/repo/v1/ref",
				Query: url.Values{
					"package_name": []string{"pkgname"},
					"ref":          []string{"some-ref"},
				},
				Body:  `{"instance_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`,
				Reply: reply,
			},
		})
		return remote.setRef(ctx, "some-ref", Pin{"pkgname", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	}

	mockListPackages := func(c C, reply string) ([]string, []string, error) {
		remote := mockRemoteImpl(c, []expectedHTTPCall{
			{
				Method: "GET",
				Path:   "/_ah/api/repo/v1/package/search",
				Query: url.Values{
					"path":        []string{"pkgpath"},
					"recursive":   []string{"false"},
					"show_hidden": []string{"false"},
				},
				Reply: reply,
			},
		})
		return remote.listPackages(ctx, "pkgpath", false, false)
	}

	mockAttachTags := func(c C, tags []string, body, reply string) error {
		remote := mockRemoteImpl(c, []expectedHTTPCall{
			{
				Method: "POST",
				Path:   "/_ah/api/repo/v1/tags",
				Query: url.Values{
					"package_name": []string{"pkgname"},
					"instance_id":  []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
				},
				Body:  body,
				Reply: reply,
			},
		})
		return remote.attachTags(ctx, Pin{"pkgname", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, tags)
	}

	mockResolveVersion := func(c C, reply string) (Pin, error) {
		remote := mockRemoteImpl(c, []expectedHTTPCall{
			{
				Method: "GET",
				Path:   "/_ah/api/repo/v1/instance/resolve",
				Query: url.Values{
					"package_name": []string{"pkgname"},
					"version":      []string{"tag_key:value"},
				},
				Reply: reply,
			},
		})
		return remote.resolveVersion(ctx, "pkgname", "tag_key:value")
	}

	Convey("makeRequest POST works", t, func(c C) {
		remote := mockRemoteImpl(c, []expectedHTTPCall{
			{
				Method: "POST",
				Path:   "/_ah/api/cas/v1/method",
				Reply:  `{"value":"123"}`,
			},
		})
		var reply struct {
			Value string `json:"value"`
		}
		err := remote.makeRequest(ctx, "cas/v1/method", "POST", nil, &reply)
		So(err, ShouldBeNil)
		So(reply.Value, ShouldEqual, "123")
	})

	Convey("makeRequest GET works", t, func(c C) {
		remote := mockRemoteImpl(c, []expectedHTTPCall{
			{
				Method: "GET",
				Path:   "/_ah/api/cas/v1/method",
				Reply:  `{"value":"123"}`,
			},
		})
		var reply struct {
			Value string `json:"value"`
		}
		err := remote.makeRequest(ctx, "cas/v1/method", "GET", nil, &reply)
		So(err, ShouldBeNil)
		So(reply.Value, ShouldEqual, "123")
	})

	Convey("makeRequest handles fatal error", t, func(c C) {
		remote := mockRemoteImpl(c, []expectedHTTPCall{
			{
				Method: "POST",
				Path:   "/_ah/api/cas/v1/method",
				Status: 403,
			},
		})
		var reply struct{}
		err := remote.makeRequest(ctx, "cas/v1/method", "POST", nil, &reply)
		So(err, ShouldNotBeNil)
	})

	Convey("makeRequest handles retries", t, func(c C) {
		remote := mockRemoteImpl(c, []expectedHTTPCall{
			{
				Method: "POST",
				Path:   "/_ah/api/cas/v1/method",
				Status: 500,
			},
			{
				Method: "POST",
				Path:   "/_ah/api/cas/v1/method",
				Reply:  `{}`,
			},
		})
		var reply struct{}
		err := remote.makeRequest(ctx, "cas/v1/method", "POST", nil, &reply)
		So(err, ShouldBeNil)
	})

	Convey("makeRequest gives up trying", t, func(c C) {
		calls := []expectedHTTPCall{}
		for i := 0; i < remoteMaxRetries; i++ {
			calls = append(calls, expectedHTTPCall{
				Method: "POST",
				Path:   "/_ah/api/cas/v1/method",
				Status: 500,
			})
		}
		remote := mockRemoteImpl(c, calls)
		var reply struct{}
		err := remote.makeRequest(ctx, "cas/v1/method", "POST", nil, &reply)
		So(err, ShouldNotBeNil)
	})

	Convey("initiateUpload ALREADY_UPLOADED", t, func(c C) {
		s, err := mockInitiateUpload(c, `{"status":"ALREADY_UPLOADED"}`)
		So(err, ShouldBeNil)
		So(s, ShouldBeNil)
	})

	Convey("initiateUpload SUCCESS", t, func(c C) {
		s, err := mockInitiateUpload(c, `{"status":"SUCCESS","upload_session_id":"123","upload_url":"http://localhost"}`)
		So(err, ShouldBeNil)
		So(s, ShouldResemble, &UploadSession{"123", "http://localhost"})
	})

	Convey("initiateUpload ERROR", t, func(c C) {
		s, err := mockInitiateUpload(c, `{"status":"ERROR","error_message":"boo"}`)
		So(err, ShouldNotBeNil)
		So(s, ShouldBeNil)
	})

	Convey("initiateUpload unknown status", t, func(c C) {
		s, err := mockInitiateUpload(c, `{"status":"???"}`)
		So(err, ShouldNotBeNil)
		So(s, ShouldBeNil)
	})

	Convey("initiateUpload bad reply", t, func(c C) {
		remote := mockRemoteImpl(c, []expectedHTTPCall{
			{
				Method: "POST",
				Path:   "/_ah/api/cas/v1/upload/SHA1/abc",
				Status: 403,
			},
		})
		s, err := remote.initiateUpload(ctx, "abc")
		So(err, ShouldNotBeNil)
		So(s, ShouldBeNil)
	})

	Convey("finalizeUpload MISSING", t, func(c C) {
		finished, err := mockFinalizeUpload(c, `{"status":"MISSING"}`)
		So(err, ShouldNotBeNil)
		So(finished, ShouldBeFalse)
	})

	Convey("finalizeUpload UPLOADING", t, func(c C) {
		finished, err := mockFinalizeUpload(c, `{"status":"UPLOADING"}`)
		So(err, ShouldBeNil)
		So(finished, ShouldBeFalse)
	})

	Convey("finalizeUpload VERIFYING", t, func(c C) {
		finished, err := mockFinalizeUpload(c, `{"status":"VERIFYING"}`)
		So(err, ShouldBeNil)
		So(finished, ShouldBeFalse)
	})

	Convey("finalizeUpload PUBLISHED", t, func(c C) {
		finished, err := mockFinalizeUpload(c, `{"status":"PUBLISHED"}`)
		So(err, ShouldBeNil)
		So(finished, ShouldBeTrue)
	})

	Convey("finalizeUpload ERROR", t, func(c C) {
		finished, err := mockFinalizeUpload(c, `{"status":"ERROR","error_message":"boo"}`)
		So(err, ShouldNotBeNil)
		So(finished, ShouldBeFalse)
	})

	Convey("finalizeUpload unknown status", t, func(c C) {
		finished, err := mockFinalizeUpload(c, `{"status":"???"}`)
		So(err, ShouldNotBeNil)
		So(finished, ShouldBeFalse)
	})

	Convey("finalizeUpload bad reply", t, func(c C) {
		remote := mockRemoteImpl(c, []expectedHTTPCall{
			{
				Method: "POST",
				Path:   "/_ah/api/cas/v1/finalize/abc",
				Status: 403,
			},
		})
		finished, err := remote.finalizeUpload(ctx, "abc")
		So(err, ShouldNotBeNil)
		So(finished, ShouldBeFalse)
	})

	Convey("registerInstance REGISTERED", t, func(c C) {
		result, err := mockRegisterInstance(c, `{
				"status": "REGISTERED",
				"instance": {
					"registered_by": "user:abc@example.com",
					"registered_ts": "1420244414571500"
				}
			}`)
		So(err, ShouldBeNil)
		So(result, ShouldResemble, &registerInstanceResponse{
			registeredBy: "user:abc@example.com",
			registeredTs: time.Unix(0, 1420244414571500000),
		})
	})

	Convey("registerInstance ALREADY_REGISTERED", t, func(c C) {
		result, err := mockRegisterInstance(c, `{
				"status": "ALREADY_REGISTERED",
				"instance": {
					"registered_by": "user:abc@example.com",
					"registered_ts": "1420244414571500"
				}
			}`)
		So(err, ShouldBeNil)
		So(result, ShouldResemble, &registerInstanceResponse{
			alreadyRegistered: true,
			registeredBy:      "user:abc@example.com",
			registeredTs:      time.Unix(0, 1420244414571500000),
		})
	})

	Convey("registerInstance UPLOAD_FIRST", t, func(c C) {
		result, err := mockRegisterInstance(c, `{
				"status": "UPLOAD_FIRST",
				"upload_session_id": "upload_session_id",
				"upload_url": "http://upload_url"
			}`)
		So(err, ShouldBeNil)
		So(result, ShouldResemble, &registerInstanceResponse{
			uploadSession: &UploadSession{"upload_session_id", "http://upload_url"},
		})
	})

	Convey("registerInstance ERROR", t, func(c C) {
		result, err := mockRegisterInstance(c, `{
				"status": "ERROR",
				"error_message": "Some error message"
			}`)
		So(err, ShouldNotBeNil)
		So(result, ShouldBeNil)
	})

	Convey("registerInstance unknown status", t, func(c C) {
		result, err := mockRegisterInstance(c, `{"status":"???"}`)
		So(err, ShouldNotBeNil)
		So(result, ShouldBeNil)
	})

	Convey("deletePackage SUCCESS", t, func(c C) {
		So(mockDeletePackage(c, `{"status": "SUCCESS"}`), ShouldBeNil)
	})

	Convey("deletePackage PACKAGE_NOT_FOUND", t, func(c C) {
		err := mockDeletePackage(c, `{"status": "PACKAGE_NOT_FOUND"}`)
		So(err, ShouldEqual, ErrPackageNotFound)
	})

	Convey("deletePackage ERROR", t, func(c C) {
		err := mockDeletePackage(c, `{
				"status": "ERROR",
				"error_message": "Some error message"
			}`)
		So(err, ShouldNotBeNil)
	})

	Convey("deletePackage unknown status", t, func(c C) {
		err := mockDeletePackage(c, `{"status":"???"}`)
		So(err, ShouldNotBeNil)
	})

	Convey("fetchInstance SUCCESS", t, func(c C) {
		result, err := mockFetchInstance(c, `{
				"status": "SUCCESS",
				"instance": {
					"registered_by": "user:abc@example.com",
					"registered_ts": "1420244414571500"
				},
				"fetch_url": "https://fetch_url"
			}`)
		So(err, ShouldBeNil)
		So(result, ShouldResemble, &fetchInstanceResponse{
			registeredBy: "user:abc@example.com",
			registeredTs: time.Unix(0, 1420244414571500000),
			fetchURL:     "https://fetch_url",
		})
	})

	Convey("fetchInstance PACKAGE_NOT_FOUND", t, func(c C) {
		result, err := mockFetchInstance(c, `{"status": "PACKAGE_NOT_FOUND"}`)
		So(err, ShouldNotBeNil)
		So(result, ShouldBeNil)
	})

	Convey("fetchInstance INSTANCE_NOT_FOUND", t, func(c C) {
		result, err := mockFetchInstance(c, `{"status": "INSTANCE_NOT_FOUND"}`)
		So(err, ShouldNotBeNil)
		So(result, ShouldBeNil)
	})

	Convey("fetchInstance ERROR", t, func(c C) {
		result, err := mockFetchInstance(c, `{
				"status": "ERROR",
				"error_message": "Some error message"
			}`)
		So(err, ShouldNotBeNil)
		So(result, ShouldBeNil)
	})

	Convey("fetchTags SUCCESS", t, func(c C) {
		result, err := mockFetchTags(c, `{
				"status": "SUCCESS",
				"tags": [
					{
						"tag": "a:b1",
						"registered_by": "user:a@example.com",
						"registered_ts": "1420244414571500"
					},
					{
						"tag": "a:b2",
						"registered_by": "user:a@example.com",
						"registered_ts": "bad-timestamp"
					}
				]
			}`, nil)
		So(err, ShouldBeNil)
		So(result, ShouldResemble, []TagInfo{
			{
				Tag:          "a:b1",
				RegisteredBy: "user:a@example.com",
				RegisteredTs: UnixTime(time.Unix(0, 1420244414571500000)),
			},
			{
				Tag:          "a:b2",
				RegisteredBy: "user:a@example.com",
				RegisteredTs: UnixTime{},
			},
		})
	})

	Convey("fetchTags PACKAGE_NOT_FOUND", t, func(c C) {
		result, err := mockFetchTags(c, `{"status": "PACKAGE_NOT_FOUND"}`, nil)
		So(err, ShouldNotBeNil)
		So(result, ShouldBeNil)
	})

	Convey("fetchTags INSTANCE_NOT_FOUND", t, func(c C) {
		result, err := mockFetchTags(c, `{"status": "INSTANCE_NOT_FOUND"}`, nil)
		So(err, ShouldNotBeNil)
		So(result, ShouldBeNil)
	})

	Convey("fetchTags ERROR", t, func(c C) {
		result, err := mockFetchTags(c, `{
				"status": "ERROR",
				"error_message": "Some error message"
			}`, nil)
		So(err, ShouldNotBeNil)
		So(result, ShouldBeNil)
	})

	Convey("fetchRefs SUCCESS", t, func(c C) {
		result, err := mockFetchRefs(c, `{
				"status": "SUCCESS",
				"refs": [
					{
						"ref": "ref1",
						"modified_by": "user:a@example.com",
						"modified_ts": "1420244414571500"
					},
					{
						"ref": "ref2",
						"modified_by": "user:a@example.com",
						"modified_ts": "bad-timestamp"
					}
				]
			}`, nil)
		So(err, ShouldBeNil)
		So(result, ShouldResemble, []RefInfo{
			{
				Ref:        "ref1",
				ModifiedBy: "user:a@example.com",
				ModifiedTs: UnixTime(time.Unix(0, 1420244414571500000)),
			},
			{
				Ref:        "ref2",
				ModifiedBy: "user:a@example.com",
				ModifiedTs: UnixTime{},
			},
		})
	})

	Convey("fetchRefs PACKAGE_NOT_FOUND", t, func(c C) {
		result, err := mockFetchRefs(c, `{"status": "PACKAGE_NOT_FOUND"}`, nil)
		So(err, ShouldNotBeNil)
		So(result, ShouldBeNil)
	})

	Convey("fetchRefs INSTANCE_NOT_FOUND", t, func(c C) {
		result, err := mockFetchRefs(c, `{"status": "INSTANCE_NOT_FOUND"}`, nil)
		So(err, ShouldNotBeNil)
		So(result, ShouldBeNil)
	})

	Convey("fetchRefs ERROR", t, func(c C) {
		result, err := mockFetchRefs(c, `{
				"status": "ERROR",
				"error_message": "Some error message"
			}`, nil)
		So(err, ShouldNotBeNil)
		So(result, ShouldBeNil)
	})

	Convey("fetchACL SUCCESS", t, func(c C) {
		result, err := mockFetchACL(c, `{
				"status": "SUCCESS",
				"acls": {
					"acls": [
						{
							"package_path": "a",
							"role": "OWNER",
							"principals": ["user:a", "group:b"],
							"modified_by": "user:abc@example.com",
							"modified_ts": "1420244414571500"
						},
						{
							"package_path": "a/b",
							"role": "READER",
							"principals": ["group:c"],
							"modified_by": "user:abc@example.com",
							"modified_ts": "1420244414571500"
						}
					]
				}
			}`)
		So(err, ShouldBeNil)
		So(result, ShouldResemble, []PackageACL{
			{
				PackagePath: "a",
				Role:        "OWNER",
				Principals:  []string{"user:a", "group:b"},
				ModifiedBy:  "user:abc@example.com",
				ModifiedTs:  UnixTime(time.Unix(0, 1420244414571500000)),
			},
			{
				PackagePath: "a/b",
				Role:        "READER",
				Principals:  []string{"group:c"},
				ModifiedBy:  "user:abc@example.com",
				ModifiedTs:  UnixTime(time.Unix(0, 1420244414571500000)),
			},
		})
	})

	Convey("fetchACL ERROR", t, func(c C) {
		result, err := mockFetchACL(c, `{
				"status": "ERROR",
				"error_message": "Some error message"
			}`)
		So(err, ShouldNotBeNil)
		So(result, ShouldBeNil)
	})

	Convey("modifyACL SUCCESS", t, func(c C) {
		expected := `{
				"changes": [
					{
						"action": "GRANT",
						"role": "OWNER",
						"principal": "user:a@example.com"
					},
					{
						"action": "REVOKE",
						"role": "READER",
						"principal": "user:b@example.com"
					}
				]
			}`
		// Strip " ", "\t" and "\n".
		expected = strings.Replace(expected, " ", "", -1)
		expected = strings.Replace(expected, "\n", "", -1)
		expected = strings.Replace(expected, "\t", "", -1)

		err := mockModifyACL(c, []PackageACLChange{
			{
				Action:    GrantRole,
				Role:      "OWNER",
				Principal: "user:a@example.com",
			},
			{
				Action:    RevokeRole,
				Role:      "READER",
				Principal: "user:b@example.com",
			},
		}, expected, `{"status":"SUCCESS"}`)
		So(err, ShouldBeNil)
	})

	Convey("modifyACL ERROR", t, func(c C) {
		err := mockModifyACL(c, []PackageACLChange{}, `{"changes":null}`, `{
				"status": "ERROR",
				"error_message": "Error message"
			}`)
		So(err, ShouldNotBeNil)
	})

	Convey("setRef SUCCESS", t, func(c C) {
		So(mockSetRef(c, `{"status":"SUCCESS"}`), ShouldBeNil)
	})

	Convey("setRef bad ref", t, func(c C) {
		err := mockRemoteImpl(c, nil).setRef(
			ctx, "BAD REF",
			Pin{"pkgname", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
		So(err, ShouldNotBeNil)
	})

	Convey("setRef PROCESSING_NOT_FINISHED_YET", t, func(c C) {
		err := mockSetRef(c, `{"status":"PROCESSING_NOT_FINISHED_YET", "error_message":"Blah"}`)
		So(err, ShouldResemble, &pendingProcessingError{message: "Blah"})
	})

	Convey("setRef ERROR", t, func(c C) {
		So(mockSetRef(c, `{"status":"ERROR", "error_message":"Blah"}`), ShouldNotBeNil)
	})

	Convey("listPackages SUCCESS", t, func(c C) {
		pkgs, dirs, err := mockListPackages(c, `{
				"status": "SUCCESS",
				"packages": [
					"pkgpath/fake1",
					"pkgpath/fake2"
				],
				"directories": []
			}`)
		So(err, ShouldBeNil)
		So(pkgs, ShouldResemble, []string{
			"pkgpath/fake1",
			"pkgpath/fake2",
		})
		So(dirs, ShouldResemble, []string{})
	})

	Convey("listPackages ERROR", t, func(c C) {
		pkgs, dirs, err := mockListPackages(c, `{
				"status": "ERROR",
				"error_message": "Some error message"
			}`)
		So(err, ShouldNotBeNil)
		So(pkgs, ShouldBeNil)
		So(dirs, ShouldBeNil)
	})

	Convey("attachTags SUCCESS", t, func(c C) {
		err := mockAttachTags(
			c, []string{"tag1:value1", "tag2:value2"},
			`{"tags":["tag1:value1","tag2:value2"]}`,
			`{"status":"SUCCESS"}`)
		So(err, ShouldBeNil)
	})

	Convey("attachTags bad tag", t, func(c C) {
		err := mockRemoteImpl(c, nil).attachTags(
			ctx, Pin{"pkgname", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
			[]string{"BADTAG"})
		So(err, ShouldNotBeNil)
	})

	Convey("attachTags PROCESSING_NOT_FINISHED_YET", t, func(c C) {
		err := mockAttachTags(
			c, []string{"tag1:value1", "tag2:value2"},
			`{"tags":["tag1:value1","tag2:value2"]}`,
			`{"status":"PROCESSING_NOT_FINISHED_YET", "error_message":"Blah"}`)
		So(err, ShouldResemble, &pendingProcessingError{message: "Blah"})
	})

	Convey("attachTags ERROR", t, func(c C) {
		err := mockAttachTags(
			c, []string{"tag1:value1", "tag2:value2"},
			`{"tags":["tag1:value1","tag2:value2"]}`,
			`{"status":"ERROR", "error_message":"Blah"}`)
		So(err, ShouldNotBeNil)
	})

	Convey("resolveVersion SUCCESS", t, func(c C) {
		pin, err := mockResolveVersion(c, `{
			"status": "SUCCESS",
			"instance_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}`)
		So(err, ShouldBeNil)
		So(pin, ShouldResemble, Pin{"pkgname", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	})

	Convey("resolveVersion SUCCESS and bad instance ID", t, func(c C) {
		_, err := mockResolveVersion(c, `{
			"status": "SUCCESS",
			"instance_id": "bad_id"
		}`)
		So(err, ShouldNotBeNil)
	})

	Convey("resolveVersion PACKAGE_NOT_FOUND", t, func(c C) {
		_, err := mockResolveVersion(c, `{"status": "PACKAGE_NOT_FOUND"}`)
		So(err, ShouldNotBeNil)
	})

	Convey("resolveVersion INSTANCE_NOT_FOUND", t, func(c C) {
		_, err := mockResolveVersion(c, `{"status": "INSTANCE_NOT_FOUND"}`)
		So(err, ShouldNotBeNil)
	})

	Convey("resolveVersion AMBIGUOUS_VERSION", t, func(c C) {
		_, err := mockResolveVersion(c, `{"status": "AMBIGUOUS_VERSION"}`)
		So(err, ShouldNotBeNil)
	})

	Convey("resolveVersion ERROR", t, func(c C) {
		_, err := mockResolveVersion(c, `{"status": "ERROR", "error_message":"Blah"}`)
		So(err, ShouldNotBeNil)
	})

	Convey("resolveVersion bad status", t, func(c C) {
		_, err := mockResolveVersion(c, `{"status": "HUH?"}`)
		So(err, ShouldNotBeNil)
	})
}

////////////////////////////////////////////////////////////////////////////////

func mockRemoteImpl(c C, expectations []expectedHTTPCall) *remoteImpl {
	client := mockClient(c, "", expectations)
	return &remoteImpl{
		serviceURL: client.ServiceURL,
		userAgent:  client.UserAgent,
		client:     client.AnonymousClient,
	}
}
