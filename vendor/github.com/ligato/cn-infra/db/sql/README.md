# SQL-like datastore

The `sql` package defines the API for accessing a data store using SQL. 
The `Broker` interface allows reading and manipulating with data.
The `Watcher` API provides functions for monitoring of changes
in a data store.

## Features

-	The user of the API has full control over the SQL statements,
    types & bindings passed to the `Broker`.
-   Expressions:
    -  Helper functions alleviate the need to write SQL strings. 
    -  The user can choose to only write expressions using helper
       functions
    -  The user can write portions of SQL statements by a hand
       (the `sql.Exp` helper function) and combine them with other
       expressions
-	The user can optionally use reflection to simplify repetitive work
    with Iterators & Go structures
-   The API will be reused for different databases.
    A specific implementation will be provided for each database.
