package slacklog

import (
	"encoding/json"
	"os"
)

// ReadFileAsJSON reads a file and unmarshal its contents as JSON to `dst`
// destination object.
func ReadFileAsJSON(filename string, dst interface{}) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	err = json.NewDecoder(f).Decode(dst)
	if err != nil {
		return err
	}
	return nil
}

// ReadLogSourceAsJSON reads an entry from LogSource and unmarshal its contents
// as JSON to `dst`.
func ReadLogSourceAsJSON(src LogSource, name string, dst interface{}) error {
	rc, err := src.Open(name)
	if err != nil {
		return err
	}
	defer rc.Close()
	err = json.NewDecoder(rc).Decode(dst)
	if err != nil {
		return err
	}
	return nil
}
