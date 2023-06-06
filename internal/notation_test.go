package internal

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNotation(t *testing.T) {
	t.Run("Given a spreadsheet conversion function", func(t *testing.T) {
		t.Run("When given the index of 0", func(t *testing.T) {
			t.Run("Then the result is A", func(t *testing.T) {
				assert.Equal(t, "A", IndexToLetter(0))
			})
		})
		t.Run("When given the index of 1", func(t *testing.T) {
			t.Run("Then the result is B", func(t *testing.T) {
				assert.Equal(t, "B", IndexToLetter(1))
			})
		})
		t.Run("When given the index of 25", func(t *testing.T) {
			t.Run("Then the result is Z", func(t *testing.T) {
				assert.Equal(t, "Z", IndexToLetter(25))
			})
		})
		t.Run("When given the index of 26", func(t *testing.T) {
			t.Run("Then the result is AA", func(t *testing.T) {
				assert.Equal(t, "AA", IndexToLetter(26))
			})
		})
		t.Run("When given the index of 51", func(t *testing.T) {
			t.Run("Then the result is AZ", func(t *testing.T) {
				assert.Equal(t, "AZ", IndexToLetter(51))
			})
		})
		t.Run("When given the index of 52", func(t *testing.T) {
			t.Run("Then the result is BA", func(t *testing.T) {
				assert.Equal(t, "BA", IndexToLetter(52))
			})
		})
	})
}
