package docker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/mockhttpclient"
)

const (
	fakeRegistry   = "fake-registry"
	fakeRepository = "my-image"
	fakeTag        = "latest"

	getManifestResponse = `{
   "schemaVersion": 2,
   "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
   "config": {
      "mediaType": "application/vnd.docker.container.image.v1+json",
      "size": 3316,
      "digest": "sha256:413668c8e4f58c8c979f9f8c4e3fbc9a5447149cc7f3a7e345b6d67a0615c5d6"
   },
   "layers": [
      {
         "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
         "size": 27811285,
         "digest": "sha256:82e01e20eb1d1a8e24c1a2568fdd977d3748c4e0d2a35d08853a0828f1282cb6"
      },
      {
         "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
         "size": 15056296,
         "digest": "sha256:9b108e8a453a70d884144cba59cc1bd4bf60e38ddf7aae08e71c5447611c0927"
      },
      {
         "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
         "size": 194179534,
         "digest": "sha256:386084a6509c415a2a231cd627ff75ea7b55650509d983d213e54ddc3df295cd"
      },
      {
         "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
         "size": 16449088,
         "digest": "sha256:118cf07964a5e185090d3b6997100c9917318a6c38790ffc5a509598d31c9ef6"
      },
      {
         "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
         "size": 404878875,
         "digest": "sha256:d19501fed31b581d3025ebe52e59ff8d16025b7e770b8749c1af9127298045bb"
      },
      {
         "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
         "size": 49,
         "digest": "sha256:3c2cba919283a210665e480bcbf943eaaf4ed87a83f02e81bb286b8bdead0e75"
      }
   ]
}`
	getConfigResponse = `{
  "architecture": "amd64",
  "author": "Bazel",
  "config": {
    "AttachStderr": true,
    "AttachStdout": true,
    "Cmd": [
      "bash"
    ],
    "Entrypoint": [
      ""
    ],
    "Env": [
      "CIPD_ROOT=/cipd",
      "PATH=/cipd/go/bin:/cipd/go/bin:/cipd:/cipd/cipd_bin_packages:/cipd/cipd_bin_packages/bin:/cipd/cipd_bin_packages/cpython:/cipd/cipd_bin_packages/cpython/bin:/cipd/cipd_bin_packages/cpython3:/cipd/cipd_bin_packages/cpython3/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
    ],
    "Hostname": "100bdce963b2",
    "Image": "30079d3d793b32475f48a8559425d35f35536c646ebefbeb93b890ea6e183562",
    "User": "skia"
  },
  "container": "100bdce963b25eb74e074ef49f3fda76d68f75884c3669c933db7318a545b4df",
  "created": "1970-01-01T00:00:00Z",
  "docker_version": "20.10.23",
  "history": [
    {
      "author": "Bazel",
      "created": "1970-01-01T00:00:00Z",
      "created_by": "bazel build ..."
    },
    {
      "author": "Bazel",
      "created": "1970-01-01T00:00:00Z",
      "created_by": "bazel build ..."
    },
    {
      "created": "2020-08-04T15:46:23.612778871Z",
      "created_by": "/bin/sh -c #(nop) ADD file:dee7987945ea8b67359e7fa5814228ccd127cbc5c9ebdd608c89f89a7fb6d2d0 in / "
    },
    {
      "created": "2020-08-04T15:46:23.859725865Z",
      "created_by": "/bin/sh -c #(nop)  CMD [\"bash\"]",
      "empty_layer": true
    },
    {
      "created": "2020-08-24T13:44:22.205739623Z",
      "created_by": "/bin/sh -c apt-get update && apt-get upgrade -y && apt-get install -y    ca-certificates   && rm -rf /var/lib/apt/lists/*   && addgroup --gid 2000 skia   && adduser --uid 2000 --gid 2000 skia"
    },
    {
      "created": "2020-08-24T13:44:22.574114965Z",
      "created_by": "/bin/sh -c #(nop)  USER skia:skia",
      "empty_layer": true
    },
    {
      "created": "2020-08-24T13:45:24.072952145Z",
      "created_by": "/bin/sh -c #(nop)  ARG CIPD_ROOT",
      "empty_layer": true
    },
    {
      "created": "2020-08-24T13:45:24.256039559Z",
      "created_by": "/bin/sh -c #(nop)  ENV CIPD_ROOT=/cipd",
      "empty_layer": true
    },
    {
      "created": "2021-09-27T13:57:56.814739607Z",
      "created_by": "/bin/sh -c #(nop) COPY dir:f55d8860fc73e9ab333e1bf01c0864ddcc415e81086e9294e263d9fb06744040 in /cipd "
    },
    {
      "created": "2021-09-27T13:58:00.314301887Z",
      "created_by": "/bin/sh -c #(nop)  ENV PATH=/cipd:/cipd/cipd_bin_packages:/cipd/cipd_bin_packages/bin:/cipd/cipd_bin_packages/cpython:/cipd/cipd_bin_packages/cpython/bin:/cipd/cipd_bin_packages/cpython3:/cipd/cipd_bin_packages/cpython3/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
      "empty_layer": true
    },
    {
      "created": "2023-04-25T04:33:13.177574832Z",
      "created_by": "sh -c apt-get update && apt-get install -y wget openssh-client curl procps unzip vim less build-essential g++ g++-11 gcc gcc-11 gcc-11-base cpp cpp-11 libgcc-11-dev libstdc++-11-dev cmake && wget --output-document=/usr/local/bin/bazelisk https://github.com/bazelbuild/bazelisk/releases/download/v1.14.0/bazelisk-linux-amd64 && chmod a+x /usr/local/bin/bazelisk && cipd install skia/bots/go --root=/cipd"
    }
  ],
  "os": "linux",
  "rootfs": {
    "diff_ids": [
      "sha256:2760827628c28b78cd38417202a35f4ebfb74fa15d8d8d29486efe2f9e7d8ef7",
      "sha256:ac98c2b90fe0c941922d78822d9db0ef473792027a6a623047073d0d67dc097c",
      "sha256:44177851d94bc2e39573ffacb0e8eee5fb14c016a99f21eed076849f4909d41b",
      "sha256:b122b9c4c2b6de00cf3bd051500faa0ddb81d9b147693b7bf3556e6d3095366c",
      "sha256:403a461f89b41095e2b047b63864a02f6a74e26708b9bca48fab41b4ef39de28",
      "sha256:84ff92691f909a05b224e1c56abb4864f01b4f8e3c854e4bb4c7baf1d3f6d652"
    ],
    "type": "layers"
  }
}
`
	listTagsResponse = `
{
  "child": [],
  "manifest": {
    "sha256:000ba24df84b6490d68069cdee599d6599f3891f6420a37cdaa65852c9f1ecbc": {
      "imageSizeBytes": "485929608",
      "layerId": "",
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "tag": [
        "2022-06-22T19_11_03Z-louhi-7d6a680-clean"
      ],
      "timeCreatedMs": "0",
      "timeUploadedMs": "1655925303330"
    },
    "sha256:001f6469f9513cda89206710c1711bb3f2cc169d7613d9d6bf688a7109eeeb30": {
      "imageSizeBytes": "654679661",
      "layerId": "",
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "tag": [
        "2023-03-27T13_22_20Z-louhi-378431d-clean"
      ],
      "timeCreatedMs": "0",
      "timeUploadedMs": "1679924189163"
    }
  },
  "name": "skia-public/autoroll-be",
  "tags": [
    "2022-06-22T19_11_03Z-louhi-7d6a680-clean",
    "2023-03-27T13_22_20Z-louhi-378431d-clean"
  ]
}
`

	listRepositoriesResponse1 = `{
  "next": "https://gcr.io/v2/_catalog?n=100&last=skia-public/img3",
  "repositories": [
    "skia-public/img1",
    "skia-public/img2",
    "skia-public/img3"
  ]
}`
	listRepositoriesResponse2 = `{
	"next": "",
	"repositories": [
	  "skia-public/img4",
	  "other/img5",
	  "other/img6"
	]
  }`

	fakeDigest = "000ba24df84b6490d68069cdee599d6599f3891f6420a37cdaa65852c9f1ecbc"
)

func TestGetManifest(t *testing.T) {
	ctx := context.Background()
	md := mockhttpclient.MockGetDialogue([]byte(getManifestResponse))
	md.RequestHeader(acceptHeader, acceptContentType)
	md.ResponseHeader(digestHeader, fakeDigest)
	urlmock := mockhttpclient.NewURLMock()
	fakeURL := fmt.Sprintf(manifestURLTemplate, fakeRegistry, fakeRepository, fakeTag)
	urlmock.MockOnce(fakeURL, md)
	client := NewClient(ctx, urlmock.Client(), fakeRegistry)

	manifest, err := client.GetManifest(ctx, fakeRepository, fakeTag)
	require.NoError(t, err)
	require.Equal(t, &Manifest{
		Digest: "000ba24df84b6490d68069cdee599d6599f3891f6420a37cdaa65852c9f1ecbc",
		Config: MediaConfig{
			MediaType: "application/vnd.docker.container.image.v1+json",
			Size:      3316,
			Digest:    "sha256:413668c8e4f58c8c979f9f8c4e3fbc9a5447149cc7f3a7e345b6d67a0615c5d6",
		},
		Layers: []MediaConfig{
			{
				MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
				Size:      27811285,
				Digest:    "sha256:82e01e20eb1d1a8e24c1a2568fdd977d3748c4e0d2a35d08853a0828f1282cb6",
			},
			{
				MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
				Size:      15056296,
				Digest:    "sha256:9b108e8a453a70d884144cba59cc1bd4bf60e38ddf7aae08e71c5447611c0927",
			},
			{
				MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
				Size:      194179534,
				Digest:    "sha256:386084a6509c415a2a231cd627ff75ea7b55650509d983d213e54ddc3df295cd",
			},
			{
				MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
				Size:      16449088,
				Digest:    "sha256:118cf07964a5e185090d3b6997100c9917318a6c38790ffc5a509598d31c9ef6",
			},
			{
				MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
				Size:      404878875,
				Digest:    "sha256:d19501fed31b581d3025ebe52e59ff8d16025b7e770b8749c1af9127298045bb",
			},
			{
				MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
				Size:      49,
				Digest:    "sha256:3c2cba919283a210665e480bcbf943eaaf4ed87a83f02e81bb286b8bdead0e75",
			},
		},
	}, manifest)
}

func TestGetDigest(t *testing.T) {
	ctx := context.Background()
	md := mockhttpclient.MockGetDialogue([]byte(getManifestResponse))
	md.RequestHeader(acceptHeader, acceptContentType)
	md.ResponseHeader(digestHeader, fakeDigest)
	urlmock := mockhttpclient.NewURLMock()
	fakeURL := fmt.Sprintf(manifestURLTemplate, fakeRegistry, fakeRepository, fakeTag)
	urlmock.MockOnce(fakeURL, md)
	client := NewClient(ctx, urlmock.Client(), fakeRegistry)

	digest, err := GetDigest(ctx, client, fakeRepository, fakeTag)
	require.NoError(t, err)
	require.Equal(t, fakeDigest, digest)
}

func TestGetConfig(t *testing.T) {
	ctx := context.Background()
	md := mockhttpclient.MockGetDialogue([]byte(getConfigResponse))
	urlmock := mockhttpclient.NewURLMock()
	// Repo digest and config digest are different, but for the purpose of this
	// test we don't care.
	configDigest := fakeDigest
	fakeURL := fmt.Sprintf(blobURLTemplate, fakeRegistry, fakeRepository, configDigest)
	urlmock.MockOnce(fakeURL, md)
	client := NewClient(ctx, urlmock.Client(), fakeRegistry)

	config, err := client.GetConfig(ctx, fakeRepository, configDigest)
	require.NoError(t, err)
	require.Equal(t, &ImageConfig{
		Architecture: "amd64",
		Author:       "Bazel",
		Config: ImageConfig_Config{
			AttachStderr: true,
			AttachStdout: true,
			Cmd:          []string{"bash"},
			Entrypoint:   []string{""},
			Env: []string{
				"CIPD_ROOT=/cipd",
				"PATH=/cipd/go/bin:/cipd/go/bin:/cipd:/cipd/cipd_bin_packages:/cipd/cipd_bin_packages/bin:/cipd/cipd_bin_packages/cpython:/cipd/cipd_bin_packages/cpython/bin:/cipd/cipd_bin_packages/cpython3:/cipd/cipd_bin_packages/cpython3/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			},
			Hostname: "100bdce963b2",
			Image:    "30079d3d793b32475f48a8559425d35f35536c646ebefbeb93b890ea6e183562",
			User:     "skia",
		},
		Container:     "100bdce963b25eb74e074ef49f3fda76d68f75884c3669c933db7318a545b4df",
		Created:       time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC),
		DockerVersion: "20.10.23",
		History: []ImageConfig_History{
			{Author: "Bazel", Created: time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC), CreatedBy: "bazel build ...", EmptyLayer: false},
			{Author: "Bazel", Created: time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC), CreatedBy: "bazel build ...", EmptyLayer: false},
			{Author: "", Created: time.Date(2020, time.August, 4, 15, 46, 23, 612778871, time.UTC), CreatedBy: "/bin/sh -c #(nop) ADD file:dee7987945ea8b67359e7fa5814228ccd127cbc5c9ebdd608c89f89a7fb6d2d0 in / ", EmptyLayer: false},
			{Author: "", Created: time.Date(2020, time.August, 4, 15, 46, 23, 859725865, time.UTC), CreatedBy: "/bin/sh -c #(nop)  CMD [\"bash\"]", EmptyLayer: true},
			{Author: "", Created: time.Date(2020, time.August, 24, 13, 44, 22, 205739623, time.UTC), CreatedBy: "/bin/sh -c apt-get update && apt-get upgrade -y && apt-get install -y    ca-certificates   && rm -rf /var/lib/apt/lists/*   && addgroup --gid 2000 skia   && adduser --uid 2000 --gid 2000 skia", EmptyLayer: false},
			{Author: "", Created: time.Date(2020, time.August, 24, 13, 44, 22, 574114965, time.UTC), CreatedBy: "/bin/sh -c #(nop)  USER skia:skia", EmptyLayer: true},
			{Author: "", Created: time.Date(2020, time.August, 24, 13, 45, 24, 72952145, time.UTC), CreatedBy: "/bin/sh -c #(nop)  ARG CIPD_ROOT", EmptyLayer: true},
			{Author: "", Created: time.Date(2020, time.August, 24, 13, 45, 24, 256039559, time.UTC), CreatedBy: "/bin/sh -c #(nop)  ENV CIPD_ROOT=/cipd", EmptyLayer: true},
			{Author: "", Created: time.Date(2021, time.September, 27, 13, 57, 56, 814739607, time.UTC), CreatedBy: "/bin/sh -c #(nop) COPY dir:f55d8860fc73e9ab333e1bf01c0864ddcc415e81086e9294e263d9fb06744040 in /cipd ", EmptyLayer: false},
			{Author: "", Created: time.Date(2021, time.September, 27, 13, 58, 0, 314301887, time.UTC), CreatedBy: "/bin/sh -c #(nop)  ENV PATH=/cipd:/cipd/cipd_bin_packages:/cipd/cipd_bin_packages/bin:/cipd/cipd_bin_packages/cpython:/cipd/cipd_bin_packages/cpython/bin:/cipd/cipd_bin_packages/cpython3:/cipd/cipd_bin_packages/cpython3/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", EmptyLayer: true},
			{Author: "", Created: time.Date(2023, time.April, 25, 4, 33, 13, 177574832, time.UTC), CreatedBy: "sh -c apt-get update && apt-get install -y wget openssh-client curl procps unzip vim less build-essential g++ g++-11 gcc gcc-11 gcc-11-base cpp cpp-11 libgcc-11-dev libstdc++-11-dev cmake && wget --output-document=/usr/local/bin/bazelisk https://github.com/bazelbuild/bazelisk/releases/download/v1.14.0/bazelisk-linux-amd64 && chmod a+x /usr/local/bin/bazelisk && cipd install skia/bots/go --root=/cipd", EmptyLayer: false},
		},
		OS: "linux",
		RootFS: ImageConfig_RootFS{
			DiffIDs: []string{
				"sha256:2760827628c28b78cd38417202a35f4ebfb74fa15d8d8d29486efe2f9e7d8ef7",
				"sha256:ac98c2b90fe0c941922d78822d9db0ef473792027a6a623047073d0d67dc097c",
				"sha256:44177851d94bc2e39573ffacb0e8eee5fb14c016a99f21eed076849f4909d41b",
				"sha256:b122b9c4c2b6de00cf3bd051500faa0ddb81d9b147693b7bf3556e6d3095366c",
				"sha256:403a461f89b41095e2b047b63864a02f6a74e26708b9bca48fab41b4ef39de28",
				"sha256:84ff92691f909a05b224e1c56abb4864f01b4f8e3c854e4bb4c7baf1d3f6d652",
			},
			Type: "layers"},
	}, config)
}

func TestListInstances(t *testing.T) {
	ctx := context.Background()
	md := mockhttpclient.MockGetDialogue([]byte(listTagsResponse))
	urlmock := mockhttpclient.NewURLMock()
	fakeURL := fmt.Sprintf(listTagsURLTemplate, fakeRegistry, fakeRepository, pageSize)
	urlmock.MockOnce(fakeURL, md)
	client := NewClient(ctx, urlmock.Client(), fakeRegistry)

	instances, err := client.ListInstances(ctx, fakeRepository)
	require.NoError(t, err)
	require.Equal(t, map[string]*ImageInstance{
		"sha256:000ba24df84b6490d68069cdee599d6599f3891f6420a37cdaa65852c9f1ecbc": {
			Digest:    "sha256:000ba24df84b6490d68069cdee599d6599f3891f6420a37cdaa65852c9f1ecbc",
			SizeBytes: 485929608,
			Tags: []string{
				"2022-06-22T19_11_03Z-louhi-7d6a680-clean",
			},
			Created:  time.Unix(0, 0),
			Uploaded: time.Unix(0, 1655925303330000000),
		},
		"sha256:001f6469f9513cda89206710c1711bb3f2cc169d7613d9d6bf688a7109eeeb30": {
			Digest:    "sha256:001f6469f9513cda89206710c1711bb3f2cc169d7613d9d6bf688a7109eeeb30",
			SizeBytes: 654679661,
			Tags: []string{
				"2023-03-27T13_22_20Z-louhi-378431d-clean",
			},
			Created:  time.Unix(0, 0),
			Uploaded: time.Unix(0, 1679924189163000000),
		},
	}, instances)
}

func TestListTags(t *testing.T) {
	ctx := context.Background()
	md := mockhttpclient.MockGetDialogue([]byte(listTagsResponse))
	urlmock := mockhttpclient.NewURLMock()
	fakeURL := fmt.Sprintf(listTagsURLTemplate, fakeRegistry, fakeRepository, pageSize)
	urlmock.MockOnce(fakeURL, md)
	client := NewClient(ctx, urlmock.Client(), fakeRegistry)

	repos, err := client.ListTags(ctx, fakeRepository)
	require.NoError(t, err)
	require.Equal(t, []string{
		"2022-06-22T19_11_03Z-louhi-7d6a680-clean",
		"2023-03-27T13_22_20Z-louhi-378431d-clean",
	}, repos)
}

func TestListRepositories(t *testing.T) {
	ctx := context.Background()
	urlmock := mockhttpclient.NewURLMock()
	url1 := fmt.Sprintf(catalogURLTemplate, fakeRegistry, pageSize)
	md1 := mockhttpclient.MockGetDialogue([]byte(listRepositoriesResponse1))
	urlmock.MockOnce(url1, md1)

	url2 := "https://gcr.io/v2/_catalog?n=100&last=skia-public/img3"
	md2 := mockhttpclient.MockGetDialogue([]byte(listRepositoriesResponse2))
	urlmock.MockOnce(url2, md2)

	client := NewClient(ctx, urlmock.Client(), fakeRegistry)

	repos, err := client.ListRepositories(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{
		"skia-public/img1",
		"skia-public/img2",
		"skia-public/img3",
		"skia-public/img4",
		"other/img5",
		"other/img6",
	}, repos)
}
