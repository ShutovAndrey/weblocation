package provider

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDownloadDB(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		files, err := downloadDB("Currency")
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(files), 1)
	})

	t.Run("Error", func(t *testing.T) {
		_, err := downloadDB("CurrencyError")
		require.ErrorContains(t, err, "Unknown DB", "")
	})
}

func TestReadCsvFile(t *testing.T) {
	//TODO make some mocking
	fileNames, _ := downloadDB("Currency")
	t.Run("Success", func(t *testing.T) {
		ipMap, err := readCsvFile(fileNames["Currency"], 1, 3)
		require.NoError(t, err)
		require.Greater(t, len(ipMap), 200)
	})

	t.Run("Error", func(t *testing.T) {
		_, err := readCsvFile(fileNames["Currency"], 0, 18)
		require.ErrorContains(t, err, "Invalid key-value pair", "")
	})
}
