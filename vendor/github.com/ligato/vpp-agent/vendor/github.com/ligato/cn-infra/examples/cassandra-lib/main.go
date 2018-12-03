// Copyright (c) 2017 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/gocql/gocql"
	"github.com/ligato/cn-infra/config"
	"github.com/ligato/cn-infra/db/sql"
	"github.com/ligato/cn-infra/db/sql/cassandra"
	"github.com/willfaught/gockle"
)

// UserTable global variable reused when building queries/statements
var UserTable = &User{}

// User is simple structure used in automated tests
type User struct {
	ID         gocql.UUID `cql:"userid" pk:"userid"`
	FirstName  string     `cql:"first_name"`
	MiddleName string     `cql:"middle_name"`
	LastName   string     `cql:"last_name"`
	//NetIP      net.IP //mapped to native cassandra type
	WrapIP *Wrapper01 //used for custom (un)marshalling
	Udt03  *Udt03
	Udt04  Udt04
	UdtCol []Udt03
}

// SchemaName demo schema name
func (entity *User) SchemaName() string {
	return "demo"
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Println(fmt.Errorf("Failed to load configuration %v", err))
		os.Exit(1)
	}

	clientCfg, err := cassandra.ConfigToClientConfig(&cfg)
	if err != nil {
		fmt.Println(fmt.Errorf("Failed to load configuration %v", err))
		os.Exit(1)
	}

	session, err := cassandra.CreateSessionFromConfig(clientCfg)
	defer session.Close()
	if err != nil {
		fmt.Println(fmt.Errorf("Failed to create session %v", err))
		os.Exit(1)
	}

	err = exampleKeyspace(session)
	if err != nil {
		fmt.Println(fmt.Errorf("Error in creating keyspace %v", err))
		os.Exit(1)
	}

	err = example(session)
	if err != nil {
		fmt.Println(fmt.Errorf("Error in executing DML/DDL statements %v", err))
		os.Exit(1)
	}
}

func loadConfig() (cassandra.Config, error) {
	var cfg cassandra.Config
	if len(os.Args) < 2 {
		return cfg, errors.New("Please provide yaml configuration file path")
	}

	configFileName := os.Args[1]
	err := config.ParseConfigFromYamlFile(configFileName, &cfg)
	return cfg, err
}

func exampleKeyspace(session *gocql.Session) (err error) {
	return session.Query("CREATE KEYSPACE IF NOT EXISTS demo WITH replication = {'class': 'SimpleStrategy', 'replication_factor' : 1};").Exec()
}

func example(session *gocql.Session) (err error) {
	err = exampleDDL(session)
	if err != nil {
		return err
	}
	err = exampleDML(session)
	if err != nil {
		return err
	}

	return nil
}

func exampleDDL(session *gocql.Session) (err error) {
	if err := session.Query("CREATE KEYSPACE IF NOT EXISTS demo WITH replication = {'class': 'SimpleStrategy', 'replication_factor' : 1};").
		Exec(); err != nil {
		return err
	}
	if err := session.Query(`CREATE TYPE IF NOT EXISTS demo.udt03 (
		tx text,
		tx2 text)`).Exec(); err != nil {
		return err
	}
	if err := session.Query(`CREATE TYPE IF NOT EXISTS demo.udt04 (
		ahoj text,
		caf frozen<udt03>)`).Exec(); err != nil {
		return err
	}

	if err := session.Query(`CREATE TABLE IF NOT EXISTS demo.user (
			userid uuid PRIMARY KEY,
				first_name text,
				middle_name text,
				last_name text,
				Udt03 frozen<Udt03>,
				Udt04 frozen<Udt04>,
				UdtCol list<frozen<Udt03>>,
				NetIP inet,
				WrapIP text,
				emails set<text>,
				topscores list<int>,
				todo map<timestamp, text>
		);`).
		Exec(); err != nil {
		return err
	}

	return session.Query("CREATE INDEX IF NOT EXISTS demo_users_last_name ON demo.user (last_name);").Exec()
}

func exampleDML(session *gocql.Session) (err error) {
	_ /*ip01 */, ipPrefix01, err := net.ParseCIDR("192.168.1.2/24")
	if err != nil {
		return err
	}
	db := cassandra.NewBrokerUsingSession(gockle.NewSession(session))
	written := &User{FirstName: "Fero",
		MiddleName: "M",
		LastName:   "Mrkva", /*ip01, */
		WrapIP:     &Wrapper01{ipPrefix01},
		Udt03:      &Udt03{Tx: "tx1", Tx2: "tx2" /*, Inet1: "201.202.203.204"*/},
		Udt04:      Udt04{"kuk", &Udt03{Tx: "txxxxxxxxx1", Tx2: "txxxxxxxxx2" /*, Inet1: "201.202.203.204"*/}},
		UdtCol:     []Udt03{{Tx: "txt1Col", Tx2: "txt2Col"}},
	}
	err = db.Put(sql.Exp("userid=c37d661d-7e61-49ea-96a5-68c34e83db3a"), written)
	if err == nil {
		fmt.Println("Successfully written: ", written)
	} else {
		return err
	}

	users := &[]User{}
	err = sql.SliceIt(users, db.ListValues(sql.FROM(UserTable,
		sql.WHERE(sql.Field(&UserTable.LastName, sql.EQ("Mrkva"))))))
	if err == nil {
		fmt.Println("Successfully queried: ", users)
	} else {
		return err
	}

	//example for AND condition
	usersForANDQuery := &[]User{}
	andQuery := sql.FROM(UserTable, sql.WHERE(
		sql.Field(&UserTable.FirstName), sql.EQ("Fero"),
		sql.AND(),
		sql.Field(&UserTable.LastName), sql.EQ("Mrkva")))

	err = sql.SliceIt(usersForANDQuery, db.ListValues(andQuery))
	if err == nil {
		fmt.Println("Successfully queried with AND: ", usersForANDQuery)
	} else {
		return err
	}

	//example for Multiple AND conditions
	usersForMultipleANDQuery := &[]User{}
	multipleAndQuery := sql.FROM(UserTable, sql.WHERE(
		sql.Field(&UserTable.FirstName), sql.EQ("Fero"),
		sql.AND(),
		sql.Field(&UserTable.LastName), sql.EQ("Mrkva"),
		sql.AND(),
		sql.Field(&UserTable.MiddleName), sql.EQ("M"),
	))

	err = sql.SliceIt(usersForMultipleANDQuery, db.ListValues(multipleAndQuery))
	if err == nil {
		fmt.Println("Successfully queried with Multiple AND: ", usersForMultipleANDQuery)
	} else {
		return err
	}

	//example for IN condition
	usersForINQuery := &[]User{}
	inQuery := sql.FROM(UserTable, sql.WHERE(
		sql.Field(&UserTable.FirstName), sql.IN("Fero"),
		sql.AND(),
		sql.Field(&UserTable.LastName), sql.IN("Mrkva")))

	err = sql.SliceIt(usersForINQuery, db.ListValues(inQuery))
	if err == nil {
		fmt.Println("Successfully queried with IN: ", usersForINQuery)
	} else {
		return err
	}

	return nil
}

// Wrapper01 implements gocql.Marshaller, gocql.Unmarshaller
// it uses string representation of net.IPNet
type Wrapper01 struct {
	ip *net.IPNet
}

// MarshalCQL serializes the string representation of net.IPNet
func (w *Wrapper01) MarshalCQL(info gocql.TypeInfo) ([]byte, error) {

	if w.ip == nil {
		return []byte{}, nil
	}

	return []byte(w.ip.String()), nil
}

// UnmarshalCQL deserializes the string representation of net.IPNet
func (w *Wrapper01) UnmarshalCQL(info gocql.TypeInfo, data []byte) error {

	if len(data) > 0 {
		_, ipPrefix, err := net.ParseCIDR(string(data))

		if err != nil {
			return err
		}
		w.ip = ipPrefix
	}

	return nil
}

// String delegates to the ip.String()
func (w *Wrapper01) String() string {
	if w.ip != nil {
		return w.ip.String()
	}

	return ""
}

// Udt03 is a simple User Defined Type with two string fields
type Udt03 struct {
	Tx  string `cql:"tx"`
	Tx2 string `cql:"tx2"`
	//Inet1 string
}

func (u *Udt03) String() string {
	return "{" + u.Tx + ", " + u.Tx2 /*+ ", " + u.Inet1*/ + "}"
}

// Udt04 is a nested User Defined Type
type Udt04 struct {
	Ahoj string `cql:"ahoj"`
	Caf  *Udt03 `cql:"caf"`
	//Inet1 string
}

func (u *Udt04) String() string {
	return "{" + u.Ahoj + ", " + u.Caf.String() /*+ ", " + u.Inet1*/ + "}"
}
