package main

import (
	"sync"
	"time"
)

type mapSegment struct {
	themap  map[string]struct{}
	created time.Time
	latest  time.Time
	next    *mapSegment
}

var txmap *mapSegment
var mutexTX = &sync.Mutex{}

var snmap *mapSegment

var mutexSN = &sync.Mutex{}

const segmentDuration = 1 * time.Minute
const retainPeriod = 3 * time.Minute

func __seenBefore(top *mapSegment, hash string) (bool, time.Time) {
	// assert themap != nil
	for ; top != nil; top = top.next {
		if _, ok := top.themap[hash]; ok {
			return true, top.latest
		}
	}
	return false, time.Time{}
}

func insertNew(hash string, ptop **mapSegment) {
	if *ptop == nil || len((*ptop).themap) != 0 && time.Since((*ptop).created) > segmentDuration {
		*ptop = &mapSegment{
			themap:  make(map[string]struct{}),
			next:    txmap,
			created: time.Now(),
		}
	}
	(*ptop).themap[hash] = struct{}{}
	(*ptop).latest = time.Now()
}

func seenBeforeTX(hash string) (bool, time.Time) {
	mutexTX.Lock()
	defer mutexTX.Unlock()

	if seen, before := __seenBefore(txmap, hash); seen {
		return true, before
	}
	insertNew(hash, &txmap)
	return false, time.Time{}
}

func seenBeforeSN(hash string) (bool, time.Time) {
	mutexSN.Lock()
	defer mutexSN.Unlock()

	if seen, before := __seenBefore(snmap, hash); seen {
		return true, before
	}
	insertNew(hash, &snmap)
	return false, time.Time{}
}

func purge(top *mapSegment) {
	for ; top != nil; top = top.next {
		if top.next != nil && time.Since(top.next.latest) > retainPeriod {
			s, t := size(top.next)
			debugf("---- removing segments = %d transactions = %d", s, t)
			top.next = nil // cut the tail
		}
	}
}

func sizeTX() (int, int) {
	mutexTX.Lock()
	defer mutexTX.Unlock()
	return size(txmap)
}

func sizeSN() (int, int) {
	mutexSN.Lock()
	defer mutexSN.Unlock()
	return size(snmap)
}

func size(top *mapSegment) (int, int) {
	var numseg int
	var numtx int

	for ; top != nil; top = top.next {
		numseg += 1
		numtx += len(top.themap)
	}
	return numseg, numtx
}

func init() {
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			mutexTX.Lock()
			purge(txmap)
			mutexTX.Unlock()

			mutexSN.Lock()
			purge(snmap)
			mutexSN.Unlock()
		}
	}()
}