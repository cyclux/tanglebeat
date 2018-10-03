package main

import (
	"errors"
	"fmt"
	"github.com/lunfardo314/giota"
	"github.com/lunfardo314/tanglebeat/lib"
	"github.com/op/go-logging"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"
	"sync"
	"time"
)

type Sequence struct {
	Name          string
	Params        SenderParams
	IotaAPI       *giota.API
	IotaAPIgTTA   *giota.API
	IotaAPIaTT    *giota.API
	Seed          giota.Trytes
	TxTag         giota.Trytes
	TxTagPromote  giota.Trytes
	SecurityLevel int
	// addr cache
	addrIdxCache   int
	addrCache      giota.Address
	addrCacheMutex sync.Mutex
	log            *logging.Logger
}

func NewSequence(name string) (*Sequence, error) {
	params, err := GetSeqParams(name)
	if err != nil {
		return nil, err
	}
	var logger *logging.Logger
	if Config.Sender.LogConsoleOnly {
		logger = log
		log.Infof("Separate logger for the sequence won't be created")
	} else {
		formatter := getFormatter()
		logger, err = createChildLogger(name, &masterLoggingBackend, &formatter)
		if err != nil {
			return nil, err
		}
	}
	var ret = Sequence{
		Name:          name,
		Params:        params,
		SecurityLevel: 2,
		log:           logger,
		addrIdxCache:  -1,
	}
	ret.IotaAPI = giota.NewAPI(
		ret.Params.IOTANode[0],
		&http.Client{
			Timeout: time.Duration(ret.Params.TimeoutAPI) * time.Second,
		},
	)
	ret.log.Infof("IOTA node: %v, Timeout: %v sec", ret.Params.IOTANode[0], ret.Params.TimeoutAPI)

	ret.IotaAPIgTTA = giota.NewAPI(
		ret.Params.IOTANodeGTTA[0],
		&http.Client{
			Timeout: time.Duration(ret.Params.TimeoutGTTA) * time.Second,
		},
	)
	ret.log.Infof("IOTA node for gTTA: %v, Timeout: %v sec", ret.Params.IOTANodeGTTA[0], ret.Params.TimeoutGTTA)

	ret.IotaAPIaTT = giota.NewAPI(
		ret.Params.IOTANodeATT[0],
		&http.Client{
			Timeout: time.Duration(ret.Params.TimeoutATT) * time.Second,
		},
	)
	ret.log.Infof("IOTA node for ATT: %v, Timeout: %v sec", ret.Params.IOTANodeATT[0], ret.Params.TimeoutATT)

	ret.Seed, _ = giota.ToTrytes(ret.Params.Seed)
	ret.TxTag, _ = giota.ToTrytes(ret.Params.TxTag)
	ret.TxTagPromote, _ = giota.ToTrytes(ret.Params.TxTagPromote)

	uid, err := ret.GetUID()
	if err != nil {
		return nil, err
	}
	ret.log.Infof("Created sequence instance. UID = %v", uid)
	return &ret, nil
}

func (seq *Sequence) Run() {
	index0 := seq.getLastIndex()
	seq.log.Infof("Start running sequence with the index0 = %v", index0)

	for index := index0; ; index++ {
		seq.processAddrWithIndex(index)
		seq.saveIndex(index)
	}
}

func (seq *Sequence) processAddrWithIndex(index int) {
	seq.log.Debugf("Start processing idx=%v time=%v", index, time.Now().Unix())
	addr, err := seq.GetAddress(index)
	if err != nil {
		seq.log.Panicf("Can't get address for idx=%v", index)
	}
	inCh, cancelBalanceChan := seq.NewAddrBalanceChan(index)
	defer cancelBalanceChan()

	count := 0
	s := <-inCh
	if s.balance != 0 {
		// start sending routine if balance is non zero
		cancelSending := seq.StartSending(index)
		defer cancelSending()
	}

	// processAddrWithIndex only finishes if balance is 0 and address is spent
	// if balance = 0 and address is not spent, loop is waiting for the iotas
	for ; !(s.balance == 0 && s.isSpent); s = <-inCh {
		if count%12 == 0 {
			if s.balance == 0 {
				seq.log.Debugf("Address with idx=%v addr=%v.. has zero balance. Waiting for balance to become non zero",
					index, addr[:9])
			} else {
				seq.log.Debugf("CURRENT address with idx=%v addr=%v.. has balance %v iotas", index, addr[:9], s.balance)
			}
		}
		time.Sleep(5 * time.Second)
		count++
	}
	seq.log.Debugf("Finish processing idx=%v time=%v", index, time.Now().Unix())
}

func (seq *Sequence) GetAddress(index int) (giota.Address, error) {
	seq.addrCacheMutex.Lock()
	defer seq.addrCacheMutex.Unlock()

	if seq.addrIdxCache == index {
		return seq.addrCache, nil
	}
	var err error
	seq.addrCache, err = giota.NewAddress(seq.Seed, index, seq.SecurityLevel)
	if err == nil {
		seq.addrIdxCache = index
	} else {
		seq.addrIdxCache = -1
	}
	return seq.addrCache, nil
}

// returns last 12 trytes of the hash of the seed
func (seq *Sequence) GetUID() (string, error) {
	hash, err := lib.KerlTrytes(seq.Seed)
	if err != nil {
		return "", errors.New(fmt.Sprintf("%v: %v", seq.Name, err))
	}
	ret := string(hash)
	return ret[len(ret)-12:], nil
}

func (seq *Sequence) getLastIndexFname() (string, error) {
	uid, err := seq.GetUID()
	return path.Join(Config.SiteDataDir, uid), err
}

func (seq *Sequence) saveIndex(index int) error {
	fname, err := seq.getLastIndexFname()
	if err != nil {
		return err
	}
	fout, err := os.OpenFile(fname, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer fout.Close()
	if _, err = fout.WriteString(fmt.Sprintf("%v", index)); err == nil {
		seq.log.Debugf("Last idx %v saved to %v", index, fname)
	}

	return err
}

func (seq *Sequence) getLastIndex() int {
	fname, err := seq.getLastIndexFname()
	if err != nil {
		return seq.Params.Index0
	}
	b, err := ioutil.ReadFile(fname)
	if err != nil {
		return seq.Params.Index0
	}
	ret, err := strconv.Atoi(string(b))
	if err != nil {
		return seq.Params.Index0
	}

	ret = lib.Max(ret, seq.Params.Index0)
	seq.log.Debugf("Last idx %v read from %v", ret, fname)
	return ret
}

func (seq *Sequence) IsSpentAddr(address giota.Address) (bool, error) {
	if resp, err := seq.IotaAPI.WereAddressesSpentFrom([]giota.Address{address}); err != nil {
		return false, err
	} else {
		return resp.States[0], nil
	}
}

func (seq *Sequence) GetBalanceAddr(addresses []giota.Address) ([]int64, error) {
	if gbResp, err := seq.IotaAPI.GetBalances(addresses, 100); err != nil {
		return nil, err
	} else {
		return gbResp.Balances, nil
	}
}
