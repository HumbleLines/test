package request

// CreateUserReq 用于绑定/校验 HTTP JSON 请求
type CreateUserReq struct {
	Name  string   `json:"name"  form:"name"  binding:"required,min=2,max=50"`
	Age   int      `json:"age"   form:"age"   binding:"required,gte=0,lte=150"`
	Email string   `json:"email" form:"email" binding:"required,email"`
	Tags  []string `json:"tags"  form:"tags"  binding:"max=10,dive,max=20"`
}

type QueryUserReq struct {
	Key    string `json:"key" form:"key" binding:"required,min=2,max=50"`
	Expire int    `json:"expire" form:"expire" binding:"required,gte=0"`
}
