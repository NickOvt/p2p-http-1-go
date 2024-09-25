package main

import "github.com/mitchellh/mapstructure"

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
