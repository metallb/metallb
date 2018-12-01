package main

import (
	"context"
	"os"

	"go.universe.tf/virtuakube"
)

const vmImageName = "virtuakube-metallb.qcow2"

func getVMImage() (string, error) {
	_, err := os.Stat(vmImageName)
	if err == nil {
		return vmImageName, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}

	// Image not present, build it.
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	cfg := &virtuakube.BuildConfig{
		OutputPath: vmImageName,
		TempDir:    wd,
		CustomizeFuncs: []virtuakube.BuildCustomizeFunc{
			virtuakube.CustomizeInstallK8s,
			virtuakube.CustomizePreloadK8sImages,
			customizeInstallBird,
		},
		BuildLog: os.Stdout,
	}
	if err := virtuakube.BuildImage(context.Background(), cfg); err != nil {
		return "", err
	}

	return vmImageName, nil
}

func customizeInstallBird(v *virtuakube.VM) error {
	_, err := v.Run("DEBIAN_FRONTEND=noninteractive apt-get -y install --no-install-recommends bird")
	return err
}
