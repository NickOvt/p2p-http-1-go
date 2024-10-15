package main

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/mitchellh/mapstructure"
)

func mapToObj(input any, output any) error {
	cfg := &mapstructure.DecoderConfig{
		Metadata: nil,
		Result:   &output,
		TagName:  "json",
	}

	decoder, err := mapstructure.NewDecoder(cfg)

	if err != nil {
		return err
	}

	decoder.Decode(input)
	return nil
}

func parseHeaders(headers []string) map[string]string {
	// Parse headers into a map

	headersMap := make(map[string]string)

	for _, header := range headers {
		splittedHeader := strings.SplitN(strings.ReplaceAll(header, " ", ""), ":", 2)
		headerKey := splittedHeader[0]
		headerValue := splittedHeader[1]

		headersMap[headerKey] = headerValue
	}

	return headersMap
}

func parseHeadersString(headers []string) map[string]string {
	var headerStrings []string

	for _, headerString := range headers {
		if headerString != "" {
			headerStrings = append(headerStrings, headerString)
		}
	}

	return parseHeaders(headerStrings)
}

func sha256encode(val []byte) string {
	h := sha256.New()

	h.Write([]byte(val))

	bs := h.Sum(nil)
	return hex.EncodeToString(bs)
}

func getPathParams(httpString string) []string {
	pathParams := strings.Split(strings.Split(httpString, " ")[1], "/")
	return pathParams[1:] // first elem is empty string
}
