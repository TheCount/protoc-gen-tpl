package gen

import (
	"fmt"
	"sync"
)

// globals holds global variables for templates.
// Normal template variables are not inherited by nested templates.
// The globals mechanism with the setglob, getglob, and delglob functions
// circumvents this issue.
var globals = make(map[string]interface{})

// globalsMtx protects globals agains concurrent access.
var globalsMtx sync.Mutex

// setglob sets the specified named global variable to the given value.
// It always returns the empty string.
func setglob(name string, value interface{}) string {
	globalsMtx.Lock()
	defer globalsMtx.Unlock()
	globals[name] = value
	return ""
}

// getglob obtains the value of the named variable.
func getglob(name string) (interface{}, error) {
	globalsMtx.Lock()
	defer globalsMtx.Unlock()
	value, ok := globals[name]
	if !ok {
		return nil, fmt.Errorf("no such global variable: %s", name)
	}
	return value, nil
}

// delglob unsets the named global variable.
// If the variable does not exist, no operation is performed.
// It always returns the empty string.
func delglob(name string) string {
	globalsMtx.Lock()
	defer globalsMtx.Unlock()
	delete(globals, name)
	return ""
}
