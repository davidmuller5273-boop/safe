package bsc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Block struct {
	Number    uint64
	Hash      string
	Timestamp time.Time
}

type Client struct {
	url        string
	httpClient *http.Client
}

func NewClient(url string) *Client {
	return &Client{url: url, httpClient: &http.Client{Timeout: 10 * time.Second}}
}

// FirstBlockAtOrAfter returns the first block whose timestamp is at least target.
func (c *Client) FirstBlockAtOrAfter(ctx context.Context, target time.Time) (Block, error) {
	latest, err := c.block(ctx, "latest")
	if err != nil {
		return Block{}, err
	}
	if latest.Timestamp.Before(target) {
		return Block{}, errors.New("目标时间的区块尚未产生")
	}

	high := latest.Number
	low := high
	step := uint64(128)
	for low > 0 {
		candidate := uint64(0)
		if low > step {
			candidate = low - step
		}
		block, err := c.block(ctx, hexNumber(candidate))
		if err != nil {
			return Block{}, err
		}
		if block.Timestamp.Before(target) || candidate == 0 {
			low = candidate
			break
		}
		low = candidate
		step *= 2
	}

	for low < high {
		mid := low + (high-low)/2
		block, err := c.block(ctx, hexNumber(mid))
		if err != nil {
			return Block{}, err
		}
		if block.Timestamp.Before(target) {
			low = mid + 1
		} else {
			high = mid
		}
	}
	return c.block(ctx, hexNumber(low))
}

func (c *Client) block(ctx context.Context, number string) (Block, error) {
	var response struct {
		Result *struct {
			Number    string `json:"number"`
			Hash      string `json:"hash"`
			Timestamp string `json:"timestamp"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := c.call(ctx, "eth_getBlockByNumber", []any{number, false}, &response); err != nil {
		return Block{}, err
	}
	if response.Error != nil {
		return Block{}, fmt.Errorf("BSC RPC %d: %s", response.Error.Code, response.Error.Message)
	}
	if response.Result == nil {
		return Block{}, errors.New("BSC RPC 未返回区块")
	}
	blockNumber, err := parseHex(response.Result.Number)
	if err != nil {
		return Block{}, fmt.Errorf("解析区块高度失败: %w", err)
	}
	timestamp, err := parseHex(response.Result.Timestamp)
	if err != nil {
		return Block{}, fmt.Errorf("解析区块时间失败: %w", err)
	}
	return Block{Number: blockNumber, Hash: response.Result.Hash, Timestamp: time.Unix(int64(timestamp), 0).UTC()}, nil
}

func (c *Client) call(ctx context.Context, method string, params []any, result any) error {
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method, "params": params, "id": 1})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("BSC RPC HTTP 状态码 %d", response.StatusCode)
	}
	return json.NewDecoder(response.Body).Decode(result)
}

func parseHex(value string) (uint64, error) {
	return strconv.ParseUint(strings.TrimPrefix(value, "0x"), 16, 64)
}

func hexNumber(value uint64) string { return fmt.Sprintf("0x%x", value) }
