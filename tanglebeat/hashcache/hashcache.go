package hashcache

import (
	"github.com/lunfardo314/tanglebeat/lib/ebuffer"
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"github.com/lunfardo314/tanglebeat/tanglebeat/cfg"
)

type CacheEntry struct {
	FirstSeen uint64
	LastSeen  uint64
	Visits    int
	Data      interface{}
}

type cacheSegment struct {
	ebuffer.ExpiringSegmentBase
	themap map[string]CacheEntry
}

type HashCacheBase struct {
	ebuffer.ExpiringBuffer
	hashLen               int
	segmentDurationMsCopy uint64
	retentionPeriodMsCopy uint64
}

var segmentConstructor = func(prev ebuffer.ExpiringSegment) ebuffer.ExpiringSegment {
	ret := &cacheSegment{
		ExpiringSegmentBase: *ebuffer.NewExpiringSegmentBase(),
		themap:              make(map[string]CacheEntry),
	}
	ret.SetPrev(prev)
	return ebuffer.ExpiringSegment(ret)
}

func NewHashCacheBase(id string, hashLen int, segmentDurationSec int, retentionPeriodSec int) *HashCacheBase {
	return &HashCacheBase{
		ExpiringBuffer:        *ebuffer.NewExpiringBuffer(id, segmentDurationSec, retentionPeriodSec, segmentConstructor),
		hashLen:               hashLen,
		segmentDurationMsCopy: uint64(segmentDurationSec * 1000),
		retentionPeriodMsCopy: uint64(retentionPeriodSec * 1000),
	}
}

func (seg *cacheSegment) Put(args ...interface{}) {
	shorthash := args[0].(string)
	nowis := utils.UnixMsNow()
	seg.themap[shorthash] = CacheEntry{
		FirstSeen: nowis,
		LastSeen:  nowis,
		Visits:    1,
		Data:      args[1],
	}
}

func (seg *cacheSegment) Size() int {
	return len(seg.themap)
}

// searches for the hash, marks if found
func (seg *cacheSegment) Find(shorthash string, ret *CacheEntry) bool {
	entry, ok := seg.themap[shorthash]
	if !ok {
		return false
	}
	seg.themap[shorthash] = CacheEntry{
		FirstSeen: entry.FirstSeen,
		LastSeen:  utils.UnixMsNow(),
		Visits:    entry.Visits + 1,
		Data:      entry.Data,
	}
	if ret != nil {
		*ret = seg.themap[shorthash]
	}
	return true
}

func (seg *cacheSegment) FindWithDelete(shorthash string, ret *CacheEntry) bool {
	entry, ok := seg.themap[shorthash]
	if !ok {
		return false
	}
	if ret != nil {
		*ret = entry
	}
	delete(seg.themap, shorthash)
	return true
}

func (cache *HashCacheBase) shortHash(hash string) string {
	if cache.hashLen == 0 {
		return hash
	}
	ret := make([]byte, cache.hashLen)
	copy(ret, hash[:cache.hashLen])
	return string(ret)
}

func (cache *HashCacheBase) __insertNew(shorthash string, data interface{}) {
	cache.NewEntry(shorthash, data)
}

// finds entry and increases visit counter if found
func (cache *HashCacheBase) __find(shorthash string, ret *CacheEntry) bool {
	var found bool
	cache.ForEachSegment(func(seg ebuffer.ExpiringSegment) {
		if seg.(*cacheSegment).Find(shorthash, ret) {
			found = true
		}
	})
	return found
}

func (cache *HashCacheBase) Find(hash string, ret *CacheEntry) bool {
	cache.Lock()
	defer cache.Unlock()
	return cache.__find(cache.shortHash(hash), ret)
}

func (cache *HashCacheBase) __findWithDelete(shorthash string, ret *CacheEntry) bool {
	var found bool
	cache.ForEachSegment(func(seg ebuffer.ExpiringSegment) {
		if seg.(*cacheSegment).FindWithDelete(shorthash, ret) {
			found = true
		}
	})
	return found
}

// if seen, return entry and deletes it
func (cache *HashCacheBase) FindWithDelete(hash string, ret *CacheEntry) bool {
	cache.Lock()
	defer cache.Unlock()

	shash := cache.shortHash(hash)
	return cache.__findWithDelete(shash, ret)
}

func (cache *HashCacheBase) SeenHash(hash string, data interface{}, ret *CacheEntry) bool {
	cache.Lock()
	defer cache.Unlock()

	shash := cache.shortHash(hash)
	if seen := cache.__find(shash, ret); seen {
		return true
	}
	cache.__insertNew(shash, data)
	return false
}

type hashcacheStats struct {
	TxCount       int
	TxCountPassed int
	SeenOnce      int
	LatencySecAvg float64
	EarliestSeen  uint64
}

func (cache *HashCacheBase) Stats(msecBack uint64) *hashcacheStats {
	earliest := utils.UnixMsNow() - msecBack
	if msecBack == 0 {
		earliest = 0 // count all of it
	}
	ret := &hashcacheStats{EarliestSeen: utils.UnixMsNow()}
	var lat float64

	cache.ForEachEntry(func(entry *CacheEntry) {
		if entry.LastSeen >= earliest {
			ret.TxCount++
			if entry.Visits > 1 {
				lat = float64(entry.LastSeen-entry.FirstSeen) / 1000
				ret.LatencySecAvg += lat
			} else {
				ret.SeenOnce++
			}
			if entry.Visits >= cfg.Config.RepeatToAcceptTX {
				ret.TxCountPassed++
			}
			if entry.FirstSeen < ret.EarliestSeen {
				ret.EarliestSeen = entry.FirstSeen
			}
		}
	}, true)

	if ret.TxCountPassed != 0 {
		ret.LatencySecAvg = ret.LatencySecAvg / float64(ret.TxCountPassed)
	} else {
		ret.LatencySecAvg = 0
	}
	return ret
}

func (cache *HashCacheBase) ForEachEntry(callback func(entry *CacheEntry), lock bool) {
	if lock {
		cache.Lock()
		defer cache.Unlock()
	}
	earliest := utils.UnixMsNow() - cache.retentionPeriodMsCopy
	cache.ForEachSegment(func(s ebuffer.ExpiringSegment) {
		seg := s.(*cacheSegment)
		for _, entry := range seg.themap {
			if entry.LastSeen >= earliest {
				callback(&entry)
			}
		}
	})
}
