// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/pkg/errors"
)

var (
	containerHandle *dockertest.Resource
	frrDir          string
)

const (
	frrImageTag = "8.5.2"
)

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("failed to create dockertest pool %s", err)
	}

	frrDir, err = os.MkdirTemp("/tmp", "frr_integration")
	if err != nil {
		log.Fatalf("failed to create temp dir %s", err)
	}

	containerHandle, err = pool.RunWithOptions(
		&dockertest.RunOptions{
			Name:       "frrtest",
			Repository: "quay.io/frrouting/frr",
			Tag:        frrImageTag,
			Mounts:     []string{fmt.Sprintf("%s:/etc/tempfrr", frrDir)},
		},
	)
	if err != nil {
		log.Fatalf("failed to run container %s", err)
	}

	cmd := exec.Command("cp", "testdata/vtysh.conf", filepath.Join(frrDir, "vtysh.conf"))
	res, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("failed to move vtysh.conf to %s - %s - %s", frrDir, err, res)
	}
	buf := new(bytes.Buffer)
	resCode, err := containerHandle.Exec([]string{"cp", "/etc/tempfrr/vtysh.conf", "/etc/frr/vtysh.conf"},
		dockertest.ExecOptions{
			StdErr: buf,
		})
	if err != nil || resCode != 0 {
		log.Fatalf("failed to move vtysh.conf inside the container - res %d %s %s", resCode, err, buf.String())
	}

	// override reloadConfig so it doesn't try to reload it.
	debounceTimeout = time.Millisecond
	reloadConfig = func() error { return nil }

	retCode := m.Run()
	// You can't defer this because os.Exit doesn't care for defer
	if err := pool.Purge(containerHandle); err != nil {
		log.Fatalf("failed to purge %s - %s", containerHandle.Container.Name, err)
	}
	os.RemoveAll(frrDir)

	os.Exit(retCode)
}

type invalidFileErr struct {
	Reason string
}

func (e invalidFileErr) Error() string {
	return e.Reason
}

func testFileIsValid(fileName string) error {
	cmd := exec.Command("cp", fileName, filepath.Join(frrDir, "frr.conf"))
	res, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "failed to copy %s to %s: %s", fileName, frrDir, string(res))
	}
	_, err = containerHandle.Exec([]string{"cp", "/etc/tempfrr/frr.conf", "/etc/frr/frr.conf"},
		dockertest.ExecOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to copy frr.conf inside the container")
	}
	buf := new(bytes.Buffer)
	code, err := containerHandle.Exec([]string{"python3", "/usr/lib/frr/frr-reload.py", "--test", "--stdout", "/etc/frr/frr.conf"},
		dockertest.ExecOptions{
			StdErr: buf,
		})
	if err != nil {
		return errors.Wrapf(err, "failed to exec reloader into the container")
	}

	if code != 0 {
		return invalidFileErr{Reason: buf.String()}
	}
	return nil
}

func TestDockerFRRFails(t *testing.T) {
	badFile := filepath.Join(testData, "TestDockerTestfails.golden")
	err := testFileIsValid(badFile)
	if !errors.As(err, &invalidFileErr{}) {
		t.Fatalf("Validity check of invalid file passed")
	}
}
