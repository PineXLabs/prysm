package das

import (
	"bytes"

	"github.com/pkg/errors"
	fieldparams "github.com/prysmaticlabs/prysm/v5/config/fieldparams"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/blocks"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/primitives"
	"github.com/prysmaticlabs/prysm/v5/crypto/hash"
)

var (
	ErrDuplicateSidecar        = errors.New("duplicate sidecar stashed in AvailabilityStore")
	errIndexOutOfBounds        = errors.New("sidecar.index > MAX_BLOBS_PER_BLOCK")
	errColumnIndexOutOfBounds  = errors.New("ColumnSidecar.index > MAX_COLUMNS_PER_BLOCK")
	errCommitmentMismatch      = errors.New("KzgCommitment of sidecar in cache did not match block commitment")
	errCommitmentsHashMismatch = errors.New("The CommitmentsHash of sidecar in cache did not match the hash of block commitments")
	errMissingSidecar          = errors.New("no sidecar in cache for block commitment")
	errMissingColumnSidecar    = errors.New("no column sidecar in cache for block commitment")
)

// cacheKey includes the slot so that we can easily iterate through the cache and compare
// slots for eviction purposes. Whether the input is the block or the sidecar, we always have
// the root+slot when interacting with the cache, so it isn't an inconvenience to use both.
type cacheKey struct {
	slot primitives.Slot
	root [32]byte
}

type cache struct {
	entries map[cacheKey]*cacheEntry
}

type columnCache struct {
	entries map[cacheKey]*columnCacheEntry
}

func newCache() *cache {
	return &cache{entries: make(map[cacheKey]*cacheEntry)}
}

func newColumnCache() *columnCache {
	return &columnCache{entries: make(map[cacheKey]*columnCacheEntry)}
}

// keyFromSidecar is a convenience method for constructing a cacheKey from a BlobSidecar value.
func keyFromSidecar(sc blocks.ROBlob) cacheKey {
	return cacheKey{slot: sc.Slot(), root: sc.BlockRoot()}
}

// keyFromColumnSidecar is a convenience method for constructing a cacheKey from a ColumnSidecar value.
func keyFromColumnSidecar(sc blocks.ROColumn) cacheKey {
	return cacheKey{slot: sc.Slot(), root: sc.BlockRoot()}
}

// keyFromBlock is a convenience method for constructing a cacheKey from a ROBlock value.
func keyFromBlock(b blocks.ROBlock) cacheKey {
	return cacheKey{slot: b.Block().Slot(), root: b.Root()}
}

// ensure returns the entry for the given key, creating it if it isn't already present.
func (c *cache) ensure(key cacheKey) *cacheEntry {
	e, ok := c.entries[key]
	if !ok {
		e = &cacheEntry{}
		c.entries[key] = e
	}
	return e
}

// delete removes the cache entry from the cache.
func (c *cache) delete(key cacheKey) {
	delete(c.entries, key)
}

// ensure returns the entry for the given key, creating it if it isn't already present.
func (c *columnCache) ensure(key cacheKey) *columnCacheEntry {
	e, ok := c.entries[key]
	if !ok {
		e = &columnCacheEntry{}
		c.entries[key] = e
	}
	return e
}

// delete removes the cache entry from the column cache.
func (c *columnCache) delete(key cacheKey) {
	delete(c.entries, key)
}

// cacheEntry holds a fixed-length cache of BlobSidecars.
type cacheEntry struct {
	scs [fieldparams.MaxBlobsPerBlock]*blocks.ROBlob
}

// columnCacheEntry holds a fixed-length cache of ColumnSidecars.
type columnCacheEntry struct {
	scs [fieldparams.MaxColumnsPerBlock]*blocks.ROColumn
}

// stash adds an item to the in-memory cache of BlobSidecars.
// Only the first BlobSidecar of a given Index will be kept in the cache.
// stash will return an error if the given blob is already in the cache, or if the Index is out of bounds.
func (e *cacheEntry) stash(sc *blocks.ROBlob) error {
	if sc.Index >= fieldparams.MaxBlobsPerBlock {
		return errors.Wrapf(errIndexOutOfBounds, "index=%d", sc.Index)
	}
	if e.scs[sc.Index] != nil {
		return errors.Wrapf(ErrDuplicateSidecar, "root=%#x, index=%d, commitment=%#x", sc.BlockRoot(), sc.Index, sc.KzgCommitment)
	}
	e.scs[sc.Index] = sc
	return nil
}

// stash adds an item to the in-memory cache of ColumnSidecars.
// Only the first ColumnSidecar of a given Index will be kept in the cache.
// stash will return an error if the given column is already in the cache, or if the Index is out of bounds.
func (e *columnCacheEntry) stash(sc *blocks.ROColumn) error {
	if sc.Index >= fieldparams.MaxColumnsPerBlock {
		return errors.Wrapf(errColumnIndexOutOfBounds, "index=%d", sc.Index)
	}
	if e.scs[sc.Index] != nil {
		return errors.Wrapf(ErrDuplicateSidecar, "root=%#x, index=%d, commitmentsHash=%#x", sc.BlockRoot(), sc.Index, sc.CommitmentsHash)
	}
	e.scs[sc.Index] = sc
	return nil
}

// filter evicts sidecars that are not committed to by the block and returns custom
// errors if the cache is missing any of the commitments, or if the commitments in
// the cache do not match those found in the block. If err is nil, then all expected
// commitments were found in the cache and the sidecar slice return value can be used
// to perform a DA check against the cached sidecars.
func (e *cacheEntry) filter(root [32]byte, kc safeCommitmentArray) ([]blocks.ROBlob, error) {
	scs := make([]blocks.ROBlob, kc.count())
	for i := uint64(0); i < fieldparams.MaxBlobsPerBlock; i++ {
		if kc[i] == nil {
			if e.scs[i] != nil {
				return nil, errors.Wrapf(errCommitmentMismatch, "root=%#x, index=%#x, commitment=%#x, no block commitment", root, i, e.scs[i].KzgCommitment)
			}
			continue
		}

		if e.scs[i] == nil {
			return nil, errors.Wrapf(errMissingSidecar, "root=%#x, index=%#x", root, i)
		}
		if !bytes.Equal(kc[i], e.scs[i].KzgCommitment) {
			return nil, errors.Wrapf(errCommitmentMismatch, "root=%#x, index=%#x, commitment=%#x, block commitment=%#x", root, i, e.scs[i].KzgCommitment, kc[i])
		}
		scs[i] = *e.scs[i]
	}

	return scs, nil
}

// filter evicts sidecars that are not committed to by the block and returns custom
// errors if the cache is missing any of the commitments, or if the commitments in
// the cache do not match those found in the block. If err is nil, then all expected
// commitments were found in the cache and the sidecar slice return value can be used
// to perform a DA check against the cached sidecars.
func (e *columnCacheEntry) filter(root [32]byte, kc columnSafeCommitmentArray) ([]blocks.ROColumn, error) {

	var commitConcat []byte
	for _, c := range kc {
		commitConcat = append(commitConcat, c...)
	}
	commitmentsHash := hash.Hash(commitConcat)

	scs := make([]blocks.ROColumn, fieldparams.MaxColumnsPerBlock)
	for i := uint64(0); i < fieldparams.MaxColumnsPerBlock; i++ {
		if e.scs[i] == nil {
			return nil, errors.Wrapf(errMissingColumnSidecar, "root=%#x, index=%#x", root, i)
		}
		if !bytes.Equal(commitmentsHash[:], e.scs[i].CommitmentsHash) { //todo: compare commitments one by one directly?
			return nil, errors.Wrapf(errCommitmentsHashMismatch, "root=%#x, index=%#x, commitmentsHash=%#x, the calculated hash of block commitments=%#x", root, i, e.scs[i].CommitmentsHash, commitmentsHash)
		}
		scs[i] = *e.scs[i]
	}

	return scs, nil
}

// safeCommitmentArray is a fixed size array of commitment byte slices. This is helpful for avoiding
// gratuitous bounds checks.
type safeCommitmentArray [fieldparams.MaxBlobsPerBlock][]byte

func (s safeCommitmentArray) count() int {
	for i := range s {
		if s[i] == nil {
			return i
		}
	}
	return fieldparams.MaxBlobsPerBlock
}

// columnSafeCommitmentArray is a fixed size array of commitment byte slices. This is helpful for avoiding
// gratuitous bounds checks.
type columnSafeCommitmentArray [fieldparams.MaxColumnsPerBlock][]byte

func (s columnSafeCommitmentArray) count() int {
	for i := range s {
		if s[i] == nil {
			return i
		}
	}
	return fieldparams.MaxColumnsPerBlock
}
