// Package gockle simplifies and mocks github.com/gocql/gocql. It provides
// simple interfaces to insert, query, and mutate Cassandra data, as well as get
// basic keyspace and table metadata.
//
// The entry points are NewSession and NewSimpleSession. Call them to get a
// Session. Session interacts with the database. It executes queries and batched
// queries and iterates result rows. Closing the Session closes the underlying
// gocql.Session, including the one passed to NewSimpleSession.
//
// Mocks are provided for testing use of Batch, Iterator, and Session.
//
// Tx is short for transaction.
//
// The name gockle comes from a pronunciation of gocql.
package gockle
