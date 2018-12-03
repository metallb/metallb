# Password Hasher

RPC plugin allows to register users via config file, where user name, password 
in hash format and permission groups are required. To make it easier, this utility
can help with password hashing.

```
password-hasher <password> <cost>
```

Put desired password as first parameter, and cost value as second. Keep in mind
that the cost value should match the one in security plugin, otherwise the password
cannot be verified. Cost value range is 4 - 31, high numbers require a lot of memory 
and CPU time to process. 