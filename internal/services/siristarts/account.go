package siristarts

import (
	"context"
	"encoding/json"
	"net/http"
	"trade-gateway/internal/app"
	"trade-gateway/internal/consts"
	ihttp "trade-gateway/internal/infra/client/http"
	spotsigner "trade-gateway/internal/pkg/signer/siristarts"
	"trade-gateway/internal/server/http/request"
)

type AccountService struct {
	deps       app.AppDepend
	ClientName string // ClientName 实例名，默认 siristars
}

func NewCHTraceService(deps app.AppDepend) *AccountService {
	return &AccountService{deps: deps, ClientName: "siristars"}
}

// BalanceAccountTransfer 母子账号直接互转 ACCOUNT A SPOT -> ACCOUNT B SPOT
func (s *AccountService) BalanceAccountTransfer(ctx context.Context, req request.BalanceAccountTransferReq) (any, error) {
	body := request.BalanceAccountTransferBody{
		BizID:         req.BizID,
		From:          req.From,
		To:            req.To,
		Currency:      req.Currency,
		Symbol:        req.Symbol,
		Amount:        req.Amount,
		ToAccountID:   req.ToAccountID,
		FromAccountID: req.FromAccountID,
	}
	jsonReq, _ := json.Marshal(body)
	res, err := spotsigner.BuildHeaders(spotsigner.BuildHeadersInput{
		Method:      http.MethodPost,
		Path:        consts.SpotBalanceAccountTransferURI,
		Query:       nil,
		ContentType: consts.MIMEJSON,
		Body:        jsonReq,
		AppKey:      req.AppKey,
		SecretKey:   req.SecretKey,
		RecvWindow:  consts.RecvWindow,
		Algorithm:   consts.Algorithm,
	})
	if err != nil {
		return nil, err
	}

	headers := res.Headers
	client, _ := s.deps.HTTPClient(s.ClientName)
	URL := s.deps.Config().Exchange.Siristars.SpotHost + consts.SpotBalanceAccountTransferURI
	resp, err := client.Do(ctx, http.MethodPost, URL, nil, body, headers, ihttp.CTJSON)
	if err != nil {
		return nil, err
	}
	var respStruct request.BalanceTransferResp
	err = json.Unmarshal(resp.Body, &respStruct)
	if err != nil {
		return nil, err
	}
	return &respStruct, nil
}

// BalanceTransfer 单账号 业务划转 SPOT->FUTURES  FUTURES->SPOT
func (s *AccountService) BalanceTransfer(ctx context.Context, req request.BalanceTransferReq) (any, error) {
	body := request.BalanceTransferBody{
		BizID:    req.BizID,
		From:     req.From,
		To:       req.To,
		Currency: req.Currency,
		Symbol:   req.Symbol,
		Amount:   req.Amount,
	}
	jsonReq, _ := json.Marshal(body)
	res, err := spotsigner.BuildHeaders(spotsigner.BuildHeadersInput{
		Method:      http.MethodPost,
		Path:        consts.SpotBalanceTransferURI,
		Query:       nil,
		ContentType: consts.MIMEJSON,
		Body:        jsonReq,
		AppKey:      req.AppKey,
		SecretKey:   req.SecretKey,
		RecvWindow:  consts.RecvWindow,
		Algorithm:   consts.Algorithm,
	})
	if err != nil {
		return nil, err
	}

	headers := res.Headers
	client, _ := s.deps.HTTPClient(s.ClientName)
	URL := s.deps.Config().Exchange.Siristars.SpotHost + consts.SpotBalanceTransferURI
	resp, err := client.Do(ctx, http.MethodPost, URL, nil, body, headers, ihttp.CTJSON)
	if err != nil {
		return nil, err
	}
	var respStruct request.BalanceTransferResp
	err = json.Unmarshal(resp.Body, &respStruct)
	if err != nil {
		return nil, err
	}
	return &respStruct, nil
}
