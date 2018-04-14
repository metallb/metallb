package e2e

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func rootdir() string {
	wd, err := os.Getwd()
	if err != nil {
		panic("can't get working dir")
	}
	return filepath.Dir(wd)
}

func tfdir() string {
	return filepath.Join(rootdir(), "e2etest/terraform")
}

func tfbin() string {
	path, err := exec.LookPath("terraform")
	if err != nil {
		panic("can't find terraform")
	}
	return path
}

func makebin() string {
	path, err := exec.LookPath("make")
	if err != nil {
		panic("can't find make")
	}
	return path
}

func terraformApply(project, clusterName, protocol, networkAddon string) error {
	args := []string{
		"apply",
		fmt.Sprintf("-state=%s.tfstate", clusterName),
		"-backup=-",
		"-auto-approve",
		"-no-color",
		fmt.Sprintf("-var=cluster_name=%s", clusterName),
		fmt.Sprintf("-var=protocol=%s", protocol),
		fmt.Sprintf("-var=network_addon=%s", networkAddon),
		fmt.Sprintf("-var=gcp_project=%s", project),
	}
	cmd := exec.Command(tfbin(), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = tfdir()
	return cmd.Run()
}

func terraformDestroy(project, clusterName string) error {
	args := []string{
		"destroy",
		"-force",
		"-no-color",
		fmt.Sprintf("-state=%s.tfstate", clusterName),
		fmt.Sprintf("-var=gcp_project=%s", project),
	}
	cmd := exec.Command(tfbin(), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = tfdir()
	return cmd.Run()
}

func pushAndDeploy(project, clusterName string) error {
	args := []string{
		"push-images",
		fmt.Sprintf("REGISTRY=gcr.io/%s", project),
		fmt.Sprintf("TAG=%s", clusterName),
	}
	cmd := exec.Command(makebin(), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = rootdir()
	return cmd.Run()
}

func runTests(m *testing.M) int {
	pfx := flag.String("test-prefix", "test", "VM name prefix for the test universe")
	proto := flag.String("proto", "ipv4", "IP protocol version to test")
	addon := flag.String("addon", "flannel", "Network addon to install for test")
	build := flag.Bool("build", true, "Build a test cluster before testing")
	destroy := flag.Bool("destroy", true, "Tear down the test cluster after testing")
	gcpProject := flag.String("gcp_project", "metallb-e2e-testing", "GCP project to use for tests")
	flag.Parse()

	clusterName := fmt.Sprintf("%s-%s-%s", *pfx, *proto, *addon)

	if *destroy {
		defer func() {
			if err := terraformDestroy(*gcpProject, clusterName); err != nil {
				log.Printf("cluster teardown of %q failed: %s", clusterName, err)
			}
		}()
	}
	if *build {
		if err := terraformDestroy(*gcpProject, clusterName); err != nil {
			log.Printf("pre-bringup cleanup of %q failed: %s", clusterName, err)
			return 1
		}
		if err := terraformApply(*gcpProject, clusterName, *proto, *addon); err != nil {
			log.Printf("bringup of cluster %q failed: %s", clusterName, err)
			return 1
		}
	}

	if err := pushAndDeploy(*gcpProject, clusterName); err != nil {
		log.Printf("code deploy to %q failed: %s", clusterName, err)
		return 1
	}

	return m.Run()
}

func TestMain(m *testing.M) {
	//os.Exit(runTests(m))
}
