package balances

import (
	"github.com/prysmaticlabs/prysm/beacon-chain/core/validators"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/mathutil"
	"github.com/prysmaticlabs/prysm/shared/params"
)

// baseRewardQuotient takes the total balance and calculates for
// the quotient of the base reward.
//
// Spec pseudocode definition:
//    base_reward_quotient =
//    	BASE_REWARD_QUOTIENT * integer_squareroot(total_balance // GWEI_PER_ETH)
func baseRewardQuotient(totalBalance uint64) uint64 {

	baseRewardQuotient := params.BeaconConfig().BaseRewardQuotient * mathutil.IntegerSquareRoot(
		totalBalance/params.BeaconConfig().Gwei)

	return baseRewardQuotient
}

// baseReward takes state and validator index to calculate for
// individual validator's base reward.
//
// Spec pseudocode definition:
//    base_reward(state, index) =
//    	get_effective_balance(state, index) // base_reward_quotient // 5
func baseReward(
	state *pb.BeaconState,
	validatorIndex uint32,
	baseRewardQuotient uint64) uint64 {

	validatorBalance := validators.EffectiveBalance(state, validatorIndex)
	return validatorBalance / baseRewardQuotient / 5
}

// inactivityPenalty takes state and validator index to calculate for
// individual validator's penalty for being offline.
//
// Spec pseudocode definition:
//    inactivity_penalty(state, index, epochs_since_finality) =
//    	base_reward(state, index) + get_effective_balance(state, index)
//    	* epochs_since_finality // INACTIVITY_PENALTY_QUOTIENT // 2
func inactivityPenalty(
	state *pb.BeaconState,
	validatorIndex uint32,
	baseRewardQuotient uint64,
	epochsSinceFinality uint64) uint64 {

	baseReward := baseReward(state, validatorIndex, baseRewardQuotient)
	validatorBalance := validators.EffectiveBalance(state, validatorIndex)
	return baseReward + validatorBalance*epochsSinceFinality/params.BeaconConfig().InactivityPenaltyQuotient/2
}
