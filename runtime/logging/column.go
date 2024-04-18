package logging

import (
	"fmt"

	"github.com/prysmaticlabs/prysm/v5/consensus-types/blocks"
	"github.com/sirupsen/logrus"
)

// ColumnFields extracts a standard set of fields from a ColumnSidecar into a logrus.Fields struct
// which can be passed to log.WithFields.
func ColumnFields(column blocks.ROColumn) logrus.Fields {
	return logrus.Fields{
		"slot":          column.Slot(),
		"proposerIndex": column.ProposerIndex(),
		"blockRoot":     fmt.Sprintf("%#x", column.BlockRoot()),
		"parentRoot":    fmt.Sprintf("%#x", column.ParentRoot()),
		"index":         column.Index,
	}
}

// BlockFieldsFromColumn extracts the set of fields from a given ColumnSidecar which are shared by the block and
// all other sidecars for the block.
func BlockFieldsFromColumn(column blocks.ROColumn) logrus.Fields {
	return logrus.Fields{
		"slot":          column.Slot(),
		"proposerIndex": column.ProposerIndex(),
		"blockRoot":     fmt.Sprintf("%#x", column.BlockRoot()),
		"parentRoot":    fmt.Sprintf("%#x", column.ParentRoot()),
	}
}
