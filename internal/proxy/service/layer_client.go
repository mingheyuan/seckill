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
			Timeout:10*time.Second,
			Transport:&http.Transport{
				MaxIdleConns:        200,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

func (lc *LayerClient) Seckill(req model.SeckillRequest) (model.InternalSeckillResponse,error) {
	var out model.InternalSeckillResponse

	b,_:=json.Marshal(req)
	resp,err :=lc.client.Post(lc.baseURL+"/internal/seckill","application/json",bytes.NewReader(b))
	if err !=nil {
		return out,fmt.Errorf("layer=%s seckill post failed: %w", lc.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict && resp.StatusCode != http.StatusTooManyRequests {
		return out, fmt.Errorf("layer=%s seckill status=%d", lc.baseURL, resp.StatusCode)
	}

	err =json.NewDecoder(resp.Body).Decode(&out)
	if err != nil {
		return out, fmt.Errorf("layer=%s seckill decode failed: %w", lc.baseURL, err)
	}
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
		return nil,fmt.Errorf("layer=%s orders get failed: %w", lc.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
	// 错误说明: 不检查状态码会把 404 HTML 当 JSON 解码，最后只看到模糊的 layer unavailable
	return nil, fmt.Errorf("layer=%s orders status=%d", lc.baseURL, resp.StatusCode)
    }

	if err :=json.NewDecoder(resp.Body).Decode(&out);err!=nil {
		return nil,err
	}
	return out.Orders,nil
}