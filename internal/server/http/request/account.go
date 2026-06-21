package request

// BizType 业务账户类型。
type BizType string

const (
	// BizTypeSpot 现货账户。
	BizTypeSpot BizType = "SPOT"

	// BizTypeLever 杠杆账户。
	BizTypeLever BizType = "LEVER"

	// BizTypeFinance 理财账户。
	BizTypeFinance BizType = "FINANCE"

	// BizTypeFuturesU USDT 本位合约账户。
	BizTypeFuturesU BizType = "FUTURES_U"

	// BizTypeFuturesC 币本位合约账户。
	BizTypeFuturesC BizType = "FUTURES_C"
)

// BalanceAccountTransferReq 现货-母子账号资金划转请求。
// 对应接口：POST /v4/balance/account/transfer
type BalanceAccountTransferReq struct {
	AppKey    string `json:"public_key" form:"public_key" binding:"required"`
	SecretKey string `json:"secret_key" form:"secret_key" binding:"required"`
	// BizID 用于幂等处理的唯一 ID，最大长度 128。
	BizID string `json:"bizId" form:"bizId" binding:"required,max=128"`

	// From 资金转出账户，取值见 BizType。
	From BizType `json:"from" form:"from" binding:"required,oneof=SPOT LEVER FINANCE FUTURES_U FUTURES_C"`

	// To 资金转入账户，取值见 BizType。
	To BizType `json:"to" form:"to" binding:"required,oneof=SPOT LEVER FINANCE FUTURES_U FUTURES_C"`

	// Currency 币种名称，必须小写，例如 usdt、btc。
	Currency string `json:"currency" form:"currency" binding:"required,lowercase,max=32"`

	// Symbol 划转交易对，必须小写。
	// 如果转入或转出账户中有一个是杠杆账户（LEVER），则必填。
	Symbol string `json:"symbol,omitempty" form:"symbol" binding:"omitempty,lowercase,max=64"`

	// Amount 划转金额。
	// 这里建议用 string 接收，避免 bigDecimal 精度问题。
	Amount int64 `json:"amount" form:"amount" binding:"required"`

	// ToAccountID 转入账户 ID，必须与转出账户 ID 属于同一用户。
	ToAccountID int64 `json:"toAccountId" form:"toAccountId" binding:"required"`

	// FromAccountID 转出账户 ID。
	FromAccountID int64 `json:"fromAccountId,omitempty" form:"fromAccountId"`
}

type BalanceAccountTransferBody struct {
	BizID string `json:"bizId"`

	From BizType `json:"from"`

	To BizType `json:"to"`

	Currency string `json:"currency"`

	Symbol string `json:"symbol,omitempty"`

	Amount int64 `json:"amount"`

	ToAccountID int64 `json:"toAccountId"`

	FromAccountID int64 `json:"fromAccountId,omitempty"`
}

// BalanceTransferReq 基础资金划转请求
// 对应接口：POST /v4/balance/transfer
type BalanceTransferReq struct {
	AppKey    string `json:"public_key" form:"public_key" binding:"required"`
	SecretKey string `json:"secret_key" form:"secret_key" binding:"required"`
	// 幂等ID
	BizID string `json:"bizId" form:"bizId" binding:"required,max=128"`

	// 转出账户
	From BizType `json:"from" form:"from" binding:"required,oneof=SPOT LEVER FINANCE FUTURES_U FUTURES_C"`

	// 转入账户
	To BizType `json:"to" form:"to" binding:"required,oneof=SPOT LEVER FINANCE FUTURES_U FUTURES_C"`

	// 币种（必须小写）
	Currency string `json:"currency" form:"currency" binding:"required,max=32"`

	// 交易对（杠杆账户必填）
	Symbol string `json:"symbol,omitempty" form:"symbol" binding:"omitempty,max=64"`

	// 金额
	Amount int64 `json:"amount" form:"amount" binding:"required"`
}

type BalanceTransferBody struct {
	// 幂等ID
	BizID string `json:"bizId" form:"bizId" binding:"required,max=128"`

	// 转出账户
	From BizType `json:"from" form:"from" binding:"required,oneof=SPOT LEVER FINANCE FUTURES_U FUTURES_C"`

	// 转入账户
	To BizType `json:"to" form:"to" binding:"required,oneof=SPOT LEVER FINANCE FUTURES_U FUTURES_C"`

	// 币种（必须小写）
	Currency string `json:"currency" form:"currency" binding:"required,max=32"`

	// 交易对（杠杆账户必填）
	Symbol string `json:"symbol,omitempty" form:"symbol" binding:"omitempty,max=64"`

	// 金额
	Amount int64 `json:"amount" form:"amount" binding:"required"`
}

// BalanceTransferResp 现货-母子账号资金划转响应。
// result 为划转唯一 ID，建议保存用于对账。
type BalanceTransferResp struct {
	RC     int      `json:"rc"`
	MC     string   `json:"mc"`
	MA     []string `json:"ma"`
	Result int64    `json:"result"`
}
