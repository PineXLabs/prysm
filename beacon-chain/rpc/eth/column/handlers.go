package column

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/prysmaticlabs/prysm/v5/api/server/structs"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/rpc/core"
	field_params "github.com/prysmaticlabs/prysm/v5/config/fieldparams"
	"github.com/prysmaticlabs/prysm/v5/network/httputil"
	eth "github.com/prysmaticlabs/prysm/v5/proto/prysm/v1alpha1"
	log "github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

// Columns is an HTTP handler for Beacon API getColumns.
func (s *Server) Columns(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "beacon.Columns")
	defer span.End()
	var sidecars []*eth.ColumnSidecar

	log.Debugf("Columns r.URL is %s", r.URL)
	indices, err := parseIndices(r.URL)
	if err != nil {
		httputil.HandleError(w, err.Error(), http.StatusBadRequest)
		return
	}
	segments := strings.Split(r.URL.Path, "/")
	blockId := segments[len(segments)-1]

	//log.Debugf("blockId is %s, indices is %v", blockId, indices)
	verifiedColumns, rpcErr := s.Blocker.Columns(ctx, blockId, indices)
	if rpcErr != nil {
		code := core.ErrorReasonToHTTP(rpcErr.Reason)
		switch code {
		case http.StatusBadRequest:
			httputil.HandleError(w, "Invalid block ID: "+rpcErr.Err.Error(), code)
			return
		case http.StatusNotFound:
			httputil.HandleError(w, "Block not found: "+rpcErr.Err.Error(), code)
			return
		case http.StatusInternalServerError:
			httputil.HandleError(w, "Internal server error: "+rpcErr.Err.Error(), code)
			return
		default:
			httputil.HandleError(w, rpcErr.Err.Error(), code)
			return
		}
	}
	//log.Debugf("verifiedColumns length is %d", len(verifiedColumns))
	for i := range verifiedColumns {
		sidecars = append(sidecars, verifiedColumns[i].ColumnSidecar)
	}
	if httputil.RespondWithSsz(r) {
		sidecarResp := &eth.ColumnSidecars{
			ColumnSidecars: sidecars,
		}
		sszResp, err := sidecarResp.MarshalSSZ()
		if err != nil {
			httputil.HandleError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		httputil.WriteSsz(w, sszResp, "column_sidecars.ssz")
		return
	}

	httputil.WriteJson(w, buildColumnSidecarsResponse(sidecars))
}

// parseIndices filters out invalid and duplicate column indices
func parseIndices(url *url.URL) ([]uint64, error) {
	rawIndices := url.Query()["indices"]
	indices := make([]uint64, 0, field_params.MaxColumnsPerBlock)
	invalidIndices := make([]string, 0)
loop:
	for _, raw := range rawIndices {
		ix, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			invalidIndices = append(invalidIndices, raw)
			continue
		}
		if ix >= field_params.MaxColumnsPerBlock {
			invalidIndices = append(invalidIndices, raw)
			continue
		}
		for i := range indices {
			if ix == indices[i] {
				continue loop
			}
		}
		indices = append(indices, ix)
	}

	if len(invalidIndices) > 0 {
		return nil, fmt.Errorf("requested column indices %v are invalid", invalidIndices)
	}
	return indices, nil
}

func buildColumnSidecarsResponse(columnSidecars []*eth.ColumnSidecar) *structs.ColumnSidecarsResponse {
	resp := &structs.ColumnSidecarsResponse{Data: make([]*structs.ColumnSidecar, len(columnSidecars))}
	for i, sc := range columnSidecars {
		var proofs = make([]*structs.KzgCommitmentInclusionProof, len(sc.CommitmentInclusionProofs))
		for j := range sc.CommitmentInclusionProofs {
			proofs[j] = &structs.KzgCommitmentInclusionProof{}
			proofs[j].CommitmentInclusionProof = make([]string, len(sc.CommitmentInclusionProofs[j].CommitmentInclusionProof))
			for k := range sc.CommitmentInclusionProofs[j].CommitmentInclusionProof {
				proofs[j].CommitmentInclusionProof[k] = hexutil.Encode(sc.CommitmentInclusionProofs[j].CommitmentInclusionProof[k])
			}
		}

		segments := make([]string, len(sc.Segments))
		for j := range sc.Segments {
			segments[j] = hexutil.Encode(sc.Segments[j])
		}

		commitments := make([]string, len(sc.BlobKzgCommitments))
		for j := range sc.BlobKzgCommitments {
			commitments[j] = hexutil.Encode(sc.BlobKzgCommitments[j])
		}

		segmentKzgProofs := make([]string, len(sc.SegmentKzgProofs))
		for j := range sc.SegmentKzgProofs {
			segmentKzgProofs[j] = hexutil.Encode(sc.SegmentKzgProofs[j])
		}

		resp.Data[i] = &structs.ColumnSidecar{
			Index:                     strconv.FormatUint(sc.Index, 10),
			Segments:                  segments,
			BlobKzgCommitments:        commitments,
			SegmentKzgProofs:          segmentKzgProofs,
			SignedBlockHeader:         structs.SignedBeaconBlockHeaderFromConsensus(sc.SignedBlockHeader),
			CommitmentInclusionProofs: proofs,
			CommitmentsHash:           hexutil.Encode(sc.CommitmentsHash),
		}
	}
	return resp
}
