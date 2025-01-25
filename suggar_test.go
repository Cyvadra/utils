package utils_test

import (
	"testing"

	"github.com/Cyvadra/utils"
	"github.com/stretchr/testify/assert"
)

func TestInArray(t *testing.T) {
	assert.Equal(t, true, utils.InArray(1, []int{1, 2, 3}), "int")
	assert.Equal(t, false, utils.InArray(4, []int{1, 2, 3}), "int")
	assert.Equal(t, true, utils.InArray("1", []string{"1", "2", "3"}), "string")
}
