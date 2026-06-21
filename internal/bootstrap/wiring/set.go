package wiring

import (
	"trade-gateway/internal/app"
	"trade-gateway/internal/services/siristarts"
)

// Set 汇总项目中用到的业务服务，启动时构造一次，handler 里复用
type Set struct {
	Account *siristarts.AccountService
}

func NewSet(deps app.AppDepend) *Set {
	return &Set{
		Account: siristarts.NewCHTraceService(deps),
	}
}
