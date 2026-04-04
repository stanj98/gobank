package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewAcc(t *testing.T) {
	acc, err := NewAccount("a", "b", "stan")
	assert.Nil(t, err)

	fmt.Printf("%+v\n", acc)

}
