package filesystem

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/verification"
	fieldparams "github.com/prysmaticlabs/prysm/v5/config/fieldparams"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/blocks"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/primitives"
	"github.com/prysmaticlabs/prysm/v5/io/file"
	ethpb "github.com/prysmaticlabs/prysm/v5/proto/prysm/v1alpha1"
	"github.com/prysmaticlabs/prysm/v5/runtime/logging"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

var (
	errColumnIndexOutOfBounds    = errors.New("column index in file name >= MaxColumnsPerBlock")
	errEmptyColumnWritten        = errors.New("zero bytes written to disk when saving column sidecar")
	errColumnSidecarEmptySSZData = errors.New("column sidecar marshalled to an empty ssz byte slice")
	errColumnNoBasePath          = errors.New("ColumnStorage base path not specified in init")
)

// ColumnStorageOption is a functional option for configuring a ColumnStorage.
type ColumnStorageOption func(*ColumnStorage) error

// WithBasePath is a required option that sets the base path of blob storage.
func WithColumnBasePath(base string) ColumnStorageOption {
	return func(c *ColumnStorage) error {
		c.base = base
		return nil
	}
}

// WithColumnRetentionEpochs is an option that changes the number of epochs columns will be persisted.
func WithColumnRetentionEpochs(e primitives.Epoch) ColumnStorageOption {
	return func(b *ColumnStorage) error {
		b.retentionEpochs = e
		return nil
	}
}

// WithSaveFsync is an option that causes Save to call fsync before renaming part files for improved durability.
func WithColumnSaveFsync(fsync bool) ColumnStorageOption {
	return func(b *ColumnStorage) error {
		b.fsync = fsync
		return nil
	}
}

// NewColumnStorage creates a new instance of the ColumnStorage object. Note that the implementation of ColumnStorage may
// attempt to hold a file lock to guarantee exclusive control of the column storage directory, so this should only be
// initialized once per beacon node.
func NewColumnStorage(opts ...ColumnStorageOption) (*ColumnStorage, error) {
	b := &ColumnStorage{}
	for _, o := range opts {
		if err := o(b); err != nil {
			return nil, errors.Wrap(err, "failed to create column storage")
		}
	}
	if b.base == "" {
		return nil, errColumnNoBasePath
	}
	b.base = path.Clean(b.base)
	if err := file.MkdirAll(b.base); err != nil {
		return nil, errors.Wrapf(err, "failed to create column storage at %s", b.base)
	}
	b.fs = afero.NewBasePathFs(afero.NewOsFs(), b.base)
	pruner, err := newColumnPruner(b.fs, b.retentionEpochs)
	if err != nil {
		return nil, err
	}
	b.pruner = pruner
	return b, nil
}

// ColumnStorage is the concrete implementation of the filesystem backend for saving and retrieving ColumnSidecars.
type ColumnStorage struct {
	base            string
	retentionEpochs primitives.Epoch
	fsync           bool
	fs              afero.Fs
	pruner          *columnPruner
}

// WarmCache runs the prune routine with an expiration of slot of 0, so nothing will be pruned, but the pruner's cache
// will be populated at node startup, avoiding a costly cold prune (~4s in syscalls) during syncing.
func (cs *ColumnStorage) WarmCache() {
	if cs.pruner == nil {
		return
	}
	go func() {
		if err := cs.pruner.prune(0); err != nil {
			log.WithError(err).Error("Error encountered while warming up column pruner cache")
		}
	}()
}

// Save saves columns given a list of sidecars.
func (cs *ColumnStorage) Save(sidecar blocks.VerifiedROColumn) error {
	//log.Debugf("func (cs *ColumnStorage) Save begins")
	startTime := time.Now()
	fname := namerForColumnSidecar(sidecar)
	sszPath := fname.path()
	//log.Debugf("sszPath is %s", sszPath)
	exists, err := afero.Exists(cs.fs, sszPath)
	if err != nil {
		return err
	}
	if exists {
		log.WithFields(logging.ColumnFields(sidecar.ROColumn)).Debug("Ignoring a duplicate column sidecar save attempt")
		return nil
	}
	if cs.pruner != nil {
		if err := cs.pruner.notify(sidecar.BlockRoot(), sidecar.Slot(), sidecar.Index); err != nil {
			return errors.Wrapf(err, "problem maintaining pruning cache/metrics for column sidecar with root=%#x", sidecar.BlockRoot())
		}
	}

	// Serialize the ethpb.ColumnSidecar to binary data using SSZ.
	sidecarData, err := sidecar.MarshalSSZ()
	if err != nil {
		return errors.Wrap(err, "failed to serialize column sidecar data")
	} else if len(sidecarData) == 0 {
		return errSidecarEmptySSZData
	}

	if err := cs.fs.MkdirAll(fname.dir(), directoryPermissions); err != nil {
		return err
	}
	partPath := fname.partPath(fmt.Sprintf("%p", sidecarData))

	partialMoved := false
	// Ensure the partial file is deleted.
	defer func() {
		if partialMoved {
			return
		}
		// It's expected to error if the save is successful.
		err = cs.fs.Remove(partPath)
		if err == nil {
			log.WithFields(logrus.Fields{
				"partPath": partPath,
			}).Debugf("Removed partial file")
		}
	}()

	// Create a partial file and write the serialized data to it.
	partialFile, err := cs.fs.Create(partPath)
	if err != nil {
		return errors.Wrap(err, "failed to create partial file")
	}

	n, err := partialFile.Write(sidecarData)
	if err != nil {
		closeErr := partialFile.Close()
		if closeErr != nil {
			return closeErr
		}
		return errors.Wrap(err, "failed to write to partial file")
	}
	if cs.fsync {
		if err := partialFile.Sync(); err != nil {
			return err
		}
	}

	if err := partialFile.Close(); err != nil {
		return err
	}

	if n != len(sidecarData) {
		return fmt.Errorf("failed to write the full bytes of sidecarData, wrote only %d of %d bytes", n, len(sidecarData))
	}

	if n == 0 {
		return errEmptyColumnWritten
	}

	// Atomically rename the partial file to its final name.
	err = cs.fs.Rename(partPath, sszPath)
	if err != nil {
		return errors.Wrap(err, "failed to rename partial file to final name")
	}
	partialMoved = true
	columnsWrittenCounter.Inc()
	columnSaveLatency.Observe(float64(time.Since(startTime).Milliseconds()))
	//log.Debugf("func (cs *ColumnStorage) Save ends")
	return nil
}

// Get retrieves a single ColumnSidecar by its root and index.
// Since ColumnStorage only writes columns that have undergone full verification, the return
// value is always a VerifiedROColumn.
func (cs *ColumnStorage) Get(root [32]byte, idx uint64) (blocks.VerifiedROColumn, error) {
	startTime := time.Now()
	expected := columnNamer{root: root, index: idx}
	encoded, err := afero.ReadFile(cs.fs, expected.path())
	var v blocks.VerifiedROColumn
	if err != nil {
		return v, err
	}
	s := &ethpb.ColumnSidecar{}
	if err := s.UnmarshalSSZ(encoded); err != nil {
		return v, err
	}
	ro, err := blocks.NewROColumnWithRoot(s, root)
	if err != nil {
		return blocks.VerifiedROColumn{}, err
	}
	defer func() {
		columnFetchLatency.Observe(float64(time.Since(startTime).Milliseconds()))
	}()
	return verification.ColumnSidecarNoop(ro)
}

// Remove removes all blobs for a given root.
func (cs *ColumnStorage) Remove(root [32]byte) error {
	rootDir := columnNamer{root: root}.dir()
	return cs.fs.RemoveAll(rootDir)
}

// Indices generates a bitmap representing which ColumnSidecar.Index values are present on disk for a given root.
// This value can be compared to the commitments observed in a block to determine which indices need to be found
// on the network to confirm data availability.
func (cs *ColumnStorage) Indices(root [32]byte) ([fieldparams.MaxColumnsPerBlock]bool, error) {
	var mask [fieldparams.MaxColumnsPerBlock]bool
	rootDir := columnNamer{root: root}.dir()
	entries, err := afero.ReadDir(cs.fs, rootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return mask, nil
		}
		return mask, err
	}
	for i := range entries {
		if entries[i].IsDir() {
			continue
		}
		name := entries[i].Name()
		if !strings.HasSuffix(name, sszExt) {
			continue
		}
		parts := strings.Split(name, ".")
		if len(parts) != 2 {
			continue
		}
		u, err := strconv.ParseUint(parts[0], 10, 64)
		if err != nil {
			return mask, errors.Wrapf(err, "unexpected directory entry breaks listing, %s", parts[0])
		}
		if u >= fieldparams.MaxColumnsPerBlock {
			return mask, errColumnIndexOutOfBounds
		}
		mask[u] = true
	}
	return mask, nil
}

// Clear deletes all files on the filesystem.
func (cs *ColumnStorage) Clear() error {
	dirs, err := listDir(cs.fs, ".")
	if err != nil {
		return err
	}
	for _, dir := range dirs {
		if err := cs.fs.RemoveAll(dir); err != nil {
			return err
		}
	}
	return nil
}

type columnNamer struct {
	root  [32]byte
	index uint64
}

func namerForColumnSidecar(sc blocks.VerifiedROColumn) columnNamer {
	return columnNamer{root: sc.BlockRoot(), index: sc.Index}
}

func (p columnNamer) dir() string {
	return rootString(p.root)
}

func (p columnNamer) partPath(entropy string) string {
	return path.Join(p.dir(), fmt.Sprintf("%s-%d.%s", entropy, p.index, partExt))
}

func (p columnNamer) path() string {
	return path.Join(p.dir(), fmt.Sprintf("%d.%s", p.index, sszExt))
}
