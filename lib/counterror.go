package lib

import "github.com/iotaledger/iota.go/api"

type ErrorCounter interface {
	CountError(api *api.API, err error) bool
}
