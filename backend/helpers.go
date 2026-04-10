package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	xdraw "golang.org/x/image/draw"
)

const editImageSize = 1024

type geminiGenerateRequest struct {
	Contents         []geminiContent        `json:"contents"`
	GenerationConfig geminiGenerationConfig `json:"generationConfig"`
}

type geminiGenerationConfig struct {
	ResponseModalities []string          `json:"responseModalities"`
	ImageConfig        geminiImageConfig `json:"imageConfig"`
}

type geminiImageConfig struct {
	AspectRatio string `json:"aspectRatio"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *geminiInlineData `json:"inline_data,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type geminiGenerateResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text            string            `json:"text"`
				InlineData      *geminiInlineData `json:"inline_data"`
				InlineDataCamel *geminiInlineData `json:"inlineData"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

type geminiErrorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// generateEditedPNG fetches a 1024x1024 WMS crop for the selected bounds,
// sends it to Nano Banana with the user's prompt, saves the edited 1024x1024
// result, and returns the saved path plus decoded image.
func generateEditedPNG(
	dir string,
	prompt string,
	west float64,
	east float64,
	south float64,
	north float64,
) (string, image.Image, error) {
	sourcePNG, err := fetchWMSBoundsPNG(west, east, south, north, editImageSize)
	if err != nil {
		return "", nil, fmt.Errorf("fetch 1024 wms crop: %w", err)
	}

	editedPNG, err := editImageWithGemini(sourcePNG, prompt)
	if err != nil {
		return "", nil, fmt.Errorf("edit image with nano banana: %w", err)
	}

	img, _, err := image.Decode(bytes.NewReader(editedPNG))
	if err != nil {
		return "", nil, fmt.Errorf("decode edited image: %w", err)
	}

	img = resizeToSquare(img, editImageSize)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", nil, fmt.Errorf("create generated dir: %w", err)
	}

	ts := time.Now().UnixNano()
	filename := fmt.Sprintf("%.6f-%.6f_%.6f-%.6f-%d.png", west, south, east, north, ts)
	path := filepath.Join(dir, filename)

	var normalized bytes.Buffer
	if err := png.Encode(&normalized, img); err != nil {
		return "", nil, fmt.Errorf("encode normalized edited png: %w", err)
	}

	if err := os.WriteFile(path, normalized.Bytes(), 0644); err != nil {
		return "", nil, fmt.Errorf("write edited png: %w", err)
	}

	return path, img, nil
}

func fetchWMSBoundsPNG(west, east, south, north float64, size int) ([]byte, error) {
	minX, minY := lngLatTo3857(west, south)
	maxX, maxY := lngLatTo3857(east, north)

	url := fmt.Sprintf(
		"%s?SERVICE=WMS&VERSION=1.3.0&REQUEST=GetMap"+
			"&LAYERS=%s&STYLES=&FORMAT=image/png&TRANSPARENT=TRUE"+
			"&CRS=EPSG:3857&WIDTH=%d&HEIGHT=%d"+
			"&BBOX=%.6f,%.6f,%.6f,%.6f",
		wmsBase, wmsLayer, size, size,
		minX, minY, maxX, maxY,
	)

	resp, err := wmsClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wms returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read wms body: %w", err)
	}

	return body, nil
}

func editImageWithGemini(sourcePNG []byte, prompt string) ([]byte, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is required")
	}

	model := os.Getenv("GEMINI_IMAGE_MODEL")
	if model == "" {
		model = "gemini-2.5-flash-image"
	}

	instruction := fmt.Sprintf(
		"Edit this 1024x1024 top-down map image so that it becomes: %s. "+
			"Keep the exact square crop, top-down map perspective, coastline/road alignment, and geographic footprint. "+
			"Return only the edited image.",
		prompt,
	)

	reqBody := geminiGenerateRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: instruction},
					{
						InlineData: &geminiInlineData{
							MimeType: "image/png",
							Data:     base64.StdEncoding.EncodeToString(sourcePNG),
						},
					},
				},
			},
		},
		GenerationConfig: geminiGenerationConfig{
			ResponseModalities: []string{"IMAGE"},
			ImageConfig: geminiImageConfig{
				AspectRatio: "1:1",
			},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal gemini request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create gemini request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)

	resp, err := wmsClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("post gemini request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read gemini response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var geminiErr geminiErrorResponse
		if err := json.Unmarshal(respBody, &geminiErr); err == nil && geminiErr.Error.Message != "" {
			return nil, fmt.Errorf("gemini returned status %d: %s", resp.StatusCode, geminiErr.Error.Message)
		}

		return nil, fmt.Errorf("gemini returned status %d", resp.StatusCode)
	}

	var result geminiGenerateResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode gemini response: %w", err)
	}

	for _, candidate := range result.Candidates {
		for _, part := range candidate.Content.Parts {
			inlineData := part.InlineData
			if inlineData == nil {
				inlineData = part.InlineDataCamel
			}
			if inlineData == nil || inlineData.Data == "" {
				continue
			}

			imageBytes, err := base64.StdEncoding.DecodeString(inlineData.Data)
			if err != nil {
				return nil, fmt.Errorf("decode gemini image: %w", err)
			}

			return imageBytes, nil
		}
	}

	return nil, fmt.Errorf("gemini response did not include an image")
}

func resizeToSquare(src image.Image, size int) image.Image {
	if src.Bounds().Dx() == size && src.Bounds().Dy() == size {
		return src
	}

	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	return dst
}
