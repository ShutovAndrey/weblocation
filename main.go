package main

import (
	"embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/ShutovAndrey/weblocation/pkg/logger"
	"github.com/ShutovAndrey/weblocation/pkg/provider"
	"github.com/go-redis/redis"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"github.com/robfig/cron/v3"
	"html/template"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
)

func init() {
	// get .env
	godotenv.Load()
}

type Weather struct {
	Main struct {
		Temp      float32 `json:"temp"`
		FeelsLike float32 `json:"feels_like"`
	} `json:"main"`
	Clouds struct {
		All uint8 `json:"all"`
	} `json:"clouds"`
}

var client *redis.Client

var tpl *template.Template

//go:embed templates
var index embed.FS

//go:embed static
var styles embed.FS

func indexHandler(w http.ResponseWriter, r *http.Request) {

	t, err := template.ParseFS(index, "templates/index.html")
	if err != nil {
		logger.Error(err)
	}

	ip := strings.Split(r.RemoteAddr, ":")[0]

	t.Execute(w, getDataByIP(ip))
	client.Incr("visitors")
}

func getDataByIP(ip string) map[string]any {
	blackList := [3]string{"localhost", "127.0.0.1", "0.0.0.0"}

	ipParsed := net.ParseIP(ip)

	for _, n := range blackList {
		if ipParsed == nil || ip == n {
			ipParsed = net.ParseIP("134.122.49.115")
		}
	}

	binaryIP := binary.BigEndian.Uint32(ipParsed[12:16])

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

		countries, err := client.ZRevRangeByScore("ip_countries", op).Result()
		if err != nil {
			countries = append(countries, "Russia")
		}
		country := strings.Split(countries[0], "_")[0]

		currency, err = client.HGet("currency", country).Result()
		if err != nil {
			currency = "RUB"
		}

	}()
	go func() {
		defer wg.Done()

		cities, err := client.ZRevRangeByScore("ip_cities", op).Result()
		if err != nil {
			cities = append(cities, "498817")
		}
		cityCode := strings.Split(cities[0], "_")[0]

		cityName, err = client.HGet("cities", cityCode).Result()
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
		logger.Error(errors.New("Received non 200 response code"))
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
		logger.Error(errors.New("Received non 200 response code"))
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "" // TODO make warning logs
	}

	return string(body)
}

//takes data from provider and set it to redis
func getOrUpdateData() {
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		ipCountries, countries := provider.GetFromDB("Country")
		client.Del("ip_countries")
		for i, ip := range *ipCountries {
			code, ok := countries[ip.Code]
			if !ok {
				continue
			}
			//unique names
			name := fmt.Sprintf("%s_%d", code, i)
			member := redis.Z{Score: float64(ip.Ip), Member: name}
			client.ZAdd("ip_countries", member)
		}
	}()

	go func() {
		defer wg.Done()
		ipCities, cities := provider.GetFromDB("City")
		client.Del("ip_cities")
		client.Del("cities")
		for i, ip := range *ipCities {
			cityName, ok := cities[ip.Code]
			if !ok {
				continue
			}
			client.HSet("cities", ip.Code, cityName)
			//unique names
			name := fmt.Sprintf("%s_%d", ip.Code, i)
			member := redis.Z{Score: float64(ip.Ip), Member: name}
			client.ZAdd("ip_cities", member)
		}
	}()

	go func() {
		defer wg.Done()
		_, currencies := provider.GetFromDB("Currency")
		client.Del("currency")
		for k, v := range currencies {
			client.HSet("currency", k, v)
		}
	}()
	wg.Wait()
	logger.Info("Redis databases updated")
}

func main() {
	logger.CreateLogger()
	defer logger.Close()

	client = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       5,
	})

	_, err := client.Ping().Result()
	if err != nil {
		logger.Error(err)
	}
	client.Set("visitors", 0, 0)

	fmt.Println("Collecting data. Please wait..")
	getOrUpdateData()

	c := cron.New()
	c.AddFunc("@daily", getOrUpdateData)
	c.Start()

	var stylesFS = http.FS(styles)
	fs := http.FileServer(stylesFS)

	// Serve static files
	http.Handle("/static/", fs)

	http.HandleFunc("/", indexHandler)

	fmt.Println("Listening on :80")

	err = http.ListenAndServe("", nil)
	if err != nil {
		c.Stop()
		logger.Error(err)
	}
}
