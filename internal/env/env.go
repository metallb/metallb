package env

import (
    "os"
    "strings"
)

func BGPDisabled() bool {
    return strings.ToLower(os.Getenv("METALLB_DISABLE_BGP")) == "true"
}

