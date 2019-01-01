package bufferwe

import (
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"sync"
	"time"
)

// BufferWE = Buffer With Expiration
type bufferweSegment struct {
	created      uint64
	lastModified uint64
	ts           []uint64
	data         []interface{}
	next         *bufferweSegment
}

type BufferWE struct {
	id                string
	withData          bool
	segmentDurationMs uint64
	retentionPeriodMs uint64
	top               *bufferweSegment
	mutex             *sync.Mutex
}

func (buf *BufferWE) Lock() {
	buf.mutex.Lock()
}

func (buf *BufferWE) Unlock() {
	buf.mutex.Unlock()
}

func NewBufferWE(withData bool, segmentDurationSec int, retentionPeriodSec int) *BufferWE {
	ret := &BufferWE{
		withData:          withData,
		segmentDurationMs: uint64(segmentDurationSec * 1000),
		retentionPeriodMs: uint64(retentionPeriodSec * 1000),
		mutex:             &sync.Mutex{},
	}
	return ret
}

func (buf *BufferWE) Reset() {
	if buf != nil {
		buf.Lock()
		buf.top = nil
		buf.Unlock()
	}
}

// returns false if empty
func (buf *BufferWE) purge() bool {
	if buf == nil {
		return false
	}
	buf.Lock()
	defer buf.Unlock()

	if buf.top == nil {
		return false
	}
	earliest := utils.UnixMsNow() - uint64(buf.retentionPeriodMs)
	if buf.top.lastModified < earliest {
		buf.top = nil
		return false
	}
	for s := buf.top; s != nil; s = s.next {
		if s.next != nil && s.next.lastModified < earliest {
			s.next = nil
		}
	}
	return true
}

// loop exits when buf becomes empty
func (buf *BufferWE) purgeLoop() {
	for buf.purge() {
		time.Sleep(10 * time.Second)
	}
}

func (buf *BufferWE) Push(data interface{}) {
	if buf == nil {
		return
	}
	buf.Lock()
	defer buf.Unlock()
	buf.Push__(data)
}

func (buf *BufferWE) Push__(data interface{}) {
	nowis := utils.UnixMsNow()
	if buf.top == nil || nowis-buf.top.created > buf.segmentDurationMs {
		capacity := 100
		if buf.top != nil {
			capacity = len(buf.top.ts)
			capacity += capacity / 20 // 5% more than the last
		}
		var dataArray []interface{}
		if buf.withData {
			dataArray = make([]interface{}, 0, capacity)
		}
		startPurge := buf.top == nil
		buf.top = &bufferweSegment{
			created:      nowis,
			lastModified: nowis,
			ts:           make([]uint64, 0, capacity),
			data:         dataArray,
			next:         buf.top,
		}
		if startPurge {
			// the purge goroutine will be started upon first push and will exit when purged uo to empty buffer
			go buf.purgeLoop()
		}
	}
	buf.top.ts = append(buf.top.ts, nowis)
	buf.top.lastModified = nowis
	if buf.withData {
		buf.top.data = append(buf.top.data, data)
	}
}

func (buf *BufferWE) Last() (uint64, interface{}) {
	if buf == nil {
		return 0, nil
	}
	buf.Lock()
	defer buf.Unlock()

	if buf.top == nil {
		return 0, nil
	}
	earliest := utils.UnixMsNow() - buf.retentionPeriodMs
	idx := len(buf.top.ts) - 1 // must be > 0, because buf.top != nil
	retts := buf.top.ts[idx]
	if retts < earliest {
		// last one is out of retention period
		return 0, nil
	}
	if buf.withData {
		return retts, buf.top.data[idx]
	}
	return retts, nil
}

func (buf *BufferWE) TouchLast__() {
	if buf.top == nil || len(buf.top.ts) == 0 {
		return
	}
	buf.top.lastModified = utils.UnixMsNow()
}

func (buf *BufferWE) ForEach(callback func(data interface{}) bool) {
	if buf == nil {
		return
	}
	buf.Lock()
	defer buf.Unlock()

	earliest := utils.UnixMsNow() - uint64(buf.retentionPeriodMs)
	for s := buf.top; s != nil; s = s.next {
		if s.created < earliest {
			return
		}
		var exit bool
		for idx := range s.ts {
			if s.ts[idx] >= earliest {
				if buf.withData {
					exit = callback(s.data[idx])
				} else {
					exit = callback(nil)
				}
				if exit {
					return
				}
			}
		}
	}
}

func (buf *BufferWE) CountAll() int {
	var ret int
	buf.ForEach(func(data interface{}) bool {
		ret++
		return false
	})
	return ret
}
