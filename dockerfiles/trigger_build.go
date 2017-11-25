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
	qerr := quay()
	herr := dockerhub()

	if qerr != nil {
		log.Printf("Error triggering quay.io build: %s", qerr)
	}
	if herr != nil {
		log.Printf("Error triggering hub.docker.com build: %s", herr)
	}
	if qerr != nil || herr != nil {
		os.Exit(1)
	}
}

func quay() error {
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

	url := os.Getenv("QUAY_TRIGGER_URL")
	if url == "" {
		return errors.New("no QUAY_TRIGGER_URL found in environment")
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

func dockerhub() error {
	url := os.Getenv("DOCKER_TRIGGER_URL")
	if url == "" {
		return errors.New("no DOCKER_TRIGGER_URL found in environment")
	}

	resp, err := http.Post(url, "application/json", bytes.NewBufferString(`{"docker_tag": "latest"}`))
	if err != nil {
		return fmt.Errorf("post to docker trigger: %s", err)
	}
	if resp.StatusCode != 200 {
		msg, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("reading error message from docker trigger response: %s", err)
		}
		return fmt.Errorf("non-200 status from docker trigger: %s (%q)", resp.Status, string(msg))
	}
	return nil
}
