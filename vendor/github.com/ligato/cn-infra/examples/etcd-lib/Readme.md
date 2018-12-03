# Phonebook example

The etcd library is showcased on the phonebook example. Phonebook entry
"Contact" is modelled by [protofile](model/phonebook/phonebook.proto).
For each entry name, company and phone number is stored.

To generate go structs from proto file run:
```
make generate
```

To start examples you have to have etcd running, if you don't have it
installed locally, you can use the following docker image:
```
sudo docker run -p 2379:2379 --name etcd --rm \
    quay.io/coreos/etcd:v3.0.16 /usr/local/bin/etcd \
    -advertise-client-urls http://0.0.0.0:2379 \
    -listen-client-urls http://0.0.0.0:2379
```

In the example, the connection to etcd is configured using `--cfg`
argument.
If the file is not specified, the application tries to connect to etcd
running on the localhost with the default port 2379.
 
The example contains three programs:

## View
View is a showcase for data retrieval. It prints the content
of the phonebook:
```
$go run view/view.go --cfg etcd.conf
Phonebook:
    John Doe
        Inc.
        4569
    Peter Smith
        Company xy
        +48621896
Revision 22
```

## Editor
Editor allows to add contact:
```
$go run editor/editor.go --cfg etcd.conf put "Peter Smith" "Company xy" "+48621896"
Saving  /phonebook/PeterSmith
```
create multiple contacts in one transaction:
```
$go run editor/editor.go puttxn '[{"name":"John Doe","company":"XY","phonenumber":"465464"}, {"name":"Tom New","company":"Comp","phonenumber":"123456"}]'
Saving  /phonebook/JohnDoe
Saving  /phonebook/TomNew
```
and to remove contacts from the phonebook:
```
$go run editor/editor.go --cfg etcd.conf delete "John Doe"
Removing  /phonebook/JohnDoe
```

## Watcher
Watcher monitors and logs the changes in the phonebook.
```
$go run watcher/watcher.go 
Watching the key:  /phonebook/
Creating  /phonebook/PeterSmith
        Peter Smith
                Company xy
                +48621896
============================================
```
