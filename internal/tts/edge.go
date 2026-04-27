// internal/tts/edge.go
package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"nhooyr.io/websocket"
)

const (
	edgeWSURL     = "wss://speech.platform.bing.com/consumer/speech/synthesize/readaloud/edge/v1?TrustedClientToken=6A5AA1D4EAFF4E9FB37E23D68491D6F4"
	edgeVoiceURL  = "https://speech.platform.bing.com/consumer/speech/synthesize/readaloud/voices/list?trustedclienttoken=6A5AA1D4EAFF4E9FB37E23D68491D6F4"
	edgeChunkSize = 3000 // characters per WebSocket request
)

// EdgeTTS implements the Edge TTS WebSocket protocol in pure Go.
type EdgeTTS struct{}

// NewEdgeTTS creates a new Edge TTS engine.
func NewEdgeTTS() *EdgeTTS {
	return &EdgeTTS{}
}

func (e *EdgeTTS) Name() string { return "edge" }

// ListVoices fetches available voices from the Edge TTS API.
func (e *EdgeTTS) ListVoices(ctx context.Context) ([]Voice, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", edgeVoiceURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch voices: %w", err)
	}
	defer resp.Body.Close()

	var raw []struct {
		ShortName    string `json:"ShortName"`
		FriendlyName string `json:"FriendlyName"`
		Locale       string `json:"Locale"`
		Gender       string `json:"Gender"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode voices: %w", err)
	}

	voices := make([]Voice, len(raw))
	for i, v := range raw {
		name := v.FriendlyName
		if idx := strings.LastIndex(name, " - "); idx > 0 {
			name = name[idx+3:]
		}
		voices[i] = Voice{
			ID:       v.ShortName,
			Name:     name,
			Language: v.Locale,
			Gender:   v.Gender,
		}
	}
	return voices, nil
}

// Synthesize converts text to audio using Edge TTS.
func (e *EdgeTTS) Synthesize(ctx context.Context, text string, opts SynthOptions) ([]byte, error) {
	if opts.Voice == "" {
		opts.Voice = "en-US-AriaNeural"
	}
	if opts.Rate == "" {
		opts.Rate = "+0%"
	}
	if opts.Volume == "" {
		opts.Volume = "+0%"
	}
	if opts.Format == "" {
		opts.Format = "audio-24khz-48kbitrate-mono-mp3"
	}

	chunks := e.chunkText(text, edgeChunkSize)
	var audio bytes.Buffer

	for _, chunk := range chunks {
		data, err := e.synthesizeChunk(ctx, chunk, opts)
		if err != nil {
			return nil, fmt.Errorf("synthesize chunk: %w", err)
		}
		audio.Write(data)
	}

	return audio.Bytes(), nil
}

func (e *EdgeTTS) synthesizeChunk(ctx context.Context, text string, opts SynthOptions) ([]byte, error) {
	connCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(connCtx, edgeWSURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"User-Agent": []string{"Mozilla/5.0"},
			"Origin":     []string{"chrome-extension://jdiccldimpdaibmpdkjnbmckianbfold"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer conn.CloseNow()

	// Send config message.
	configMsg := fmt.Sprintf(
		"Content-Type:application/json; charset=utf-8\r\nPath:speech.config\r\n\r\n"+
			`{"context":{"synthesis":{"audio":{"metadataoptions":{"sentenceBoundaryEnabled":"false","wordBoundaryEnabled":"false"},"outputFormat":"%s"}}}}`,
		opts.Format,
	)
	if err := conn.Write(connCtx, websocket.MessageText, []byte(configMsg)); err != nil {
		return nil, fmt.Errorf("send config: %w", err)
	}

	// Send SSML message.
	ssml := e.buildSSML(text, opts)
	ssmlMsg := "Content-Type:application/ssml+xml\r\nPath:ssml\r\n\r\n" + ssml
	if err := conn.Write(connCtx, websocket.MessageText, []byte(ssmlMsg)); err != nil {
		return nil, fmt.Errorf("send ssml: %w", err)
	}

	// Read audio responses.
	var audio bytes.Buffer
	for {
		msgType, data, err := conn.Read(connCtx)
		if err != nil {
			// Connection closed — done.
			break
		}

		if msgType == websocket.MessageBinary {
			// Binary messages contain audio data after a header.
			// Header ends with "Path:audio\r\n" followed by audio bytes.
			marker := []byte("Path:audio\r\n")
			idx := bytes.Index(data, marker)
			if idx >= 0 {
				audio.Write(data[idx+len(marker):])
			}
		} else if msgType == websocket.MessageText {
			// Check for turn.end signal.
			if strings.Contains(string(data), "turn.end") {
				break
			}
		}
	}

	conn.Close(websocket.StatusNormalClosure, "done")
	return audio.Bytes(), nil
}

// buildSSML creates an SSML document for the given text and options.
func (e *EdgeTTS) buildSSML(text string, opts SynthOptions) string {
	// Escape XML special characters.
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")

	return fmt.Sprintf(
		`<speak version='1.0' xmlns='http://www.w3.org/2001/10/synthesis' xml:lang='en-US'>`+
			`<voice name='%s'>`+
			`<prosody rate='%s' volume='%s'>%s</prosody>`+
			`</voice></speak>`,
		opts.Voice, opts.Rate, opts.Volume, text,
	)
}

// chunkText splits text into chunks of at most maxLen characters,
// preferring to split at sentence boundaries.
func (e *EdgeTTS) chunkText(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string
	remaining := text

	for len(remaining) > 0 {
		if len(remaining) <= maxLen {
			chunks = append(chunks, remaining)
			break
		}

		// Find the last sentence-ending punctuation within maxLen.
		cutoff := remaining[:maxLen]
		splitIdx := -1
		for _, sep := range []string{". ", "! ", "? ", ".\n", "!\n", "?\n"} {
			idx := strings.LastIndex(cutoff, sep)
			if idx > splitIdx {
				splitIdx = idx + len(sep)
			}
		}

		if splitIdx <= 0 {
			// No sentence boundary — split at last space.
			splitIdx = strings.LastIndex(cutoff, " ")
			if splitIdx <= 0 {
				splitIdx = maxLen
			}
		}

		chunks = append(chunks, strings.TrimSpace(remaining[:splitIdx]))
		remaining = strings.TrimSpace(remaining[splitIdx:])
	}

	return chunks
}
