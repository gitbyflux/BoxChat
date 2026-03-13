package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type GiphyService struct {
	apiKey string
	baseURL string
}

type GiphyImage struct {
	URL     string `json:"url"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Size    string `json:"size"`
}

type GiphyImages struct {
	Original         GiphyImage `json:"original"`
	FixedWidth       GiphyImage `json:"fixed_width"`
	FixedWidthSmall  GiphyImage `json:"fixed_width_small"`
	PreviewGif       GiphyImage `json:"preview_gif"`
	Downsized        GiphyImage `json:"downsized"`
}

type GiphyItem struct {
	ID     string      `json:"id"`
	Title  string      `json:"title"`
	Images GiphyImages `json:"images"`
	URL    string      `json:"url"`
}

type GiphyPagination struct {
	TotalCount  int `json:"total_count"`
	Count       int `json:"count"`
	Offset      int `json:"offset"`
}

type GiphyResponse struct {
	Data       []GiphyItem    `json:"data"`
	Pagination GiphyPagination `json:"pagination"`
	Meta       GiphyMeta      `json:"meta"`
}

type GiphyMeta struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
}

func NewGiphyService(apiKey string) *GiphyService {
	return &GiphyService{
		apiKey:  apiKey,
		baseURL: "https://api.giphy.com/v1",
	}
}

func (s *GiphyService) GetTrendingGifs(limit, offset int) (*GiphyResponse, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("GIPHY_API_KEY is not configured")
	}
	
	if limit <= 0 {
		limit = 24
	}
	if limit > 50 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	
	params := url.Values{}
	params.Set("api_key", s.apiKey)
	params.Set("limit", strconv.Itoa(limit))
	params.Set("offset", strconv.Itoa(offset))
	params.Set("rating", "pg-13")
	
	reqURL := fmt.Sprintf("%s/gifs/trending?%s", s.baseURL, params.Encode())
	
	resp, err := http.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch trending gifs: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var giphyResp GiphyResponse
	if err := json.Unmarshal(body, &giphyResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	return &giphyResp, nil
}

func (s *GiphyService) SearchGifs(q string, limit, offset int) (*GiphyResponse, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("GIPHY_API_KEY is not configured")
	}
	
	if q == "" {
		return &GiphyResponse{
			Data: []GiphyItem{},
			Pagination: GiphyPagination{
				TotalCount: 0,
				Count:      0,
				Offset:     0,
			},
		}, nil
	}
	
	if limit <= 0 {
		limit = 24
	}
	if limit > 50 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	
	params := url.Values{}
	params.Set("api_key", s.apiKey)
	params.Set("q", q)
	params.Set("limit", strconv.Itoa(limit))
	params.Set("offset", strconv.Itoa(offset))
	params.Set("rating", "pg-13")
	
	reqURL := fmt.Sprintf("%s/gifs/search?%s", s.baseURL, params.Encode())
	
	resp, err := http.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("failed to search gifs: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var giphyResp GiphyResponse
	if err := json.Unmarshal(body, &giphyResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	return &giphyResp, nil
}

// GetGifByID retrieves a specific GIF by its ID
func (s *GiphyService) GetGifByID(gifID string) (*GiphyItem, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("GIPHY_API_KEY is not configured")
	}
	
	params := url.Values{}
	params.Set("api_key", s.apiKey)
	
	reqURL := fmt.Sprintf("%s/gifs/%s?%s", s.baseURL, gifID, params.Encode())
	
	resp, err := http.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch gif: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var giphyResp struct {
		Data GiphyItem `json:"data"`
		Meta GiphyMeta `json:"meta"`
	}
	if err := json.Unmarshal(body, &giphyResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	if giphyResp.Meta.Status != 200 {
		return nil, fmt.Errorf("gif not found")
	}
	
	return &giphyResp.Data, nil
}

// MapGiphyItemToResponse converts GiphyItem to a simpler response format
func MapGiphyItemToResponse(item *GiphyItem) map[string]interface{} {
	if item == nil {
		return nil
	}
	
	result := map[string]interface{}{
		"id":    item.ID,
		"title": item.Title,
		"url":   item.Images.Original.URL,
	}
	
	// Get preview URL
	previewURL := item.Images.FixedWidthSmall.URL
	if previewURL == "" {
		previewURL = item.Images.PreviewGif.URL
	}
	if previewURL == "" {
		previewURL = item.Images.Original.URL
	}
	
	result["preview"] = previewURL
	
	return result
}

// MapGiphyResponse converts GiphyResponse to a simpler response format
func MapGiphyResponse(resp *GiphyResponse) map[string]interface{} {
	if resp == nil {
		return map[string]interface{}{
			"gifs": []interface{}{},
			"pagination": map[string]interface{}{
				"offset":      0,
				"limit":       0,
				"total":       0,
				"next_offset": 0,
			},
		}
	}
	
	gifs := make([]interface{}, 0)
	for _, item := range resp.Data {
		if mapped := MapGiphyItemToResponse(&item); mapped != nil {
			gifs = append(gifs, mapped)
		}
	}
	
	nextOffset := resp.Pagination.Offset + resp.Pagination.Count
	
	return map[string]interface{}{
		"gifs": gifs,
		"pagination": map[string]interface{}{
			"offset":      resp.Pagination.Offset,
			"limit":       resp.Pagination.Count,
			"total":       resp.Pagination.TotalCount,
			"next_offset": nextOffset,
		},
	}
}
