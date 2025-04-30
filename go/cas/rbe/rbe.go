package rbe

import (
	"context"
	"sort"

	"github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/command"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/digest"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/filemetadata"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/uploadinfo"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"

	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/golang/protobuf/ptypes/wrappers"
	"go.opencensus.io/trace"
	"golang.org/x/oauth2"
	grpc_oauth "google.golang.org/grpc/credentials/oauth"

	"go.skia.org/infra/go/cas"
	"go.skia.org/infra/go/skerr"
)

const (
	// ExcludeGitDir is a regular expression which may be passed to
	// Client.Upload to exclude the ".git" directory.
	ExcludeGitDir = `^(.*\/)*\.git(\/.*)*$`

	// InstanceChromiumSwarm is the RBE instance used for chromium-swarm.
	InstanceChromiumSwarm = "projects/chromium-swarm/instances/default_instance"

	// InstanceChromiumSwarmDev is the RBE instance used for chromium-swarm-dev.
	InstanceChromiumSwarmDev = "projects/chromium-swarm-dev/instances/default_instance"

	// InstanceChromeSwarming is the RBE instance used for chrome-swarming.
	InstanceChromeSwarming = "projects/chrome-swarming/instances/default_instance"

	rbeService = "remotebuildexecution.googleapis.com:443"
)

var (
	// EmptyDigest is the digest of an empty entry in RBE-CAS.
	EmptyDigest = digest.Empty.String()
)

// StringToDigest splits the digest string into a digest.Digest instance.
func StringToDigest(str string) (string, int64, error) {
	digest, err := digest.NewFromString(str)
	if err != nil {
		return "", 0, skerr.Wrap(err)
	}
	return digest.Hash, digest.Size, nil
}

// DigestToString creates a string for the digest with the given hash and size.
func DigestToString(hash string, size int64) string {
	return digest.Digest{
		Hash: hash,
		Size: size,
	}.String()
}

// Client is a struct used to interact with RBE-CAS.
type Client struct {
	client   RBEClient
	instance string
}

// RBEClient is an abstraction of client.Client which enables mocks for testing.
type RBEClient interface {
	Close() error
	ComputeMerkleTree(ctx context.Context, execRoot, workingDir, remoteWorkingDir string, is *command.InputSpec, cache filemetadata.Cache) (root digest.Digest, inputs []*uploadinfo.Entry, stats *client.TreeStats, err error)
	DownloadDirectory(ctx context.Context, d digest.Digest, execRoot string, cache filemetadata.Cache) (map[string]*client.TreeOutput, *client.MovedBytesMetadata, error)
	GetDirectoryTree(ctx context.Context, d *remoteexecution.Digest) (result []*remoteexecution.Directory, err error)
	UploadIfMissing(ctx context.Context, data ...*uploadinfo.Entry) ([]digest.Digest, int64, error)
}

// NewClient returns a Client instance.
func NewClient(ctx context.Context, instance string, ts oauth2.TokenSource) (*Client, error) {
	client, err := client.NewClient(ctx, instance, client.DialParams{
		Service:            rbeService,
		TransportCredsOnly: true,
	}, &client.PerRPCCreds{
		Creds: grpc_oauth.TokenSource{
			TokenSource: ts,
		},
	})
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &Client{
		client:   client,
		instance: instance,
	}, nil
}

// Upload the given paths to RBE-CAS.
func (c *Client) Upload(ctx context.Context, root string, paths, excludes []string) (string, error) {
	ctx, span := trace.StartSpan(ctx, "cas_rbe_Upload")
	span.AddAttributes(trace.StringAttribute("root", root))
	span.AddAttributes(trace.Int64Attribute("num_paths", int64(len(paths))))
	span.AddAttributes(trace.Int64Attribute("num_excludes", int64(len(excludes))))
	defer span.End()
	ex := make([]*command.InputExclusion, 0, len(excludes))
	for _, regex := range excludes {
		ex = append(ex, &command.InputExclusion{
			Regex: regex,
			Type:  command.UnspecifiedInputType,
		})
	}
	is := command.InputSpec{
		Inputs:          paths,
		InputExclusions: ex,
	}
	rootDigest, entries, _, err := c.client.ComputeMerkleTree(ctx, root, "" /* =workingDir */, "" /* =remoteWorkingDir */, &is, filemetadata.NewNoopCache())
	if err != nil {
		return "", skerr.Wrap(err)
	}
	if _, _, err := c.client.UploadIfMissing(ctx, entries...); err != nil {
		return "", skerr.Wrap(err)
	}
	return rootDigest.String(), nil
}

// Download the given digests from RBE-CAS.
func (c *Client) Download(ctx context.Context, root, casDigest string) error {
	ctx, span := trace.StartSpan(ctx, "cas_rbe_Download")
	defer span.End()
	d, err := digest.NewFromString(casDigest)
	if err != nil {
		return skerr.Wrap(err)
	}
	if _, _, err := c.client.DownloadDirectory(ctx, d, root, filemetadata.NewNoopCache()); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// checkNodePropertiesIdentical returns an error if the two NodeProperties are
// not identical.
func checkNodePropertiesIdentical(a, b *remoteexecution.NodeProperties) error {
	if (a != nil) != (b != nil) {
		return skerr.Fmt("One NodeProperties is set while the other is not")
	}
	if a != nil {
		if (a.Mtime != nil) != (b.Mtime != nil) {
			return skerr.Fmt("One file has Mtime while the other does not")
		}
		if a.Mtime != nil {
			if !a.Mtime.AsTime().Equal(b.Mtime.AsTime()) {
				return skerr.Fmt("Mtime %q != %q", a.Mtime, b.Mtime)
			}
		}
		if (a.UnixMode != nil) != (b.UnixMode != nil) {
			return skerr.Fmt("One NodeProperties has UnixMode while the other does not")
		}
		if a.UnixMode != nil {
			if a.UnixMode.Value != b.UnixMode.Value {
				return skerr.Fmt("UnixMode %q != %q", a.UnixMode.Value, b.UnixMode.Value)
			}
		}
		if len(a.Properties) != len(b.Properties) {
			return skerr.Fmt("Properties differ in length; %d vs %d", len(a.Properties), len(b.Properties))
		}
		propsA := make([]string, 0, len(a.Properties))
		for _, prop := range a.Properties {
			propsA = append(propsA, prop.String())
		}
		sort.Strings(propsA)
		propsB := make([]string, 0, len(b.Properties))
		for _, prop := range b.Properties {
			propsB = append(propsB, prop.String())
		}
		sort.Strings(propsB)
		for idx, propA := range propsA {
			propB := propsB[idx]
			if propA != propB {
				return skerr.Fmt("Properties differ: %s vs %s", propA, propB)
			}
		}
	}
	return nil
}

// checkFilesIdentical returns an error if the two FileNodes are not identical.
func checkFilesIdentical(a, b *remoteexecution.FileNode) error {
	if a.Name != b.Name {
		return skerr.Fmt("Name %q != %q", a.Name, b.Name)
	}
	if a.IsExecutable != b.IsExecutable {
		return skerr.Fmt("Executable %v != %v", a.IsExecutable, b.IsExecutable)
	}
	if (a.Digest != nil) != (b.Digest != nil) {
		return skerr.Fmt("One file has a digest while the other does not")
	}
	if a.Digest != nil && b.Digest != nil {
		if a.Digest.Hash != b.Digest.Hash {
			return skerr.Fmt("Digest hash %q != %q", a.Digest.Hash, b.Digest.Hash)
		}
		if a.Digest.SizeBytes != b.Digest.SizeBytes {
			return skerr.Fmt("Size %q != %q", a.Digest.SizeBytes, b.Digest.SizeBytes)
		}
	}
	return checkNodePropertiesIdentical(a.NodeProperties, b.NodeProperties)
}

// checkSymlinksIdentical returns an error if the two SymlinkNodes are not
// identical.
func checkSymlinksIdentical(a, b *remoteexecution.SymlinkNode) error {
	if a.Name != b.Name {
		return skerr.Fmt("Name %q != %q", a.Name, b.Name)
	}
	if a.Target != b.Target {
		return skerr.Fmt("Target %q != %q", a.Target, b.Target)
	}
	return checkNodePropertiesIdentical(a.NodeProperties, b.NodeProperties)
}

// directoryNode helps to organize a tree of remoteexecution.Directory.
type directoryNode struct {
	*remoteexecution.Directory
	Children map[string]*directoryNode
}

// makeTree builds a tree of directoryNodes using the given Directories.
// Requires that the map of Directories is complete, ie. each of the
// subdirectory digests referenced by each of the Directories is itself present
// in the map.
func makeTree(dirsByDigest map[digest.Digest]*remoteexecution.Directory, d digest.Digest) (*directoryNode, error) {
	dir, ok := dirsByDigest[d]
	if !ok {
		return nil, skerr.Fmt("no information provided for digest %s", d.String())
	}
	rv := &directoryNode{
		Directory: dir,
		Children:  make(map[string]*directoryNode, len(dir.Directories)),
	}
	for _, childDir := range rv.Directories {
		childNode, err := makeTree(dirsByDigest, digest.NewFromProtoUnvalidated(childDir.Digest))
		if err != nil {
			return nil, err
		}
		rv.Children[childDir.Name] = childNode
	}
	return rv, nil
}

// mergeTrees merges the given trees of directoryNodes into a new directoryNode.
// Returns an error if the two trees are incompatible, eg. trees that contain
// files with the same name but different digests.
func (c *Client) mergeTrees(ctx context.Context, a, b *directoryNode) (*directoryNode, error) {
	if a == nil {
		return b, nil
	}
	if b == nil {
		return a, nil
	}
	// Files.
	filesMap := map[string]*remoteexecution.FileNode{}
	for _, file := range a.Files {
		filesMap[file.Name] = file
	}
	for _, file := range b.Files {
		exist, ok := filesMap[file.Name]
		if ok {
			if err := checkFilesIdentical(file, exist); err != nil {
				return nil, skerr.Wrapf(err, "file %s", file.Name)
			}
		} else {
			filesMap[file.Name] = file
		}
	}
	fileNames := make([]string, 0, len(filesMap))
	for name := range filesMap {
		fileNames = append(fileNames, name)
	}
	sort.Strings(fileNames)
	files := make([]*remoteexecution.FileNode, 0, len(filesMap))
	for _, fileName := range fileNames {
		files = append(files, filesMap[fileName])
	}

	// Symlinks.
	symlinksMap := map[string]*remoteexecution.SymlinkNode{}
	for _, symlink := range a.Symlinks {
		symlinksMap[symlink.Name] = symlink
	}
	for _, symlink := range b.Symlinks {
		exist, ok := symlinksMap[symlink.Name]
		if ok {
			if err := checkSymlinksIdentical(symlink, exist); err != nil {
				return nil, skerr.Wrapf(err, "symlink %s", symlink.Name)
			}
		} else {
			symlinksMap[symlink.Name] = symlink
		}
	}
	symlinkNames := make([]string, 0, len(symlinksMap))
	for name := range symlinksMap {
		symlinkNames = append(symlinkNames, name)
	}
	sort.Strings(symlinkNames)
	symlinks := make([]*remoteexecution.SymlinkNode, 0, len(symlinksMap))
	for _, symlinkName := range symlinkNames {
		symlinks = append(symlinks, symlinksMap[symlinkName])
	}

	// Directories.
	children := map[string]*directoryNode{}
	dirsMap := map[string]*remoteexecution.DirectoryNode{}
	for _, dir := range a.Directories {
		dirsMap[dir.Name] = dir
	}
	for _, dir := range b.Directories {
		if _, ok := dirsMap[dir.Name]; ok {
			merged, err := c.mergeTrees(ctx, a.Children[dir.Name], b.Children[dir.Name])
			if err != nil {
				return nil, skerr.Wrap(err)
			}

			entry, err := uploadinfo.EntryFromProto(merged.Directory)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			if _, _, err := c.client.UploadIfMissing(ctx, entry); err != nil {
				return nil, skerr.Wrap(err)
			}
			dn := &remoteexecution.DirectoryNode{
				Name:   dir.Name,
				Digest: entry.Digest.ToProto(),
			}
			dirsMap[dir.Name] = dn
			children[dir.Name] = merged
		} else {
			dirsMap[dir.Name] = dir
			children[dir.Name] = b.Children[dir.Name]
		}
	}
	dirNames := make([]string, 0, len(dirsMap))
	for name := range dirsMap {
		dirNames = append(dirNames, name)
	}
	sort.Strings(dirNames)
	dirs := make([]*remoteexecution.DirectoryNode, 0, len(dirsMap))
	for _, dirName := range dirNames {
		dirs = append(dirs, dirsMap[dirName])
	}

	// Properties.
	properties := map[string]string{}
	var mTime *timestamp.Timestamp
	var unixMode *wrappers.UInt32Value
	if a.NodeProperties != nil {
		for _, prop := range a.NodeProperties.Properties {
			properties[prop.Name] = prop.Value
		}
		if a.NodeProperties.Mtime != nil {
			mTime = a.NodeProperties.Mtime
		}
		if a.NodeProperties.UnixMode != nil {
			unixMode = a.NodeProperties.UnixMode
		}
	}
	if b.NodeProperties != nil {
		// TODO(borenet): Do we need to make sure that both trees have the same
		// set of property keys?
		for _, prop := range b.NodeProperties.Properties {
			exist, ok := properties[prop.Name]
			if ok {
				if exist != prop.Value {
					return nil, skerr.Fmt("Property %q has conflicting values %q and %q", prop.Name, prop.Value, exist)
				}
			} else {
				properties[prop.Name] = prop.Value
			}
		}
		if b.NodeProperties.Mtime != nil {
			if mTime == nil {
				mTime = b.NodeProperties.Mtime
			} else if mTime.AsTime().Before(b.NodeProperties.Mtime.AsTime()) {
				mTime = b.NodeProperties.Mtime
			}
		}
		if b.NodeProperties.UnixMode != nil {
			if unixMode == nil {
				unixMode = b.NodeProperties.UnixMode
			} else if unixMode.Value != b.NodeProperties.UnixMode.Value {
				return nil, skerr.Fmt("Directory has conflicting modes %v and %v", unixMode, b.NodeProperties.UnixMode)
			}
		}
	}
	var nodeProps *remoteexecution.NodeProperties
	if len(properties) > 0 || mTime != nil || unixMode != nil {
		var propList []*remoteexecution.NodeProperty
		if len(properties) > 0 {
			propList = make([]*remoteexecution.NodeProperty, 0, len(properties))
			for name, value := range properties {
				propList = append(propList, &remoteexecution.NodeProperty{
					Name:  name,
					Value: value,
				})
			}
		}
		nodeProps = &remoteexecution.NodeProperties{
			Properties: propList,
			Mtime:      mTime,
			UnixMode:   unixMode,
		}
	}

	rv := &directoryNode{
		Directory: &remoteexecution.Directory{
			Files:          files,
			Directories:    dirs,
			Symlinks:       symlinks,
			NodeProperties: nodeProps,
		},
		Children: children,
	}
	return rv, nil
}

// Merge the given digests, returning a new digest which contains all of them.
func (c *Client) Merge(ctx context.Context, digests []string) (string, error) {
	// Shortcut for empty/single inputs.
	if len(digests) == 0 {
		return digest.Empty.String(), nil
	} else if len(digests) == 1 {
		return digests[0], nil
	}
	ctx, span := trace.StartSpan(ctx, "cas_rbe_Merge")
	span.AddAttributes(trace.Int64Attribute("num_digests", int64(len(digests))))
	defer span.End()
	// Build an in-memory directory tree for each of the given digests.
	var trees []*directoryNode
	for _, casDigest := range digests {
		// Normalize the digest and retrieve the directory tree from the API.
		d, err := digest.NewFromString(casDigest)
		if err != nil {
			return "", skerr.Wrap(err)
		}
		dirs, err := c.client.GetDirectoryTree(ctx, d.ToProto())
		if err != nil {
			return "", skerr.Wrapf(err, "failed retrieving %s", d.String())
		}

		// Start by creating a map of Directory digest to Directory, so that we
		// don't have to rely on the ordering of the Directories returned by
		// GetDirectoryTree.
		dirsByDigest := make(map[digest.Digest]*remoteexecution.Directory, len(dirs))
		rootDigest, err := digest.NewFromMessage(dirs[0])
		if err != nil {
			return "", skerr.Wrapf(err, "failed to create digest for root dir: %+v", dirs[0])
		}
		dirsByDigest[rootDigest] = dirs[0]
		for _, dir := range dirs[1:] {
			d, err := digest.NewFromMessage(dir)
			if err != nil {
				return "", skerr.Wrapf(err, "failed to create digest for dir: %+v", dir)
			}
			dirsByDigest[d] = dir
		}

		// Build the in-memory tree representation.
		tree, err := makeTree(dirsByDigest, rootDigest)
		if err != nil {
			return "", skerr.Wrapf(err, "failed to create tree for digest %q", casDigest)
		}
		trees = append(trees, tree)
	}

	// Merge the trees.
	root := trees[0]
	for _, tree := range trees[1:] {
		var err error
		root, err = c.mergeTrees(ctx, root, tree)
		if err != nil {
			return "", skerr.Wrap(err)
		}
	}

	// Upload the new root.
	entry, err := uploadinfo.EntryFromProto(root.Directory)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	if _, _, err := c.client.UploadIfMissing(ctx, entry); err != nil {
		return "", skerr.Wrap(err)
	}
	return entry.Digest.String(), nil
}

// Close implements cas.CAS.
func (c *Client) Close() error {
	return c.client.Close()
}

func GetCASInstance(c cas.CAS) (string, error) {
	client, ok := c.(*Client)
	if !ok {
		return "", skerr.Fmt("CAS is not an instance of rbe.Client")
	}
	return client.instance, nil
}

var _ cas.CAS = &Client{}
