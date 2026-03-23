package model

type SeckillRequest struct {
	UserID 		string 	`json:"user_id" binding:"required"`
	ActivityID 	int64 	`json:"activity_id" binding:"required"` 
}

type SeckillResponse struct {
	Code 	int 	`json:"code"`
	Message string 	`json:"message"`
}

type InternalSeckillResponse struct {
	OK 		bool 	`json:"ok"`
	Message string 	`json:"message"`
}