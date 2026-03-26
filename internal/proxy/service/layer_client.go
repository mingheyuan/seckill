package service

import (
	"bytes"
	"encoding/json"
	"net/http"
	"fmt"
	"time"
	"net/url"

	"seckill/internal/common/model"
)

type LayerClient struct {
	baseURL string
	client 	*http.Client
}

func NewLayerClient(baseURL string) *LayerClient {
	return &LayerClient{
		baseURL:baseURL,
		client:&http.Client{
			Timeout:2*time.Second,
		},
	}
}

func (lc *LayerClient) Seckill(req model.SeckillRequest) (model.InternalSeckillResponse,error) {
	var out model.InternalSeckillResponse

	b,_:=json.Marshal(req)
	resp,err :=lc.client.Post(lc.baseURL+"/internal/seckill","application/json",bytes.NewReader(b))
	if err !=nil {
		return out,err
	}
	defer resp.Body.Close()

	err =json.NewDecoder(resp.Body).Decode(&out)
	return out,err
}

func (lc *LayerClient) OrdersByUser(userID string) ([]model.SeckillRequest,error) {
	out :=struct {
		Code int 		`json:"code"`
		Orders []model.SeckillRequest `json:"orders"`
	}{}

	u :=lc.baseURL +"/internal/orders?user_id=" +url.QueryEscape(userID)
	resp,err:=lc.client.Get(u)
	if err !=nil {
		return nil,err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
        // 错误说明: 不检查状态码会把 404 HTML 当 JSON 解码，最后只看到模糊的 layer unavailable
        return nil, fmt.Errorf("layer orders status=%d", resp.StatusCode)
    }

	if err :=json.NewDecoder(resp.Body).Decode(&out);err!=nil {
		return nil,err
	}
	return out.Orders,nil
}