package infra

import (
	"log"
	"net/http"

	"github.com/tsumida/lunaship/utils"
)

var DEFAULT_PPROF_ADDR = ":6060"

func InitGopprof(
	addr string,
) error {

	go utils.Go(func() {
		log.Println(http.ListenAndServe(addr, nil))
	})

	return nil
}
