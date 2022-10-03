package main

import (
	"github.com/stretchr/testify/require"
	"strconv"
	"testing"
)

func TestGetWeather(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		data := getWeather("524901") // Moscow
		require.IsType(t, 0.123, data.Main.Temp)
		require.GreaterOrEqual(t, data.Clouds.All, 0)
		require.LessOrEqual(t, data.Clouds.All, 100)
	})
}

func TestGetExRates(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		data := getExRates("RUB") // Moscow
		rate, _ := strconv.ParseFloat(data, 64)
		require.GreaterOrEqual(t, rate, 0.0)

		data = getExRates("test")
		require.Empty(t, data)
	})

}
