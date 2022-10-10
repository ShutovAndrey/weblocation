package provider

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/ShutovAndrey/weblocation/internal/database"
	"github.com/ShutovAndrey/weblocation/internal/services/logger"
	"github.com/go-redis/redis"
	"github.com/pkg/errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
)

type Weather struct {
	Main struct {
		Temp      float32 `json:"temp"`
		FeelsLike float32 `json:"feels_like"`
	} `json:"main"`
	Clouds struct {
		All uint8 `json:"all"`
	} `json:"clouds"`
}

func getBinaryIp(ip string) uint32 {
	database.Incr("visitors")

	blackList := [3]string{"localhost", "127.0.0.1", "0.0.0.0"}

	ipParsed := net.ParseIP(ip)

	for _, n := range blackList {
		if ipParsed == nil || ip == n {
			ipParsed = net.ParseIP("134.122.49.115")
		}
	}

	return binary.BigEndian.Uint32(ipParsed[12:16])
}

func GetDataByIP(ip string) map[string]any {
	binaryIP := getBinaryIp(ip)

	op := redis.ZRangeBy{
		Min:    "-inf",
		Max:    fmt.Sprint(binaryIP),
		Offset: 0,
		Count:  1,
	}
	var weather Weather
	var cityName, currency string

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()

		countries, err := database.ZRevRangeByScore("ip_country", op)
		if err != nil {
			countries = append(countries, "Russia")
		}
		country := strings.Split(countries[0], "_")[0]

		currency, err = database.HGet("currency", country)
		if err != nil {
			currency = "RUB"
		}

	}()
	go func() {
		defer wg.Done()

		cities, err := database.ZRevRangeByScore("ip_city", op)
		if err != nil {
			cities = append(cities, "498817")
		}
		cityCode := strings.Split(cities[0], "_")[0]

		cityName, err = database.HGet("cities", cityCode)
		if err != nil || cityName == "" {
			cityName = "Saint-Petersburg"
		}
		weather = getWeather(cityCode)
	}()

	wg.Wait()

	return map[string]any{
		"city":         cityName,
		"clouds":       weather.Clouds.All,
		"temp":         weather.Main.Temp,
		"temp_feels":   weather.Main.FeelsLike,
		"currencyRate": getExRates(currency),
		"currency":     currency,
	}
}

func getWeather(cityId string) Weather {
	var w Weather

	key, ok := os.LookupEnv("WEATHER_KEY")
	if !ok {
		w.Main.Temp = 10.00
		w.Main.FeelsLike = 11.00
		w.Clouds.All = 40
		return w
	}

	uri := fmt.Sprintf(
		"https://api.openweathermap.org/data/2.5/weather?id=%s&units=metric&appid=%s",
		cityId, key)
	resp, _ := http.Get(uri)
	if resp.StatusCode != 200 {
		logger.Error(errors.New("Received non 200 response code from Openweather"))
	}

	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&w); err != nil {
		logger.Error(err)
	}

	return w
}

func getExRates(currency string) string {
	from := "EUR"
	if currency == "EUR" {
		from = "USD"
	}

	uri := fmt.Sprintf(
		"https://api.coingate.com/v2/rates/merchant/%s/%s",
		from, currency)
	resp, _ := http.Get(uri)
	if resp.StatusCode != 200 {
		logger.Error(errors.New("Received non 200 response code from Coingate"))
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "" // TODO make warning logs
	}

	return string(body)
}
