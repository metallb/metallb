package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ligato/cn-infra/config"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/db/keyval/etcd"
	"github.com/ligato/cn-infra/db/keyval/kvproto"
	"github.com/ligato/cn-infra/examples/etcd-lib/model/phonebook"
	"github.com/ligato/cn-infra/logging/logrus"
)

const (
	// Put represents put operation for a single key-value pair.
	Put = iota
	// PutTxn represents put operation used in transaction.
	PutTxn = iota
	// Delete represents delete operation.
	Delete = iota
)

// processArgs processes input arguments.
func processArgs() (cfg *etcd.ClientConfig, op int, data []string, err error) {
	var task []string

	// default args
	fileConfig := &etcd.Config{}
	op = Put

	if len(os.Args) > 2 {
		if os.Args[1] == "--cfg" {
			err = config.ParseConfigFromYamlFile(os.Args[2], fileConfig)
			if err != nil {
				return
			}
			cfg, err = etcd.ConfigToClient(fileConfig)
			if err != nil {
				return
			}

			task = os.Args[3:]
		} else {
			task = os.Args[1:]
		}
	} else {
		return cfg, 0, nil, fmt.Errorf("incorrect arguments")
	}

	if len(task) < 2 || (task[0] == "put" && len(task) < 4) {
		return cfg, 0, nil, fmt.Errorf("incorrect arguments")
	}

	if task[0] == "delete" {
		op = Delete
	} else if task[0] == "puttxn" {
		op = PutTxn
	}

	return cfg, op, task[1:], nil
}

func printUsage() {
	fmt.Printf("\n\n%s: [--cfg CONFIG_FILE] <delete NAME | put NAME COMPANY PHONE | puttxn JSONENCODED_CONTACTS>\n\n", os.Args[0])
}

// put demonstrates the use Put() API to create a new contact in the database.
func put(db keyval.ProtoBroker, data []string) {
	c := &phonebook.Contact{Name: data[0], Company: data[1], Phonenumber: data[2]}

	key := phonebook.EtcdContactPath(c)

	// Insert the key-value pair.
	db.Put(key, c)

	fmt.Println("Saving ", key)
}

// putTxn demonstrates the use of NewTxn() and Commit() APIs to create multiple
// contacts in the database in one transaction.
func putTxn(db keyval.ProtoBroker, data string) {
	contacts := []phonebook.Contact{}

	json.Unmarshal([]byte(data), &contacts)

	txn := db.NewTxn()

	for i := range contacts {

		key := phonebook.EtcdContactPath(&contacts[i])
		fmt.Println("Saving ", key)
		//add the key-value pair into transaction
		txn.Put(key, &contacts[i])
	}

	txn.Commit()

}

// delete demonstrates the use of Delete() API to remove contact with a given
// name.
func delete(db keyval.ProtoBroker, name string) {
	key := phonebook.EtcdContactPath(&phonebook.Contact{Name: name})

	// Remove the key.
	db.Delete(key)
	fmt.Println("Removing ", key)
}

func main() {
	cfg, op, data, err := processArgs()
	if err != nil {
		printUsage()
		fmt.Println(err)
		os.Exit(1)
	}

	db, err := etcd.NewEtcdConnectionWithBytes(*cfg, logrus.DefaultLogger())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Initialize proto decorator.
	protoDb := kvproto.NewProtoWrapper(db)

	switch op {
	case Put:
		put(protoDb, data)
	case PutTxn:
		putTxn(protoDb, data[0])
	case Delete:
		delete(protoDb, data[0])
	default:
		fmt.Println("Unknown operation")
	}

	protoDb.Close()

}
