package phonebook

import "strings"

// EtcdPath returns the base path were the phonebook records are stored.
func EtcdPath() string {
	return "/phonebook/"
}

// EtcdContactPath returns the path for a given contact.
func EtcdContactPath(contact *Contact) string {
	return EtcdPath() + strings.Replace(contact.Name, " ", "", -1)
}
