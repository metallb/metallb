package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

func main() {
	log.Printf("Triggering controller build")
	if err := quay("QUAY_TRIGGER_URL_CONTROLLER"); err != nil {
		log.Print(err)
		os.Exit(1)
	}
	log.Printf("Triggering BGP speaker build")
	if err := quay("QUAY_TRIGGER_URL_BGP_SPEAKER"); err != nil {
		log.Print(err)
		os.Exit(1)
	}
	log.Printf("Triggering tutorial BGP router build")
	if err := quay("QUAY_TRIGGER_URL_TUTORIAL_BGP_ROUTER"); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func quay(env string) error {
	info := struct {
		Commit string `json:"commit"`
		Ref    string `json:"ref"`
		Branch string `json:"default_branch"`
	}{
		Commit: os.Getenv("TRAVIS_COMMIT"),
		Ref:    "refs/heads/master",
		Branch: "master",
	}

	if info.Commit == "" {
		return errors.New("no TRAVIS_COMMIT found in environment")
	}

	bs, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshal commit info: %s", err)
	}

	url := os.Getenv(env)
	if url == "" {
		return fmt.Errorf("no %s found in environment", env)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(bs))
	if err != nil {
		return fmt.Errorf("post to quay trigger: %s", err)
	}
	if resp.StatusCode != 200 {
		msg, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("reading error message from quay trigger response: %s", err)
		}
		return fmt.Errorf("non-200 status from quay trigger: %s (%q)", resp.Status, string(msg))
	}
	return nil
}
