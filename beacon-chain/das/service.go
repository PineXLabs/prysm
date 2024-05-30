package das

import (
	"context"
	"time"

	dill_das "github.com/PineXLabs/das"
	lru "github.com/hashicorp/golang-lru"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	errors "github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/db/filesystem"
	lruwrpr "github.com/prysmaticlabs/prysm/v5/cache/lru"
	"github.com/prysmaticlabs/prysm/v5/config/params"
)

type csProvider struct {
	cs              *filesystem.ColumnStorage
	commitmentCache *lru.Cache
}

func (p *csProvider) GetColumn(ctx context.Context, key *dill_das.ColumnKey) (*dill_das.SampleColumn, error) {
	col, err := p.cs.Get([32]byte(key.Root), uint64(key.ColumNumber))
	if err != nil {
		return nil, err
	}

	comms, err := dill_das.UnmarshalCommitmentList(col.BlobKzgCommitments)
	if err != nil {
		return nil, err
	}
	proofs, err := dill_das.UnmarshalProofList(col.SegmentKzgProofs)
	if err != nil {
		return nil, err
	}
	segments := make([]dill_das.SegmentData, len(col.Segments))
	for i := range col.Segments {
		err = segments[i].Unmarshal(col.Segments[i])
		if err != nil {
			return nil, err
		}
	}
	sc := dill_das.SampleColumn{
		ColumnNumber:    int(key.ColumNumber),
		Commitments:     comms,
		Proofs:          proofs,
		SegmentDataList: segments,
	}
	return &sc, nil
}
func (p *csProvider) HasColumn(ctx context.Context, key *dill_das.ColumnKey) (bool, error) {
	_, err := p.cs.Get([32]byte(key.Root), uint64(key.ColumNumber))
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (p *csProvider) GetCommitment(root [32]byte, blobIndex uint64) (*dill_das.Commitment, error) {
	val, ok := p.commitmentCache.Get(root)
	if ok {
		comms, ok := val.([]dill_das.Commitment)
		if !ok {
			return nil, errors.New("bad cache value")
		}
		if int(blobIndex) >= len(comms) {
			return nil, errors.New("bad blob index")
		}
		return &comms[blobIndex], nil
	}
	indices, err := p.cs.Indices(root)
	if err != nil {
		return nil, err
	}
	for i, ok := range indices {
		if ok {
			col, err := p.cs.Get(root, uint64(i))
			if err != nil {
				return nil, err
			}
			comms, err := dill_das.UnmarshalCommitmentList(col.BlobKzgCommitments)
			if err != nil {
				return nil, err
			}
			p.commitmentCache.Add(root, comms)
			if int(blobIndex) >= len(comms) {
				return nil, errors.New("bad blob index")
			}
			return &comms[blobIndex], nil
		}
	}
	return nil, errors.New("not found")
}

type Config struct {
	host           host.Host
	columnStorage  *filesystem.ColumnStorage
	dataStorePath  string
	bootstrapNodes []string
}

type Option func(cfg *Config)

func WithHost(h host.Host) Option {
	return func(cfg *Config) {
		cfg.host = h
	}
}

func WithColumnStorage(cs *filesystem.ColumnStorage) Option {
	return func(cfg *Config) {
		cfg.columnStorage = cs
	}
}

func WithDataStorePath(path string) Option {
	return func(cfg *Config) {
		cfg.dataStorePath = path
	}
}

func WithBootStrapNodes(url []string) Option {
	return func(cfg *Config) {
		cfg.bootstrapNodes = url
	}
}

type Service struct {
	dasServer *dill_das.DASServer
	ctx       context.Context
}

func NewService(ctx context.Context, opts ...Option) (*Service, error) {
	cfg := &Config{}
	for _, opt := range opts {
		opt(cfg)
	}

	p := &csProvider{
		cs: cfg.columnStorage,
		// we keep commitments of the last epoch
		commitmentCache: lruwrpr.New(32 * 512 * 48),
	}

	infos := []peer.AddrInfo{}
	for _, addr := range cfg.bootstrapNodes {
		bsNode, err := peer.AddrInfoFromString(addr)
		if err != nil {
			return nil, err
		}
		infos = append(infos, *bsNode)
	}

	dillDasOpts := []dill_das.Option{
		dill_das.WithColumnProvider(p),
		dill_das.WithCommitmentProvider(p),
		dill_das.WithEnableDasProvider(true),
		dill_das.WithAllowPutNoCommitments(true),
		dill_das.WithDataTTL(time.Duration(params.BeaconConfig().DhtDataTTL)),
		dill_das.WithDasLogger(log),
		dill_das.WithBootstrapNodes(infos),
	}
	if cfg.dataStorePath != "" {
		dillDasOpts = append(dillDasOpts, dill_das.WithDatastorePath(cfg.dataStorePath))
	}
	dasServer, err := dill_das.NewDasServer(
		ctx,
		cfg.host,
		dillDasOpts...,
	)
	if err != nil {
		return nil, err
	}

	return &Service{
		ctx:       ctx,
		dasServer: dasServer,
	}, nil
}

func (s *Service) Start() {
	err := s.dasServer.Start()
	if err != nil {
		log.WithError(err).Fatal("start das server failed")
	}
}
func (s *Service) Stop() error {
	return s.dasServer.Stop()
}

func (s *Service) Status() error {
	return nil
}

func (s *Service) NotifyColumnReceived(root [32]byte, columnIndex int) {
	err := s.dasServer.ProvideColumn(s.ctx, root, columnIndex)
	if err != nil {
		log.WithError(err).Error("provide column failed")
	}
}
