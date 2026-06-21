package siristarts

import (
	"trade-gateway/internal/server/http/request"
	"trade-gateway/internal/server/http/response"
	"trade-gateway/internal/server/http/validator"

	"github.com/gin-gonic/gin"
)

func (h *Handlers) BalanceAccountTransfer(c *gin.Context) {
	var req request.BalanceAccountTransferReq
	if ok := validator.Bind(c, &req); !ok {
		return
	}
	resp, err := h.S.Account.BalanceAccountTransfer(c.Request.Context(), req)
	if err != nil {
		response.Internal(c, "db down", gin.H{"error": err.Error()})
		return
	}
	response.OK(c, resp)
}

func (h *Handlers) BalanceTransfer(c *gin.Context) {
	var req request.BalanceTransferReq
	if ok := validator.Bind(c, &req); !ok {
		return
	}
	resp, err := h.S.Account.BalanceTransfer(c.Request.Context(), req)
	if err != nil {
		response.Internal(c, "db down", gin.H{"error": err.Error()})
		return
	}
	response.OK(c, resp)
}
