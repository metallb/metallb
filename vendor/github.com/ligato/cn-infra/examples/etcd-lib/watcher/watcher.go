package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/ligato/cn-infra/config"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/db/keyval/etcd"
	"github.com/ligato/cn-infra/db/keyval/kvproto"
	"github.com/ligato/cn-infra/examples/etcd-lib/model/phonebook"
	"github.com/ligato/cn-infra/logging/logrus"
)

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

func printContact(c *phonebook.Contact) {
	fmt.Printf("\t%s\n\t\t%s\n\t\t%s\n", c.Name, c.Company, c.Phonenumber)
}

func printPrevContact(c *phonebook.Contact) {
	fmt.Printf("Previous: \t%s\n\t\t%s\n\t\t%s\n", c.Name, c.Company, c.Phonenumber)
}

func main() {
	cfg, err := processArgs()
	if err != nil {
		printUsage()
		fmt.Println(err)
		os.Exit(1)
	}

	// Create connection to etcd datastore.
	broker, err := etcd.NewEtcdConnectionWithBytes(*cfg, logrus.DefaultLogger())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	// Initialize proto decorator.
	protoBroker := kvproto.NewProtoWrapper(broker)

	respChan := make(chan datasync.ProtoWatchResp, 0)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	// Register watcher and select the respChan channel as the destination
	// for the delivery of all the change events.
	err = protoBroker.Watch(keyval.ToChanProto(respChan), make(chan string), phonebook.EtcdPath())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("Watching the key: ", phonebook.EtcdPath())

	// Keep watching for changes until the interrupt signal is received.
watcherLoop:
	for {
		select {
		case resp := <-respChan:
			switch resp.GetChangeType() {
			case datasync.Put:
				contact := &phonebook.Contact{}
				prevContact := &phonebook.Contact{}
				fmt.Println("Creating ", resp.GetKey())
				resp.GetValue(contact)
				exists, err := resp.GetPrevValue(prevContact)
				if err != nil {
					logrus.DefaultLogger().Errorf("err: %v\n", err)
				}
				printContact(contact)
				if exists {
					printPrevContact(prevContact)
				} else {
					fmt.Printf("Previous value does not exist\n")
				}
			case datasync.Delete:
				fmt.Println("Removing ", resp.GetKey())
				prevContact := &phonebook.Contact{}
				exists, err := resp.GetPrevValue(prevContact)
				if err != nil {
					logrus.DefaultLogger().Errorf("err: %v\n", err)
				}
				if exists {
					printPrevContact(prevContact)
				} else {
					fmt.Printf("Previous value does not exist\n")
				}
			}
			fmt.Println("============================================")
		case <-sigChan:
			break watcherLoop
		}
	}
	fmt.Println("Stop requested ...")
	protoBroker.Close()
}
