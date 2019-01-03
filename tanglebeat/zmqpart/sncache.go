package zmqpart

import (
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"github.com/lunfardo314/tanglebeat/tanglebeat/hashcache"
)

type hashCacheSN struct {
	hashcache.HashCacheBase
	largestIndex             int
	largestIndexCandidate    int
	largestIndexCandidateUri string
	indexChanged             uint64
}

func newHashCacheSN(hashLen int, segmentDurationSec int, retentionPeriodSec int) *hashCacheSN {
	ret := &hashCacheSN{
		HashCacheBase: *hashcache.NewHashCacheBase("sncache", hashLen, segmentDurationSec, retentionPeriodSec),
	}
	//ret.StartPurge()
	return ret
}

func (cache *hashCacheSN) checkCurrentMilestoneIndex(index int, uri string) (bool, uint64) {
	cache.Lock()
	defer cache.Unlock()

	if index < cache.largestIndex {
		return true, cache.indexChanged
	}
	if index == cache.largestIndex {
		return false, cache.indexChanged
	}
	// milestone index is considered changed only when seen from two different zmq hosts
	if cache.largestIndexCandidate == index && cache.largestIndexCandidateUri != uri {
		debugf("------ milestone index changed %v --> %v ", cache.largestIndex, index)
		cache.largestIndex = index
		cache.indexChanged = utils.UnixMsNow()
	} else {
		cache.largestIndexCandidate = index
		cache.largestIndexCandidateUri = uri
	}
	return false, cache.indexChanged
}
