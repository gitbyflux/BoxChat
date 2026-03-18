package services

import (
	"testing"
)

// ============================================================================
// NewGiphyService Tests
// ============================================================================

func TestNewGiphyService(t *testing.T) {
	apiKey := "test_api_key"
	service := NewGiphyService(apiKey)

	if service == nil {
		t.Fatal("NewGiphyService() returned nil")
	}
	if service.apiKey != apiKey {
		t.Errorf("NewGiphyService() apiKey = %s, want %s", service.apiKey, apiKey)
	}
	if service.baseURL != "https://api.giphy.com/v1" {
		t.Errorf("NewGiphyService() baseURL = %s, want https://api.giphy.com/v1", service.baseURL)
	}
}

func TestNewGiphyService_EmptyAPIKey(t *testing.T) {
	service := NewGiphyService("")

	if service == nil {
		t.Fatal("NewGiphyService() returned nil")
	}
	if service.apiKey != "" {
		t.Errorf("NewGiphyService() apiKey = %s, want empty string", service.apiKey)
	}
}

// ============================================================================
// GetTrendingGifs Tests - Validation
// ============================================================================

func TestGiphyService_GetTrendingGifs_NoAPIKey(t *testing.T) {
	service := NewGiphyService("")

	_, err := service.GetTrendingGifs(10, 0)
	if err == nil {
		t.Error("GetTrendingGifs() should return error when API key is empty")
	}
	if err.Error() != "GIPHY_API_KEY is not configured" {
		t.Errorf("GetTrendingGifs() error = %v, want 'GIPHY_API_KEY is not configured'", err)
	}
}

func TestGiphyService_GetTrendingGifs_LimitValidation(t *testing.T) {
	service := NewGiphyService("test_key")

	// Test with limit <= 0, should default to 24
	_, err := service.GetTrendingGifs(0, 0)
	if err != nil {
		t.Errorf("GetTrendingGifs(0, 0) should not return error, got %v", err)
	}

	// Test with limit > 50, should cap at 50
	_, err = service.GetTrendingGifs(100, 0)
	if err != nil {
		t.Errorf("GetTrendingGifs(100, 0) should not return error, got %v", err)
	}

	// Test with negative offset, should default to 0
	_, err = service.GetTrendingGifs(10, -5)
	if err != nil {
		t.Errorf("GetTrendingGifs(10, -5) should not return error, got %v", err)
	}
}

// ============================================================================
// SearchGifs Tests - Validation
// ============================================================================

func TestGiphyService_SearchGifs_NoAPIKey(t *testing.T) {
	service := NewGiphyService("")

	_, err := service.SearchGifs("test", 10, 0)
	if err == nil {
		t.Error("SearchGifs() should return error when API key is empty")
	}
	if err.Error() != "GIPHY_API_KEY is not configured" {
		t.Errorf("SearchGifs() error = %v, want 'GIPHY_API_KEY is not configured'", err)
	}
}

func TestGiphyService_SearchGifs_EmptyQuery(t *testing.T) {
	service := NewGiphyService("test_key")

	result, err := service.SearchGifs("", 10, 0)
	if err != nil {
		t.Errorf("SearchGifs(\"\") should not return error, got %v", err)
	}
	if result == nil {
		t.Fatal("SearchGifs(\"\") should return empty result, not nil")
	}
	if len(result.Data) != 0 {
		t.Errorf("SearchGifs(\"\") should return empty data, got %d items", len(result.Data))
	}
}

func TestGiphyService_SearchGifs_LimitValidation(t *testing.T) {
	service := NewGiphyService("test_key")

	// Test with limit <= 0, should default to 24
	_, err := service.SearchGifs("test", 0, 0)
	if err != nil {
		t.Errorf("SearchGifs(\"test\", 0, 0) should not return error, got %v", err)
	}

	// Test with limit > 50, should cap at 50
	_, err = service.SearchGifs("test", 100, 0)
	if err != nil {
		t.Errorf("SearchGifs(\"test\", 100, 0) should not return error, got %v", err)
	}
}

// ============================================================================
// GetGifByID Tests - Validation
// ============================================================================

func TestGiphyService_GetGifByID_NoAPIKey(t *testing.T) {
	service := NewGiphyService("")

	_, err := service.GetGifByID("test_id")
	if err == nil {
		t.Error("GetGifByID() should return error when API key is empty")
	}
	if err.Error() != "GIPHY_API_KEY is not configured" {
		t.Errorf("GetGifByID() error = %v, want 'GIPHY_API_KEY is not configured'", err)
	}
}

// ============================================================================
// MapGiphyItemToResponse Tests
// ============================================================================

func TestMapGiphyItemToResponse_Nil(t *testing.T) {
	result := MapGiphyItemToResponse(nil)
	if result != nil {
		t.Errorf("MapGiphyItemToResponse(nil) = %v, want nil", result)
	}
}

func TestMapGiphyItemToResponse_Valid(t *testing.T) {
	item := &GiphyItem{
		ID:    "test123",
		Title: "Test GIF",
		URL:   "https://giphy.com/test123",
		Images: GiphyImages{
			Original: GiphyImage{
				URL:    "https://media.giphy.com/test.gif",
				Width:  480,
				Height: 360,
				Size:   "1234567",
			},
			FixedWidthSmall: GiphyImage{
				URL: "https://media.giphy.com/test_small.gif",
			},
			PreviewGif: GiphyImage{
				URL: "https://media.giphy.com/test_preview.gif",
			},
		},
	}

	result := MapGiphyItemToResponse(item)
	if result == nil {
		t.Fatal("MapGiphyItemToResponse() returned nil")
	}

	if result["id"] != "test123" {
		t.Errorf("MapGiphyItemToResponse() id = %v, want test123", result["id"])
	}
	if result["title"] != "Test GIF" {
		t.Errorf("MapGiphyItemToResponse() title = %v, want Test GIF", result["title"])
	}
	if result["url"] != "https://media.giphy.com/test.gif" {
		t.Errorf("MapGiphyItemToResponse() url = %v, want https://media.giphy.com/test.gif", result["url"])
	}
	if result["preview"] != "https://media.giphy.com/test_small.gif" {
		t.Errorf("MapGiphyItemToResponse() preview = %v, want https://media.giphy.com/test_small.gif", result["preview"])
	}
}

func TestMapGiphyItemToResponse_FallbackToPreviewGif(t *testing.T) {
	item := &GiphyItem{
		ID:    "test456",
		Title: "Test GIF 2",
		URL:   "https://giphy.com/test456",
		Images: GiphyImages{
			Original: GiphyImage{
				URL: "https://media.giphy.com/test2.gif",
			},
			FixedWidthSmall: GiphyImage{
				URL: "", // Empty
			},
			PreviewGif: GiphyImage{
				URL: "https://media.giphy.com/test2_preview.gif",
			},
		},
	}

	result := MapGiphyItemToResponse(item)
	if result == nil {
		t.Fatal("MapGiphyItemToResponse() returned nil")
	}

	if result["preview"] != "https://media.giphy.com/test2_preview.gif" {
		t.Errorf("MapGiphyItemToResponse() preview should fallback to PreviewGif")
	}
}

func TestMapGiphyItemToResponse_FallbackToOriginal(t *testing.T) {
	item := &GiphyItem{
		ID:    "test789",
		Title: "Test GIF 3",
		URL:   "https://giphy.com/test789",
		Images: GiphyImages{
			Original: GiphyImage{
				URL: "https://media.giphy.com/test3.gif",
			},
			FixedWidthSmall: GiphyImage{
				URL: "", // Empty
			},
			PreviewGif: GiphyImage{
				URL: "", // Empty
			},
		},
	}

	result := MapGiphyItemToResponse(item)
	if result == nil {
		t.Fatal("MapGiphyItemToResponse() returned nil")
	}

	if result["preview"] != "https://media.giphy.com/test3.gif" {
		t.Errorf("MapGiphyItemToResponse() preview should fallback to Original")
	}
}

// ============================================================================
// MapGiphyResponse Tests
// ============================================================================

func TestMapGiphyResponse_Nil(t *testing.T) {
	result := MapGiphyResponse(nil)
	if result == nil {
		t.Fatal("MapGiphyResponse(nil) should return empty response, not nil")
	}

	gifs, ok := result["gifs"].([]interface{})
	if !ok {
		t.Fatal("MapGiphyResponse(nil) gifs should be []interface{}")
	}
	if len(gifs) != 0 {
		t.Errorf("MapGiphyResponse(nil) should return empty gifs, got %d", len(gifs))
	}

	pagination, ok := result["pagination"].(map[string]interface{})
	if !ok {
		t.Fatal("MapGiphyResponse(nil) pagination should be map[string]interface{}")
	}
	if pagination["offset"] != 0 || pagination["limit"] != 0 || pagination["total"] != 0 {
		t.Error("MapGiphyResponse(nil) pagination should have all zeros")
	}
}

func TestMapGiphyResponse_Valid(t *testing.T) {
	response := &GiphyResponse{
		Data: []GiphyItem{
			{
				ID:    "gif1",
				Title: "GIF 1",
				Images: GiphyImages{
					Original: GiphyImage{URL: "https://media.giphy.com/gif1.gif"},
					FixedWidthSmall: GiphyImage{URL: "https://media.giphy.com/gif1_small.gif"},
				},
			},
			{
				ID:    "gif2",
				Title: "GIF 2",
				Images: GiphyImages{
					Original: GiphyImage{URL: "https://media.giphy.com/gif2.gif"},
					FixedWidthSmall: GiphyImage{URL: "https://media.giphy.com/gif2_small.gif"},
				},
			},
		},
		Pagination: GiphyPagination{
			TotalCount: 100,
			Count:      2,
			Offset:     0,
		},
	}

	result := MapGiphyResponse(response)
	if result == nil {
		t.Fatal("MapGiphyResponse() returned nil")
	}

	gifs, ok := result["gifs"].([]interface{})
	if !ok {
		t.Fatal("MapGiphyResponse() gifs should be []interface{}")
	}
	if len(gifs) != 2 {
		t.Errorf("MapGiphyResponse() should return 2 gifs, got %d", len(gifs))
	}

	pagination, ok := result["pagination"].(map[string]interface{})
	if !ok {
		t.Fatal("MapGiphyResponse() pagination should be map[string]interface{}")
	}
	if pagination["offset"] != 0 {
		t.Errorf("MapGiphyResponse() pagination offset = %v, want 0", pagination["offset"])
	}
	if pagination["limit"] != 2 {
		t.Errorf("MapGiphyResponse() pagination limit = %v, want 2", pagination["limit"])
	}
	if pagination["total"] != 100 {
		t.Errorf("MapGiphyResponse() pagination total = %v, want 100", pagination["total"])
	}
	if pagination["next_offset"] != 2 {
		t.Errorf("MapGiphyResponse() pagination next_offset = %v, want 2", pagination["next_offset"])
	}
}

func TestMapGiphyResponse_EmptyData(t *testing.T) {
	response := &GiphyResponse{
		Data: []GiphyItem{},
		Pagination: GiphyPagination{
			TotalCount: 0,
			Count:      0,
			Offset:     0,
		},
	}

	result := MapGiphyResponse(response)
	if result == nil {
		t.Fatal("MapGiphyResponse() returned nil")
	}

	gifs, ok := result["gifs"].([]interface{})
	if !ok {
		t.Fatal("MapGiphyResponse() gifs should be []interface{}")
	}
	if len(gifs) != 0 {
		t.Errorf("MapGiphyResponse() should return 0 gifs for empty data, got %d", len(gifs))
	}
}

func TestMapGiphyResponse_NextOffset(t *testing.T) {
	response := &GiphyResponse{
		Data: []GiphyItem{
			{
				ID:    "gif1",
				Title: "GIF 1",
				Images: GiphyImages{
					Original: GiphyImage{URL: "https://media.giphy.com/gif1.gif"},
				},
			},
		},
		Pagination: GiphyPagination{
			TotalCount: 50,
			Count:      1,
			Offset:     25,
		},
	}

	result := MapGiphyResponse(response)
	pagination, ok := result["pagination"].(map[string]interface{})
	if !ok {
		t.Fatal("MapGiphyResponse() pagination should be map[string]interface{}")
	}

	expectedNextOffset := 25 + 1 // offset + count
	nextOffset := pagination["next_offset"]
	// next_offset can be int or float64 depending on JSON encoding
	switch v := nextOffset.(type) {
	case int:
		if v != expectedNextOffset {
			t.Errorf("MapGiphyResponse() next_offset = %d, want %d", v, expectedNextOffset)
		}
	case float64:
		if int(v) != expectedNextOffset {
			t.Errorf("MapGiphyResponse() next_offset = %v, want %d", v, expectedNextOffset)
		}
	default:
		t.Errorf("MapGiphyResponse() next_offset has unexpected type %T", nextOffset)
	}
}

// ============================================================================
// GiphyResponse Structure Tests
// ============================================================================

func TestGiphyResponseStructure(t *testing.T) {
	response := GiphyResponse{
		Data: []GiphyItem{
			{
				ID:    "test",
				Title: "Test",
				Images: GiphyImages{
					Original: GiphyImage{
						URL:    "http://test.gif",
						Width:  100,
						Height: 100,
						Size:   "1000",
					},
				},
			},
		},
		Pagination: GiphyPagination{
			TotalCount: 1,
			Count:      1,
			Offset:     0,
		},
		Meta: GiphyMeta{
			Status: 200,
			Msg:    "OK",
		},
	}

	if len(response.Data) != 1 {
		t.Errorf("GiphyResponse.Data length = %d, want 1", len(response.Data))
	}
	if response.Pagination.TotalCount != 1 {
		t.Errorf("GiphyResponse.Pagination.TotalCount = %d, want 1", response.Pagination.TotalCount)
	}
	if response.Meta.Status != 200 {
		t.Errorf("GiphyResponse.Meta.Status = %d, want 200", response.Meta.Status)
	}
}

// ============================================================================
// GiphyImage Structure Tests
// ============================================================================

func TestGiphyImageStructure(t *testing.T) {
	image := GiphyImage{
		URL:    "http://test.gif",
		Width:  480,
		Height: 360,
		Size:   "123456",
	}

	if image.URL != "http://test.gif" {
		t.Errorf("GiphyImage.URL = %s, want http://test.gif", image.URL)
	}
	if image.Width != 480 {
		t.Errorf("GiphyImage.Width = %d, want 480", image.Width)
	}
	if image.Height != 360 {
		t.Errorf("GiphyImage.Height = %d, want 360", image.Height)
	}
	if image.Size != "123456" {
		t.Errorf("GiphyImage.Size = %s, want 123456", image.Size)
	}
}

// ============================================================================
// GiphyItem Structure Tests
// ============================================================================

func TestGiphyItemStructure(t *testing.T) {
	item := GiphyItem{
		ID:    "test123",
		Title: "Test Title",
		URL:   "http://giphy.com/test123",
		Images: GiphyImages{
			Original: GiphyImage{
				URL:    "http://test.gif",
				Width:  480,
				Height: 360,
			},
		},
	}

	if item.ID != "test123" {
		t.Errorf("GiphyItem.ID = %s, want test123", item.ID)
	}
	if item.Title != "Test Title" {
		t.Errorf("GiphyItem.Title = %s, want Test Title", item.Title)
	}
	if item.URL != "http://giphy.com/test123" {
		t.Errorf("GiphyItem.URL = %s, want http://giphy.com/test123", item.URL)
	}
}
