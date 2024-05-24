package cache

import (
	"fmt"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/patrickmn/go-cache"
	lruwrpr "github.com/prysmaticlabs/prysm/v5/cache/lru"
	"github.com/prysmaticlabs/prysm/v5/config/params"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/primitives"
	"github.com/prysmaticlabs/prysm/v5/container/slice"
)

type subnetIDs struct {
	attester          *lru.Cache
	attesterLock      sync.RWMutex
	aggregator        *lru.Cache
	aggregatorLock    sync.RWMutex
	persistentSubnets *cache.Cache
	subnetsLock       sync.RWMutex

	persistentColumnSubnets        *cache.Cache
	persistentColumnSubnetsLock    sync.RWMutex
	extraRequiredColumnSubnets     *cache.Cache
	extraRequiredColumnSubnetsLock sync.RWMutex
}

// SubnetIDs for attester and aggregator.
var SubnetIDs = newSubnetIDs()

var subnetKey = "persistent-subnets"

var columnSubnetKey = "column-persistent-subnets"

var extraColumRequiredKey = "extra-column-required"

func newSubnetIDs() *subnetIDs {
	// Given a node can calculate committee assignments of current epoch and next epoch.
	// Max size is set to 2 epoch length.
	cacheSize := int(params.BeaconConfig().SlotsPerEpoch.Mul(params.BeaconConfig().MaxCommitteesPerSlot * 2)) // lint:ignore uintcast -- constant values that would panic on startup if negative.
	attesterCache := lruwrpr.New(cacheSize)
	aggregatorCache := lruwrpr.New(cacheSize)
	epochDuration := time.Duration(params.BeaconConfig().SlotsPerEpoch.Mul(params.BeaconConfig().SecondsPerSlot))
	subLength := epochDuration * time.Duration(params.BeaconConfig().EpochsPerRandomSubnetSubscription)
	persistentCache := cache.New(subLength*time.Second, epochDuration*time.Second)

	colSubscirbeTime := time.Duration(params.BeaconConfig().EpochsPerColumnSubnetSubscription)
	persistentColumnSubnets := cache.New(colSubscirbeTime*epochDuration*time.Second, epochDuration*time.Second)
	extraRequiredColumnSubnets := cache.New(colSubscirbeTime*epochDuration*time.Second, epochDuration*time.Second)
	return &subnetIDs{attester: attesterCache,
		aggregator:                 aggregatorCache,
		persistentSubnets:          persistentCache,
		persistentColumnSubnets:    persistentColumnSubnets,
		extraRequiredColumnSubnets: extraRequiredColumnSubnets,
	}
}

// GetPersistentColumnSubnets retrieves the persistent subnet and expiration time of the beacon node's
// subscription.
func (s *subnetIDs) GetPersistentColumnSubnets() ([]uint64, bool, time.Time) {
	s.persistentColumnSubnetsLock.RLock()
	defer s.persistentColumnSubnetsLock.RUnlock()

	id, duration, ok := s.persistentColumnSubnets.GetWithExpiration(columnSubnetKey)
	if !ok {
		return []uint64{}, ok, time.Time{}
	}
	return id.([]uint64), ok, duration
}

// GetAllColumnSubnets retrieves all the non-expired subscribed subnets of the beacon node
// in the cache.
func (s *subnetIDs) GetAllColumnSubnets() []uint64 {
	s.persistentColumnSubnetsLock.RLock()
	defer s.persistentColumnSubnetsLock.RUnlock()

	itemsMap := s.persistentColumnSubnets.Items()
	var subnetIDs []uint64

	for _, v := range itemsMap {
		if v.Expired() {
			continue
		}
		subnetIDs = append(subnetIDs, v.Object.([]uint64)...)
	}
	return slice.SetUint64(subnetIDs)
}

// AddPersistentColumnSubnets adds the relevant committee for the beacon node along with its
// expiration period.
func (s *subnetIDs) AddPersistentColumnSubnets(subnetIDs []uint64, duration time.Duration) {
	s.persistentColumnSubnetsLock.Lock()
	defer s.persistentColumnSubnetsLock.Unlock()

	s.persistentColumnSubnets.Set(columnSubnetKey, subnetIDs, duration)
}

func (s *subnetIDs) AddExtraRequiredSubnetRecord(validatorIndex primitives.ValidatorIndex, duration time.Duration) {
	s.extraRequiredColumnSubnetsLock.Lock()
	defer s.extraRequiredColumnSubnetsLock.Unlock()
	key := fmt.Sprintf("%s_%d", extraColumRequiredKey, validatorIndex)
	s.extraRequiredColumnSubnets.Set(key, validatorIndex, duration)
}

func (s *subnetIDs) GetExtraRequiredColumns() int {
	s.extraRequiredColumnSubnetsLock.Lock()
	defer s.extraRequiredColumnSubnetsLock.Unlock()
	s.extraRequiredColumnSubnets.DeleteExpired()

	return s.extraRequiredColumnSubnets.ItemCount()
}

// AddAttesterSubnetID adds the subnet index for subscribing subnet for the attester of a given slot.
func (s *subnetIDs) AddAttesterSubnetID(slot primitives.Slot, subnetID uint64) {
	s.attesterLock.Lock()
	defer s.attesterLock.Unlock()

	ids := []uint64{subnetID}
	val, exists := s.attester.Get(slot)
	if exists {
		ids = slice.UnionUint64(append(val.([]uint64), ids...))
	}
	s.attester.Add(slot, ids)
}

// GetAttesterSubnetIDs gets the subnet IDs for subscribed subnets for attesters of the slot.
func (s *subnetIDs) GetAttesterSubnetIDs(slot primitives.Slot) []uint64 {
	s.attesterLock.RLock()
	defer s.attesterLock.RUnlock()

	val, exists := s.attester.Get(slot)
	if !exists {
		return nil
	}
	if v, ok := val.([]uint64); ok {
		return v
	}
	return nil
}

// AddAggregatorSubnetID adds the subnet ID for subscribing subnet for the aggregator of a given slot.
func (s *subnetIDs) AddAggregatorSubnetID(slot primitives.Slot, subnetID uint64) {
	s.aggregatorLock.Lock()
	defer s.aggregatorLock.Unlock()

	ids := []uint64{subnetID}
	val, exists := s.aggregator.Get(slot)
	if exists {
		ids = slice.UnionUint64(append(val.([]uint64), ids...))
	}
	s.aggregator.Add(slot, ids)
}

// GetAggregatorSubnetIDs gets the subnet IDs for subscribing subnet for aggregator of the slot.
func (s *subnetIDs) GetAggregatorSubnetIDs(slot primitives.Slot) []uint64 {
	s.aggregatorLock.RLock()
	defer s.aggregatorLock.RUnlock()

	val, exists := s.aggregator.Get(slot)
	if !exists {
		return []uint64{}
	}
	return val.([]uint64)
}

// GetPersistentSubnets retrieves the persistent subnet and expiration time of that validator's
// subscription.
func (s *subnetIDs) GetPersistentSubnets() ([]uint64, bool, time.Time) {
	s.subnetsLock.RLock()
	defer s.subnetsLock.RUnlock()

	id, duration, ok := s.persistentSubnets.GetWithExpiration(subnetKey)
	if !ok {
		return []uint64{}, ok, time.Time{}
	}
	return id.([]uint64), ok, duration
}

// GetAllSubnets retrieves all the non-expired subscribed subnets of all the validators
// in the cache.
func (s *subnetIDs) GetAllSubnets() []uint64 {
	s.subnetsLock.RLock()
	defer s.subnetsLock.RUnlock()

	itemsMap := s.persistentSubnets.Items()
	var committees []uint64

	for _, v := range itemsMap {
		if v.Expired() {
			continue
		}
		committees = append(committees, v.Object.([]uint64)...)
	}
	return slice.SetUint64(committees)
}

// AddPersistentCommittee adds the relevant committee for that particular validator along with its
// expiration period.
func (s *subnetIDs) AddPersistentCommittee(comIndex []uint64, duration time.Duration) {
	s.subnetsLock.Lock()
	defer s.subnetsLock.Unlock()

	s.persistentSubnets.Set(subnetKey, comIndex, duration)
}

// EmptyAllCaches empties out all the related caches and flushes any stored
// entries on them. This should only ever be used for testing, in normal
// production, handling of the relevant subnets for each role is done
// separately.
func (s *subnetIDs) EmptyAllCaches() {
	// Clear the caches.
	s.attesterLock.Lock()
	s.attester.Purge()
	s.attesterLock.Unlock()

	s.aggregatorLock.Lock()
	s.aggregator.Purge()
	s.aggregatorLock.Unlock()

	s.subnetsLock.Lock()
	s.persistentSubnets.Flush()
	s.subnetsLock.Unlock()

	s.persistentColumnSubnetsLock.Lock()
	s.persistentColumnSubnets.Flush()
	s.persistentColumnSubnetsLock.Unlock()
}
