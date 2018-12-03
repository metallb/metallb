//go:generate protoc --proto_path=../model/phonebook --gogo_out=../model/phonebook ../model/phonebook/phonebook.proto

// Package view contains an example that shows how to read data from etcd.
package main

import (
	"fmt"
	"os"

	"github.com/ligato/cn-infra/config"
	"github.com/ligato/cn-infra/db/keyval/etcd"
	"github.com/ligato/cn-infra/db/keyval/kvproto"
	"github.com/ligato/cn-infra/examples/etcd-lib/model/phonebook"
	"github.com/ligato/cn-infra/logging/logrus"
)

// processArgs processes input arguments.
func processArgs() (*etcd.ClientConfig, error) {
	fileConfig := &etcd.Config{}
	if len(os.Args) > 2 {
		if os.Args[1] == "--cfg" {

			err := config.ParseConfigFromYamlFile(os.Args[2], fileConfig)
			if err != nil {
				return nil, err
			}

		} else {
			return nil, fmt.Errorf("incorrect arguments")
		}
	}

	return etcd.ConfigToClient(fileConfig)
}

func printUsage() {
	fmt.Printf("\n\n%s: [--cfg CONFIG_FILE] <delete NAME | put NAME COMPANY PHONE>\n\n", os.Args[0])
}

func main() {
	cfg, err := processArgs()
	if err != nil {
		printUsage()
		fmt.Println(err)
		os.Exit(1)
	}

	// Create connection to etcd.
	db, err := etcd.NewEtcdConnectionWithBytes(*cfg, logrus.DefaultLogger())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Initialize proto decorator.
	protoDb := kvproto.NewProtoWrapper(db)

	// Retrieve all contacts from database.
	resp, err := protoDb.ListValues(phonebook.EtcdPath())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Print out all contacts one-by-one.
	var revision int64
	fmt.Println("Phonebook:")
	for {
		c := &phonebook.Contact{}
		kv, stop := resp.GetNext()
		if stop {
			break
		}
		// Maintain the latest revision.
		if kv.GetRevision() > revision {
			revision = kv.GetRevision()
		}
		err = kv.GetValue(c)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Printf("\t%s\n\t\t%s\n\t\t%s\n", c.Name, c.Company, c.Phonenumber)

	}
	fmt.Println("Revision", revision)
	protoDb.Close()
}
