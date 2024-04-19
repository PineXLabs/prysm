package verification

import (
	"testing"

	"github.com/prysmaticlabs/prysm/v5/consensus-types/blocks"
)

// BlobSidecarNoop is a FAKE verification function that simply launders a ROBlob->VerifiedROBlob.
// TODO: find all code that uses this method and replace it with full verification.
func BlobSidecarNoop(b blocks.ROBlob) (blocks.VerifiedROBlob, error) {
	return blocks.NewVerifiedROBlob(b), nil
}

// ColumnSidecarNoop is a FAKE verification function that simply launders a ROColumn->VerifiedROColumn.
// TODO: find all code that uses this method and replace it with full verification.
func ColumnSidecarNoop(c blocks.ROColumn) (blocks.VerifiedROColumn, error) {
	return blocks.NewVerifiedROColumn(c), nil
}

// BlobSidecarSliceNoop is a FAKE verification function that simply launders a ROBlob->VerifiedROBlob.
// TODO: find all code that uses this method and replace it with full verification.
func BlobSidecarSliceNoop(b []blocks.ROBlob) ([]blocks.VerifiedROBlob, error) {
	vbs := make([]blocks.VerifiedROBlob, len(b))
	for i := range b {
		vbs[i] = blocks.NewVerifiedROBlob(b[i])
	}
	return vbs, nil
}

// ColumnSidecarSliceNoop is a FAKE verification function that simply launders a ROColumn->VerifiedROColumn.
// TODO: find all code that uses this method and replace it with full verification.
func ColumnSidecarSliceNoop(c []blocks.ROColumn) ([]blocks.VerifiedROColumn, error) {
	vbs := make([]blocks.VerifiedROColumn, len(c))
	for i := range c {
		vbs[i] = blocks.NewVerifiedROColumn(c[i])
	}
	return vbs, nil
}

// FakeVerifyForTest can be used by tests that need a VerifiedROBlob but don't want to do all the
// expensive set up to perform full validation.
func FakeVerifyForTest(t *testing.T, b blocks.ROBlob) blocks.VerifiedROBlob {
	// log so that t is truly required
	t.Log("producing fake VerifiedROBlob for a test")
	return blocks.NewVerifiedROBlob(b)
}

// FakeVerifyForColumnTest can be used by tests that need a VerifiedROColumn but don't want to do all the
// expensive set up to perform full validation.
func FakeVerifyForColumnTest(t *testing.T, c blocks.ROColumn) blocks.VerifiedROColumn {
	// log so that t is truly required
	t.Log("producing fake VerifiedROColumn for a test")
	return blocks.NewVerifiedROColumn(c)
}

// FakeVerifySliceForTest can be used by tests that need a []VerifiedROBlob but don't want to do all the
// expensive set up to perform full validation.
func FakeVerifySliceForTest(t *testing.T, b []blocks.ROBlob) []blocks.VerifiedROBlob {
	// log so that t is truly required
	t.Log("producing fake []VerifiedROBlob for a test")
	// tautological assertion that ensures this function can only be used in tests.
	vbs := make([]blocks.VerifiedROBlob, len(b))
	for i := range b {
		vbs[i] = blocks.NewVerifiedROBlob(b[i])
	}
	return vbs
}

// FakeVerifySliceForColumnTest can be used by tests that need a []VerifiedROColumn but don't want to do all the
// expensive set up to perform full validation.
func FakeVerifySliceForColumnTest(t *testing.T, c []blocks.ROColumn) []blocks.VerifiedROColumn {
	// log so that t is truly required
	t.Log("producing fake []VerifiedROColumn for a test")
	// tautological assertion that ensures this function can only be used in tests.
	vbs := make([]blocks.VerifiedROColumn, len(c))
	for i := range c {
		vbs[i] = blocks.NewVerifiedROColumn(c[i])
	}
	return vbs
}
